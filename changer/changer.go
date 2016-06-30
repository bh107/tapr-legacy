package changer

import (
	"sync"

	"github.com/bh107/tapr/util/mtx"
	"github.com/bh107/tapr/util/mtx/mock"
	"github.com/bh107/tapr/util/mtx/scsi"
)

type Changer struct {
	mtx.Interface

	mu sync.Mutex
}

func New(path string) *Changer {
	return newChanger(path, false)
}

func Mock(path string) *Changer {
	return newChanger(path, true)
}

func newChanger(path string, mocked bool) *Changer {
	var impl mtx.Interface
	if mocked {
		impl = mock.New(path)
	} else {
		impl = scsi.New(path)
	}

	chgr := &Changer{
		Interface: impl,
	}

	return chgr
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

func (tx *Tx) Status() (*mtx.StatusInfo, error) {
	return mtx.Status(tx.chgr)
}

func (tx *Tx) Load(slot int, drivenum int) error {
	return mtx.Load(tx.chgr, slot, drivenum)
}

func (tx *Tx) Unload(slot int, drivenum int) error {
	return mtx.Unload(tx.chgr, slot, drivenum)
}
