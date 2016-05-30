// Package ltfs functions as a wrapper around the LTFS binaries.
package ltfs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/bh107/tapr/osutil"
)

const (
	SyncModeUnmount = "unmount"
	SyncModeTime    = "time@5min"
	SyncModeClose   = "close"
)

// Handle represents an LTFS formatted volume, possibly mounted.
type Handle struct {
	mountpoint string

	devpath  string
	rawcmd   string
	synctype string

	mounted bool
}

const (
	ltfsBinary  string = "/usr/local/bin/ltfs"
	mountCmdFmt string = "%s %s -o direct_io -o sync_type=%s -o devname=%s -o log_directory=/tmp"
	unmountCmd  string = "fusermount -u"
	mkltfsCmd   string = "/usr/local/bin/mkltfs"
)

var (
	ErrNotMounted = errors.New("ltfs: volume not mounted")
)

// New returns a new LTFS handle and formats the volume at the device if
// requested.
func New(devpath string) (*Handle, error) {
	finfo, err := os.Stat(devpath)
	if err != nil {
		return nil, err
	}

	if finfo.IsDir() {
		return nil, fmt.Errorf("%s is a directory", devpath)
	}

	ltfs := &Handle{
		devpath: devpath,
	}

	return ltfs, nil
}

func (h *Handle) Format() error {
	if h.mounted {
		return errors.New("volume mounted")
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s -d %s", mkltfsCmd, h.devpath))

	_, err := osutil.Run(cmd)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handle) Mount(path string, mode string) error {
	finfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !finfo.IsDir() {
		return fmt.Errorf("%s does not exist", path)
	}

	// build ltfs command line
	h.rawcmd = fmt.Sprintf(mountCmdFmt, ltfsBinary, path, mode, h.devpath)
	h.mountpoint = path

	cmd := exec.Command("sh", "-c", h.rawcmd)

	_, err = osutil.Run(cmd)
	if err != nil {
		return err
	}

	h.mounted = true

	return nil
}

func (h *Handle) Create(filepath string) (*os.File, error) {
	return os.Create(path.Join(h.mountpoint, filepath))
}

// unix umount returns immediately. must wait for the ltfs process to
// terminate. using f*cking pgrep. sigh.
func (h *Handle) Unmount() error {
	cmd := exec.Command(unmountCmd, h.mountpoint)

	_, err := osutil.Run(cmd)
	if err != nil {
		return err
	}

	h.mounted = false

	// wait for ltfs to terminate. sigh.
	waiter := exec.Command("sh", "-c",
		fmt.Sprintf("'while pgrep -xf \"%s\" > /dev/null; do sleep 1; done'",
			h.rawcmd,
		),
	)

	_, err = osutil.Run(waiter)
	if err != nil {
		return err
	}

	return nil
}
