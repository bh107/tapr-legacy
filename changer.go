package tapr

import (
	"os/exec"
	"strconv"
	"sync"

	"github.com/bh107/tapr/internal/util"
)

type Changer struct {
	sync.Mutex

	path string
}

func (chgr *Changer) String() string {
	return chgr.path
}

// execute fn with exclusive use of the changer
func (chgr *Changer) use(fn func(*Tx) error) error {
	defer chgr.Unlock()
	chgr.Lock()

	tx := &Tx{chgr: chgr}

	if err := fn(tx); err != nil {
		return err
	}

	return nil
}

type Tx struct {
	chgr *Changer
}

func (tx *Tx) status() ([]byte, error) {
	return exec.Command(mtxCmd, "-f", tx.chgr.path, "status").Output()
}

func (tx *Tx) load(slot int, drivenum int) error {
	cmd := exec.Command(mtxCmd, "-f", tx.chgr.path, "load", strconv.Itoa(slot), strconv.Itoa(drivenum))

	return util.ExecCmd(cmd)
}

func (tx *Tx) unload(slot int, drivenum int) error {
	cmd := exec.Command(mtxCmd, "-f", tx.chgr.path, "unload", strconv.Itoa(slot), strconv.Itoa(drivenum))

	return util.ExecCmd(cmd)
}
