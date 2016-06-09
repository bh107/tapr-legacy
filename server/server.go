package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"golang.org/x/net/context"

	"github.com/bh107/tapr/changer"
	"github.com/bh107/tapr/config"
	"github.com/bh107/tapr/inventory"
	"github.com/bh107/tapr/ltfs"
	"github.com/bh107/tapr/mtx"
	"github.com/bh107/tapr/stream"
	"github.com/bh107/tapr/stream/policy"

	"github.com/boltdb/bolt"
)

type Library struct {
	name   string
	chgr   *changer.Changer
	drives map[string][]*Drive
}

func NewLibrary(name string) *Library {
	return &Library{
		name:   name,
		drives: make(map[string][]*Drive),
	}
}

func (lib *Library) String() string {
	return lib.name
}

type driveStruct struct {
	unused          chan *Drive
	shared          chan *Drive
	exclusiveWaiter chan struct{}
}

type Server struct {
	libraries map[string]*Library
	cfg       *config.Config

	chunkdb *bolt.DB
	inv     *inventory.DB

	root string

	drives map[string]driveStruct

	mocked bool
}

func initChunkStore(cfg *config.Config) (*bolt.DB, error) {
	if cfg.Chunkstore.Path == "" {
		cfg.Chunkstore.Path = "./chunks.db"
	}

	chunkdb, err := bolt.Open(cfg.Chunkstore.Path, 0600, nil)
	if err != nil {
		return nil, err
	}

	return chunkdb, nil
}

func initInventory(cfg *config.Config) (*inventory.DB, error) {
	if cfg.Inventory.Path == "" {
		cfg.Inventory.Path = "./inventory.db"
	}

	inv, err := inventory.Open(cfg.Inventory.Path)
	if err != nil {
		return nil, err
	}

	return inv, nil
}

func New(cfg *config.Config, debug bool, audit bool, mock bool) (*Server, error) {
	srv := new(Server)

	srv.cfg = cfg

	if mock {
		srv.mocked = true
	}

	var err error

	srv.chunkdb, err = initChunkStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chunkstore: %s", err)
	}

	srv.inv, err = initInventory(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize inventory: %s", err)
	}

	srv.libraries = make(map[string]*Library)
	srv.drives = make(map[string]driveStruct)
	for _, d := range []string{"read", "write"} {
		srv.drives[d] = driveStruct{
			unused: make(chan *Drive),
			shared: make(chan *Drive),
		}
	}

	var numWriters int

	// initialize libraries
	for _, libCfg := range cfg.Libraries {
		log.Printf("init: adding library:   \"%s\"", libCfg.Name)
		lib := NewLibrary(libCfg.Name)

		for _, chgrCfg := range libCfg.Changers {
			log.Printf("init: + adding changer: %s", chgrCfg.Path)
			if mock {
				lib.chgr = changer.Mock(chgrCfg.Path)
			} else {
				lib.chgr = changer.New(chgrCfg.Path)
			}
		}

		for _, drvCfg := range libCfg.Drives {
			log.Printf("init: + adding drive:   %s (%s)", drvCfg.Path, drvCfg.Type)
			drv := srv.NewDrive(drvCfg.Path, drvCfg.Type, drvCfg.Slot, lib)
			lib.drives[drvCfg.Type] = append(lib.drives[drvCfg.Type], drv)
		}

		srv.libraries[libCfg.Name] = lib

		if audit {
			log.Printf("init: audit started for \"%s\" library", libCfg.Name)
			_, err := srv.Audit(context.Background(), libCfg.Name)
			if err != nil {
				return nil, err
			}
			log.Printf("init: audit finished for \"%s\" library", libCfg.Name)
		}

		numWriters += len(lib.drives["write"])
	}

	return srv, nil
}

func (srv *Server) Volumes(libname string) ([]*mtx.Volume, error) {
	return srv.inv.Volumes(libname)
}

func (srv *Server) Load(dev *Drive, vol *mtx.Volume) error {
	if dev.vol != nil {
		if dev.vol.Serial == vol.Serial {
			log.Printf("drive %s already loaded with %s", dev, vol)
			return nil
		}
	}

	err := dev.lib.chgr.Use(func(tx *changer.Tx) error {
		log.Printf("loading drive %s with volume %s from slot %d", dev, vol, vol.Home)
		var err error
		err = tx.Load(vol.Home, dev.slot)
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				return errors.New(string(exitError.Stderr))
			}

			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	dev.vol = vol

	return nil
}

func (srv *Server) Unload(dev *Drive) error {
	if dev.vol == nil {
		log.Printf("drive %s already unloaded", dev)
		return nil
	}

	err := dev.lib.chgr.Use(func(tx *changer.Tx) error {
		log.Printf("unloading drive %s, returning volume %s to slot %d", dev, dev.vol, dev.vol.Home)
		var err error
		err = tx.Unload(dev.vol.Home, dev.slot)
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				return errors.New(string(exitError.Stderr))
			}

			return err
		}

		return nil
	})

	return err
}

func (srv *Server) Audit(ctx context.Context, libname string) (*mtx.Status, error) {
	if lib, ok := srv.libraries[libname]; ok {
		var status *mtx.Status
		err := lib.chgr.Use(func(tx *changer.Tx) error {
			var err error
			status, err = tx.Status()
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					return errors.New(string(exitError.Stderr))
				}

				return err
			}

			// we do all auditing inside the changer lock
			err = srv.inv.Audit(status, libname)
			return err
		})

		if err != nil {
			return nil, err
		}

		return status, nil
	}

	return nil, fmt.Errorf("unknown library: %s", libname)
}

type ErrShortWrite struct {
	Written int
}

func (e ErrShortWrite) Error() string {
	return fmt.Sprintf("short write. wrote %d bytes", e.Written)
}

// Store grabs an io.Reader, reads until EOF and stores the data on a tape.
func (srv *Server) Store(ctx context.Context, archive string, rd io.Reader) error {
	log.Printf("store archive: %s", archive)

	pol := policy.DefaultPolicy

	// see if there is a write policy associated
	if newpol, ok := policy.Unwrap(ctx); ok {
		pol = newpol
	}

	// create new stream
	stream := stream.New(pol)

	// attach the stream to the writing system
	// include the setup-timeout
	var cancel context.CancelFunc
	if pol.ExclusiveTimeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, pol.ExclusiveTimeout)
	}

	if err := srv.Attach(ctx, stream); err != nil {
		return err
	}

	if cancel != nil {
		cancel()
	}

	reader := bufio.NewReader(rd)

	buf := make([]byte, 4096)

	var written, total int

	for {
		n, err := reader.Read(buf[:cap(buf)])
		buf = buf[:n]
		if n == 0 {
			if err == nil {
				continue
			}

			if err == io.EOF {
				break
			}

			return err
		}

		if err != nil && err != io.EOF {
			return err
		}

		if written, err = stream.Write(ctx, buf); err != nil {
			return ErrShortWrite{total + written}
		}

		total += len(buf)
	}

	if err := stream.Close(ctx); err != nil {
		return err
	}

	return nil
}

func (srv *Server) Shutdown() {
	fmt.Println()
	log.Print("shutdown: starting...")
	srv.chunkdb.Close()
	log.Print("shutdown: chunkdb closed")
	srv.inv.Close()
	log.Print("shutdown: inv closed")

	log.Print("shutdown: stats")
}

func (srv *Server) Create(archive string) error {
	return srv.chunkdb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(archive))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		return nil
	})
}

// Attach attaches a Stream to the Writer.
func (srv *Server) Attach(ctx context.Context, s *stream.Stream) error {
	drv, err := srv.Get(ctx, s.Policy())
	if err != nil {
		return err
	}

	in, _ := drv.writer.Ingress()
	s.SetOut(in)

	return nil
}

// Get returns a shared/exclusive writer or nil if operation is cancelled
// before one could be acquired.
func (srv *Server) Get(ctx context.Context, pol *policy.Policy) (*Drive, error) {
	var drv *Drive
	var err error

	if pol.Exclusive {
		drv, err = srv.GetExclusive(ctx)
	} else {
		drv, err = srv.GetShared(ctx)
	}

	if err != nil {
		return nil, err
	}

	drv.ctrl <- func(drv *Drive) error {
		drv.attached++

		return nil
	}

	return drv, nil
}

// GetShared returns a shared writer or nil if operation is cancelled before
// one could be acquired.
func (srv *Server) GetShared(ctx context.Context) (*Drive, error) {
	var drv *Drive

	select {
	case <-srv.drives["write"].exclusiveWaiter:
		// don't grab anything at the buffet, wait for for the exclusive writer
		// to get a chance at the table and leave.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case drv = <-srv.drives["write"].unused:
		}
	default:
		// try either
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case drv = <-srv.drives["write"].unused:
		case drv = <-srv.drives["write"].shared:
		}
	}

	// send it back to the shared channel if anyone wants one from it, or to
	// the unused, if it is released before we have a chance to send it.
	go func() {
		select {
		case <-drv.released:
			// return to unused channel
			srv.drives["write"].unused <- drv
		case srv.drives["write"].shared <- drv:
		}
	}()

	return drv, nil
}

// GetExclusive returns a writer or nil if timeout was closed before one could
// be acquired.
func (srv *Server) GetExclusive(ctx context.Context) (*Drive, error) {
	// tell the communal hippies that we want the table for our selves and
	// don't wanna starve!
	go func() { srv.drives["write"].exclusiveWaiter <- struct{}{} }()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case drv := <-srv.drives["write"].unused:
		// wait until released, then put back as unused
		go func() { <-drv.released; srv.drives["write"].unused <- drv }()

		return drv, nil
	}
}

// Release returns the drive as unused if after release, no streams are
// attached.
func (srv *Server) Release(drv *Drive) {
	drv.ctrl <- func(drv *Drive) error {
		drv.attached--

		if drv.attached == 0 {
			// release it (go back to unused)
			drv.released <- struct{}{}
		}

		return nil
	}
}

func (srv *Server) GetScratch(drv *Drive) (*mtx.Volume, error) {
	if drv.vol != nil {
		if err := srv.Unload(drv); err != nil {
			return nil, err
		}
	}

	vol, err := srv.inv.GetScratch(drv.lib.name)
	if err != nil {
		return nil, err
	}

	if err := srv.Load(drv, vol); err != nil {
		return nil, err
	}

	mountpoint, err := drv.Mountpoint()
	if err != nil {
		return nil, err
	}

	if err := os.Mkdir(mountpoint, os.ModePerm); err != nil {
		return nil, err
	}

	if !srv.mocked {
		h, err := ltfs.New(drv.path)
		if err != nil {
			return nil, err
		}

		if err := h.Format(); err != nil {
			return nil, err
		}

		if err := h.Mount(mountpoint, ltfs.SyncModeUnmount); err != nil {
			return nil, err
		}
	}

	return vol, nil
}
