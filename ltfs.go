package tapr

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/bh107/tapr/internal/util"
	"github.com/golang/glog"
)

const (
	LTFSSyncModeUnmount = "unmount"
)

// LTFS represents an LTFS formatted volume, possibly mounted.
type LTFS struct {
	drv        *Drive
	mountpoint string
	rawcmd     string
	synctype   string

	mounted bool
}

const (
	mountCmd   string = "/usr/local/bin/ltfs"
	unmountCmd string = "fusermount -u"
	mkltfsCmd  string = "/usr/local/bin/mkltfs"
)

var (
	ErrNotMounted = errors.New("ltfs: volume not mounted")
)

// NewLTFS mounts a new LTFS filesystem
func NewLTFS(drv *Drive, root string, synctype string, format bool) (*LTFS, error) {
	mountpoint := path.Join(root, drv.vol.Serial)

	if err := os.MkdirAll(mountpoint, os.ModePerm); err != nil { // XXX fix modeperm
		return nil, err
	}

	// build ltfs command line
	rawcmd := fmt.Sprintf("ltfs %s -o direct_io -o sync_type=%s -o devname=%s -o log_directory=/tmp",
		mountpoint, synctype, drv.path,
	)

	ltfs := &LTFS{
		drv:        drv,
		mountpoint: mountpoint,
		rawcmd:     rawcmd,
		synctype:   synctype,
	}

	if format {
		if err := ltfs.format(); err != nil {
			return nil, err
		}
	}

	if err := ltfs.mount(); err != nil {
		return nil, err
	}

	return ltfs, nil
}

func (ltfs *LTFS) format() error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s -d %s", mkltfsCmd, ltfs.drv.path))

	err := util.ExecCmd(cmd)
	if err != nil {
		return err
	}

	return nil
}

func (ltfs *LTFS) mount() error {
	glog.Infof("mounting volume %s on %s", ltfs.drv.vol, ltfs.mountpoint)
	cmd := exec.Command("sh", "-c", ltfs.rawcmd)

	err := util.ExecCmd(cmd)
	if err != nil {
		return err
	}

	ltfs.mounted = true

	return nil
}

// unix umount returns immediately. must wait for the ltfs process to
// terminate. using f*cking pgrep. sigh.
func (ltfs *LTFS) unmount() error {
	glog.Infof("unmounting volume %s on %s", ltfs.drv.vol, ltfs.mountpoint)
	cmd := exec.Command(unmountCmd, ltfs.mountpoint)

	err := util.ExecCmd(cmd)
	if err != nil {
		return err
	}

	ltfs.mounted = false

	// wait for ltfs to terminate. sigh.
	waiter := exec.Command("sh", "-c",
		fmt.Sprintf("'while pgrep -xf \"%s\" > /dev/null; do sleep 1; done'",
			ltfs.rawcmd,
		),
	)

	err = util.ExecCmd(waiter)
	if err != nil {
		return err
	}

	// clean up (bails out if the directory contains stuff, phew)
	if err := os.Remove(ltfs.mountpoint); err != nil {
		return err
	}

	glog.Infof("volume %s unmounted", ltfs.drv.vol)

	return nil
}
