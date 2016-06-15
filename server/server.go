package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"golang.org/x/net/context"

	"github.com/bh107/tapr/changer"
	"github.com/bh107/tapr/config"
	"github.com/bh107/tapr/inventory"
	"github.com/bh107/tapr/ltfs"
	"github.com/bh107/tapr/mtx"
	"github.com/bh107/tapr/stream/policy"
	"github.com/pkg/errors"

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

type iodev struct {
	unused          chan *Drive
	shared          chan *Drive
	exclusiveWaiter chan struct{}
}

type driveGroup struct {
	drives []*Drive
	in     chan *Chunk
}

type Server struct {
	libraries map[string]*Library
	drives    map[string][]*Drive
	cfg       *config.Config

	chunkdb *bolt.DB
	inv     *inventory.DB

	//iodevs map[string]iodev
	groups map[string]*driveGroup

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
		return nil, errors.Wrap(err, "failed to initialize chunkstore")
	}

	srv.inv, err = initInventory(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize inventory")
	}

	srv.libraries = make(map[string]*Library)
	srv.drives = make(map[string][]*Drive)
	//srv.iodevs = make(map[string]iodev)
	srv.groups = make(map[string]*driveGroup)

	/*
		for _, d := range []string{"read", "write"} {
			srv.iodevs[d] = iodev{
				unused: make(chan *Drive),
				shared: make(chan *Drive),
			}
		}*/

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
			lib.drives[drvCfg.Path] = append(lib.drives[drvCfg.Path], drv)
			srv.drives[drvCfg.Type] = append(srv.drives[drvCfg.Type], drv)

			if drvCfg.Group != "" {
				var grp *driveGroup
				var ok bool
				if grp, ok = srv.groups[drvCfg.Group]; !ok {
					log.Printf("init: + adding group: %s", drvCfg.Group)
					grp = &driveGroup{
						in:     make(chan *Chunk),
						drives: make([]*Drive, 0),
					}
					srv.groups[drvCfg.Group] = grp
				}

				drv.group = grp

				log.Printf("init: + adding %s to %s group", drv, drvCfg.Group)
				grp.drives = append(grp.drives, drv)
			}
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

	}

	for _, drv := range srv.drives["write"] {
		vol, err := srv.GetScratch(drv)
		if err != nil {
			panic(err)
		}

		mountpoint, err := drv.Mountpoint()
		if err != nil {
			panic(err)
		}

		var agg chan *Chunk
		if drv.group != nil {
			agg = drv.group.in
		}

		drv.writer = NewWriter(mountpoint, vol, drv.in, agg, drv)

		go drv.Run()

		/*
			go func(drv *Drive) {
				srv.iodevs["write"].shared <- drv
			}(drv)
		*/
	}

	return srv, nil
}

func (srv *Server) Volumes(libname string) ([]*mtx.Volume, error) {
	return srv.inv.Volumes(libname)
}

func (srv *Server) Load(dev *Drive, vol *mtx.Volume) error {
	if dev.vol != nil {
		if dev.vol.Serial == vol.Serial {
			log.Printf("load: drive %s already loaded with %s", dev, vol)
			return nil
		}
	}

	err := dev.lib.chgr.Use(func(tx *changer.Tx) error {
		log.Printf("load: loading drive %s with volume %s from slot %d", dev, vol, vol.Home)

		// simulate loading time
		if srv.mocked {
			time.Sleep(2 * time.Second)
		}

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
		log.Printf("unload: drive %s already unloaded", dev)
		return nil
	}

	err := dev.lib.chgr.Use(func(tx *changer.Tx) error {
		log.Printf("unload: unloading drive %s, returning volume %s to slot %d", dev, dev.vol, dev.vol.Home)

		// simulate unloading time
		if srv.mocked {
			time.Sleep(2 * time.Second)
		}

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

	return nil, errors.Errorf("unknown library: %s", libname)
}

type ErrShortWrite struct {
	Written int
}

func (e ErrShortWrite) Error() string {
	return fmt.Sprintf("short write: wrote %d bytes", e.Written)
}

// Store grabs an io.Reader, reads until EOF and stores the data on a tape.
func (srv *Server) Store(ctx context.Context, archive string, rd io.Reader) error {
	log.Printf("store: store archive: %s", archive)

	pol := policy.DefaultPolicy

	// see if there is a write policy associated
	if newpol, ok := policy.Unwrap(ctx); ok {
		pol = newpol
	}

	// create new stream
	stream := NewStream(archive, pol)

	// create a context with setup timeout if necessary
	var cancel context.CancelFunc
	if pol.ExclusiveTimeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, pol.ExclusiveTimeout)
	}

	if pol.Parallel() {
		if grp, ok := srv.groups[pol.WriteGroup]; ok {
			stream.out = grp.in
		}

		return errors.New("no such write group")
	} else {
		// Get a drive
		drv, err := AcquireDrive(ctx, srv.drives["write"], pol)
		if err != nil {
			return err
		}

		stream.out = drv.in
		stream.onclose = func() { drv.Release(context.Background()) }
	}

	if cancel != nil {
		cancel()
	}

	reader := bufio.NewReader(rd)

	buf := make([]byte, 1024)

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
			return errors.Wrap(err, "create archive failed")
		}

		return nil
	})
}

func AcquireDrive(ctx context.Context, pool []*Drive, pol *policy.Policy) (*Drive, error) {
	ch := make(chan *Drive)

	ctx, cancel := context.WithCancel(ctx)

	// send use request to all drives
	for _, drv := range pool {
		go func(drv *Drive) {
			if err := drv.Use(ctx, pol); err != nil {
				log.Printf("%v: %v", drv, err)
			}

			select {
			case <-ctx.Done():
				drv.Release(context.Background())
			case ch <- drv:
			}
		}(drv)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case drv := <-ch:
			// cancel other requests
			cancel()

			return drv, nil
		}
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

	log.Printf("GetScratch: new volume mounted at %s", mountpoint)
	if err := os.MkdirAll(mountpoint, os.ModePerm); err != nil {
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
