package changer

import (
	"sync"

	"github.com/bh107/tapr/mtx"
	"github.com/bh107/tapr/mtx/mock"
	"github.com/bh107/tapr/mtx/scsi"
)

type Changer struct {
	*mtx.Changer

	mu sync.Mutex
}

func New(path string, mocked bool) *Changer {
	if mocked {
		return &Changer{
			Changer: mtx.NewChanger(mock.New(path)),
		}
	}

	return &Changer{
		Changer: mtx.NewChanger(scsi.New(path)),
	}
}

// execute fn with exclusive use of the changer
func (chgr *Changer) Use(fn func(*Tx) error) error {
	defer chgr.mu.Unlock()
	chgr.mu.Lock()

	tx := &Tx{chgr: chgr}

	if err := fn(tx); err != nil {
		return err
	}

	return nil
}

type Tx struct {
	chgr *Changer
}

func (tx *Tx) Status() (*mtx.Status, error) {
	return tx.chgr.Status()
}

func (tx *Tx) Load(slot int, drivenum int) error {
	return tx.chgr.Load(slot, drivenum)
}

func (tx *Tx) Unload(slot int, drivenum int) error {
	return tx.chgr.Unload(slot, drivenum)
}
