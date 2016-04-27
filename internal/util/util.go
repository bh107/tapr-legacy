package util

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os/exec"
)

// itob returns an 8-byte big endian representation of v.
func Itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func ExecCmd(cmd *exec.Cmd) error {
	var stderr bytes.Buffer

	if cmd.Stderr == nil {
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("%s: %s", exitError, stderr.String())
		}

		return err
	}

	return nil
}
