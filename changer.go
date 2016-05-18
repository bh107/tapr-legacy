package tapr

import (
	"sync"

	"github.com/kbj/mtx"
)

type Changer struct {
	*mtx.Changer

	mu sync.Mutex
}

// execute fn with exclusive use of the changer
func (chgr *Changer) use(fn func(*Tx) error) error {
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

func (tx *Tx) status() (*mtx.Status, error) {
	return tx.chgr.Status()
}

func (tx *Tx) load(slot int, drivenum int) error {
	return tx.chgr.Load(slot, drivenum)
}

func (tx *Tx) unload(slot int, drivenum int) error {
	return tx.chgr.Unload(slot, drivenum)
}
