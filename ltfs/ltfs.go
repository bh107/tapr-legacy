// Package ltfs functions as a wrapper around the LTFS binaries.
//
// +build linux
package ltfs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"

	"github.com/bh107/tapr/util"
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

	proc *os.Process
}

const (
	ltfsCmd    string = "/usr/local/bin/ltfs"
	unmountCmd string = "fusermount -u"
	mkltfsCmd  string = "/usr/local/bin/mkltfs"
)

var (
	ErrNotMounted = errors.New("ltfs: volume not mounted")
)

// New returns a new LTFS handle.
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

	cmd := exec.Command(mkltfsCmd, fmt.Sprintf("-d %s", h.devpath))

	_, err := util.Run(cmd)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handle) Mount(mountpoint string, mode string) error {
	finfo, err := os.Stat(mountpoint)
	if err != nil {
		return err
	}

	if !finfo.IsDir() {
		return fmt.Errorf("%s does not exist", mountpoint)
	}

	h.mountpoint = mountpoint

	ltfsOptions := []string{
		mountpoint,

		fmt.Sprintf("-o devname=%s", h.devpath),
		fmt.Sprintf("-o sync_type=%s", mode),

		"-o direct_io", "-o log_drectory=/tmp",
	}

	cmd := exec.Command(ltfsCmd, ltfsOptions...)

	_, err = util.Run(cmd)
	if err != nil {
		return err
	}

	h.mounted = true

	// The LTFS process goes to the background, not reporting the PID. So, do
	// some evil stuff and find it.
	out, err := exec.Command("pgrep", "-f", fmt.Sprintf("ltfs %s", h.mountpoint)).Output()
	if err != nil {
		panic(err)
	}

	pid, err := strconv.Atoi(string(out[:len(out)-1]))
	if err != nil {
		panic(err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		panic(err)
	}

	h.proc = proc

	return nil
}

func (h *Handle) Create(filepath string) (*os.File, error) {
	return os.Create(path.Join(h.mountpoint, filepath))
}

// unix umount returns immediately. must wait for the ltfs process to
// terminate. using f*cking pgrep. sigh.
func (h *Handle) Unmount() error {
	cmd := exec.Command(unmountCmd, h.mountpoint)

	_, err := util.Run(cmd)
	if err != nil {
		return err
	}

	state, err := h.proc.Wait()
	if err != nil {
		return err
	}

	if !state.Success() {
		status := state.Sys().(syscall.WaitStatus)
		return fmt.Errorf("ltfs process exited with status %d", status.ExitStatus())
	}

	h.mounted = false

	return nil
}
