package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"

	"golang.org/x/net/context"

	"github.com/bh107/tapr/changer"
	"github.com/bh107/tapr/config"
	"github.com/bh107/tapr/inventory"
	"github.com/bh107/tapr/mtx"
	"github.com/bh107/tapr/stream"

	"github.com/boltdb/bolt"
)

type Server struct {
	libraries map[string]*Library
	cfg       *config.Config

	chunkdb *bolt.DB
	inv     *inventory.DB
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

	// initialize libraries
	for _, cfgLib := range cfg.Libraries {
		log.Printf("init: adding library:   \"%s\"", cfgLib.Name)
		lib := NewLibrary(cfgLib.Name)

		for _, cfgChgr := range cfgLib.Changers {
			log.Printf("init: + adding changer: %s", cfgChgr.Path)
			lib.chgr = changer.New(cfgChgr.Path, mock)
		}

		for _, cfgDrv := range cfgLib.Drives {
			log.Printf("init: + adding drive:   %s (%s)", cfgDrv.Path, cfgDrv.Type)
			drv := NewDrive(cfgDrv.Path, cfgDrv.Type, cfgDrv.Slot, lib)
			lib.drives[drv.devtype] = append(lib.drives[drv.devtype], drv)
		}

		srv.libraries[cfgLib.Name] = lib

		if audit {
			log.Printf("init: audit started for \"%s\" library", cfgLib.Name)
			_, err := srv.Audit(context.Background(), cfgLib.Name)
			if err != nil {
				return nil, err
			}
			log.Printf("init: audit finished for \"%s\" library", cfgLib.Name)
		}
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

/*
func (srv *Server) Retrieve(wr io.Writer, name string) error {
	archive, err := srv.chunks(name)
	if err != nil {
		return err
	}

	vols := archive.Volumes()

	for _, vol := range vols {
		// locate the slot this volume is in
		lib, err := srv.inv.Locate(vol)
		if err != nil {
			return err
		}

		// find a read drive to use
		for _, drv := range srv.libraries[lib].drives {
			if drv.devtype == "read" {
				if drv.vol != nil {
					// damn, read drive is already in use, is it already the correct volume?
					if drv.vol.Serial != vol.Serial {
						// no.. damn. No resource available right now then..
						return errors.New("resource unavailable")
					}
				}

				// load the volume into the drive
				if err := srv.Load(drv, vol); err != nil {
					return fmt.Errorf("could not load volume: %s", err)
				}

				handle, err := ltfs.New(drv.path)
				if err != nil {
					return fmt.Errorf("could not create LTFS handle: %s", err)
				}

				mountpoint := path.Join(srv.ltfsRoot, vol.Serial)
				if err := handle.Mount(mountpoint, ltfs.SyncModeUnmount); err != nil {
					return fmt.Errorf("failed to mount LTFS file system: %s", err)
				}

				for e := archive.chunks.Front(); e != nil; e = e.Next() {
					cnk := e.Value.(*stream.Chunk)
					_ = cnk
				}

				if err := handle.Unmount(); err != nil {
					return fmt.Errorf("could not unmount LTFS file system: %s", err)
				}

				if err := srv.Unload(drv); err != nil {
					return fmt.Errorf("could not unload volume: %s", err)
				}
			}
		}
	}

	return nil
}
*/

// Store grabs an io.Reader, reads until EOF and stores the data on a tape.
func (srv *Server) Store(archive string, rd io.Reader) error {
	log.Printf("store archive: %s", archive)

	drv := srv.libraries["primary"].drives["write"][0]

	// get chunkwriter
	stream := stream.New(drv.wr)

	r := bufio.NewReader(rd)

	buf := make([]byte, 4096)

	for {
		n, err := r.Read(buf[:cap(buf)])
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

		if err := stream.Add(buf); err != nil {
			return err
		}
	}

	if err := stream.Close(); err != nil {
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

// Return a slice of chunks of the archive
/*
func (srv *Server) chunks(name string) (*Archive, error) {
	ar := NewArchive(name)

	err := srv.chunkdb.View(func(tx *bolt.Tx) error {
		if bkt := tx.Bucket([]byte(name)); bkt != nil {
			c := bkt.Cursor()

			// iterate in byte-order
			for k, v := c.First(); k != nil; k, v = c.Next() {
				id, _ := binary.Varint(k)

				ar.chunks.PushBack(&stream.Chunk{
					id:      int(id),
					archive: ar,
					vol:     &mtx.Volume{Serial: string(v)},
				})
			}

			return nil
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ar, nil
}
*/
func (srv *Server) Create(archive string) error {
	return srv.chunkdb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(archive))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		return nil
	})
}
