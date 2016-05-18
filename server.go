package tapr

import (
	"bufio"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/kbj/mtx"
	"github.com/kbj/mtx/mock"
)

var chunkdb *bolt.DB
var inv *Inventory

type Server struct {
	libraries map[string]*Library
	ltfsRoot  string
	xferStats xferStats
}

func New(configpath string) (*Server, error) {
	config, err := loadConfig("./config.toml")
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("sqlite3", config.Invdb)
	if err != nil {
		return nil, err
	}

	chunkdb, err = bolt.Open(config.Chunkdb, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	// create the global archive list
	if err := chunkdb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("archivelist"))

		return err
	}); err != nil {
		return nil, err
	}

	inv = &Inventory{db: db}

	srv := &Server{
		libraries: make(map[string]*Library),
		ltfsRoot:  config.Mountroot,
		xferStats: xferStats{},
	}

	// initialize libraries
	for k, v := range config.Libraries {
		lib := NewLibrary(k, &Changer{Changer: mtx.NewChanger(mock.New(8, 32, 4, 16))})

		// writers
		for slot, path := range v.Drives {
			tmp := strings.Split(path, ":")
			if len(tmp) != 2 {
				return nil, fmt.Errorf("could not parse device: %s", path)
			}

			devtype := tmp[0]
			path := tmp[1]

			lib.drives = append(lib.drives, NewDrive(path, devtype, slot, lib))
		}

		srv.libraries[k] = lib

		status, err := srv.Audit(k)
		if err != nil {
			return nil, err
		}

		for i, elem := range status.Drives {
			if i > len(lib.drives)-1 {
				break
			}

			if elem.Vol != nil {
				glog.Infof("drive %s has volume %s loaded", lib.drives[i], elem.Vol)
			}

			lib.drives[i].vol = elem.Vol
		}

		for _, drv := range srv.libraries[k].drives {
			if drv.devtype == "write" {
				glog.Infof("starting chunk writer on %s in %s library", drv, k)
				if drv.wr, err = NewChunkWriter(srv, drv, 32); err != nil {
					return nil, err
				}
			}
		}
	}

	return srv, nil
}

func (srv *Server) Volumes(libname string) ([]*mtx.Volume, error) {
	return inv.volumes(libname)
}

func (srv *Server) Load(dev *Drive, vol *mtx.Volume) error {
	if dev.vol != nil {
		if dev.vol.Serial == vol.Serial {
			glog.Infof("drive %s already loaded with %s", dev, vol)
			return nil
		}
	}

	err := dev.lib.chgr.use(func(tx *Tx) error {
		glog.Infof("loading drive %s with volume %s from slot %d", dev, vol, vol.Home)
		var err error
		err = tx.load(vol.Home, dev.slot)
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
		glog.Infof("drive %s already unloaded", dev)
		return nil
	}

	err := dev.lib.chgr.use(func(tx *Tx) error {
		glog.Infof("unloading drive %s, returning volume %s to slot %d", dev, dev.vol, dev.vol.Home)
		var err error
		err = tx.unload(dev.vol.Home, dev.slot)
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

func (srv *Server) Audit(libname string) (*mtx.Status, error) {
	if lib, ok := srv.libraries[libname]; ok {
		glog.Infof("auditing %s library", libname)

		var status *mtx.Status
		err := lib.chgr.use(func(tx *Tx) error {
			var err error
			status, err = tx.status()
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					return errors.New(string(exitError.Stderr))
				}

				return err
			}

			// we do all auditing inside the changer lock
			err = inv.audit(status, libname)
			return err
		})

		glog.Infof("finished auditing %s library", libname)

		if err != nil {
			return nil, err
		}

		return status, nil
	}

	return nil, fmt.Errorf("unknown library: %s", libname)
}

func (srv *Server) Retrieve(wr io.Writer, name string) error {
	archive, err := srv.chunks(name)
	if err != nil {
		return err
	}

	vols := archive.Volumes()

	for _, vol := range vols {
		// locate the slot this volume is in
		lib, err := inv.locate(vol)
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

				ltfs, err := NewLTFS(drv, srv.ltfsRoot, LTFSSyncModeUnmount, false)
				if err != nil {
					return fmt.Errorf("could not mount LTFS file system: %s", err)
				}

				for e := archive.chunks.Front(); e != nil; e = e.Next() {
					cnk := e.Value.(*Chunk)
					_ = cnk
				}

				if err := ltfs.unmount(); err != nil {
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

// Store grabs an io.Reader, reads until EOF and stores the data on a tape.
func (srv *Server) Store(archive string, rd io.Reader, policy *Policy) error {
	glog.Infof("store archive: %s", archive)

	drv := srv.libraries["primary"].drives[0]

	// get chunkwriter
	stream := NewStream(archive, drv.wr)

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

		if err := stream.Add(buf); err != nil {
			return err
		}

		if err != nil && err != io.EOF {
			return err
		}
	}

	if err := stream.Close(); err != nil {
		return err
	}

	return nil
}

func (srv *Server) Shutdown() {
	glog.Info("shutdown: starting...")
	chunkdb.Close()
	glog.Info("shutdown: chunkdb closed")
	inv.db.Close()
	glog.Info("shutdown: inv closed")

	glog.Info("shutdown: stats")
	glog.Infof("+ total bytes transfered: %d", srv.xferStats.totalBytes)
	glog.Infof("+ total transfer time: %v", srv.xferStats.total)
}

// Return a slice of chunks of the archive
func (srv *Server) chunks(name string) (*Archive, error) {
	ar := NewArchive(name)

	err := chunkdb.View(func(tx *bolt.Tx) error {
		if bkt := tx.Bucket([]byte(name)); bkt != nil {
			c := bkt.Cursor()

			// iterate in byte-order
			for k, v := c.First(); k != nil; k, v = c.Next() {
				id, _ := binary.Varint(k)

				ar.chunks.PushBack(&Chunk{
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

func (srv *Server) Create(archive string) error {
	return chunkdb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(archive))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		return nil
	})
}
