// Package scsi implements the mtx.Interface for a scsi library auto changer by
// using the 'mtx' program.
package scsi

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Changer represents a library changer managed by the 'mtx' program.
type Changer struct {
	path string
	prog string
}

// New returns a new changer implementation using 'mtx' for library operations.
func New(path string) *Changer {
	return &Changer{
		path: path,
		prog: "/usr/bin/mtx",
	}
}

// Do performs the given operation.
func (chgr *Changer) Do(args ...string) ([]byte, error) {
	// this is a little bit wonky Go...
	params := append([]string{"-f", chgr.path}, args...)

	return run(exec.Command(chgr.prog, params...))
}

func run(cmd *exec.Cmd) ([]byte, error) {
	var stderr bytes.Buffer

	if cmd.Stderr == nil {
		cmd.Stderr = &stderr
	}

	out, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return out, fmt.Errorf("%s: %s", exitError, stderr.String())
		}

		return out, err
	}

	return out, nil
}
