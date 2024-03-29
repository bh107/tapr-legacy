package util

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os/exec"
)

// Itob returns an 8-byte big endian representation of v.
func Itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func Run(cmd *exec.Cmd) ([]byte, error) {
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
