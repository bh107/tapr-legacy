package library

import (
	"errors"
	"os/exec"

	"github.com/bh107/tapr/library/changer"
	"github.com/bh107/tapr/util/mtx"
	"github.com/bh107/tapr/util/proc"
	"golang.org/x/net/context"
)

type AutomatedLibrary interface {
	// Name returns the identification of the library.
	Name() string

	// Audit performs an audit (full inventory check) of the library.
	Audit(ctx context.Context) (*mtx.StatusInfo, error)

	/*
		// Load loads a volume from a storage slot into a drive.
		Load(context.Context, drive.Loader, *mtx.Volume) error

		// Unload unloads a volume from a drive back to its storage slot.
		Unload(context.Context, drive.Unloader, *mtx.Volume) error
	*/
}

type mtxLibrary struct {
	*proc.Proc

	name string
	chgr *changer.Changer
	errc chan error

	driveStats map[string]*DriveStatistics
}

func New(name string) AutomatedLibrary {
	lib := &mtxLibrary{
		name: name,
	}

	lib.Proc = proc.Create(lib)

	return lib
}

func (lib *mtxLibrary) Name() string {
	return lib.name
}

func (lib *mtxLibrary) ProcessName() string {
	return "library"
}

func (lib *mtxLibrary) Handle(ctx context.Context, req proc.HandleFn) error {
	return req(ctx)
}

// Audit performs an audit (full inventory check) of the library.
func (lib *mtxLibrary) Audit(ctx context.Context) (*mtx.StatusInfo, error) {
	var status *mtx.StatusInfo

	req := func(ctx context.Context) error {
		var err error
		status, err = lib.chgr.Status(ctx)
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				return errors.New(string(exitError.Stderr))
			}

			return err
		}

		// we do all auditing inside the changer lock
		//err = srv.inv.Audit(status, libname)
		return err
	}

	if err := lib.Wait(ctx, req); err != nil {
		return nil, err
	}

	return status, nil
}

/*
// Scratch returns a new scratch volume.
func (lib *mtxLibrary) Scratch(drv *Drive) (*mtx.Volume, error) {
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

	log.Printf("new volume %v mounted at %s", vol, mountpoint)
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

func (lib *mtxLibrary) Load(dev *Drive, vol *mtx.Volume) error {
	if dev.vol != nil {
		if dev.vol.Serial == vol.Serial {
			log.Printf("load: drive %s already loaded with %s", dev, vol)
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

func (lib *mtxLibrary) Unload(dev *Drive) error {
	if dev.vol == nil {
		log.Printf("unload: drive %s already unloaded", dev)
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
}*/
