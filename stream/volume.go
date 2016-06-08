package stream

import (
	"fmt"
	"os"
	"path"

	"github.com/bh107/tapr/mtx"
)

type ErrIO struct {
	err error
	cnk *Chunk
}

func (e ErrIO) Error() string {
	return fmt.Sprintf("i/o error: %s", e.err)
}

// Volume represents a read- or writable media.
type Volume struct {
	root      string
	globalSeq int

	media *mtx.Volume

	in  chan *Chunk
	agg chan *Chunk

	errc chan error
}

// NewVolume returns a new Volume and starts the communicating process.
func NewVolume(root string, media *mtx.Volume, in, agg chan *Chunk) *Volume {
	vol := &Volume{
		root: path.Join(root, media.Serial),

		// in channel for direct/exclusive access
		in:  in,
		agg: agg,
	}

	go vol.run()

	return vol
}

func (vol *Volume) run() {
	var err error
	var cnk *Chunk

	// grab chunks from all streams
	for {
		select {
		case cnk = <-vol.in:
		case cnk = <-vol.agg:
		}

		vol.globalSeq++

		// generate filename
		fname := fmt.Sprintf("%07d-%s.cnk%07d",
			vol.globalSeq, string(cnk.upstream.archive),
			cnk.id,
		)

		f, err := os.Create(path.Join(vol.root, fname))
		if err != nil {
			break
		}

		if _, err = f.Write(cnk.buf); err != nil {
			break
		}

		if err = f.Close(); err != nil {
			break
		}

		// report success (no error), bypassing drive
		cnk.upstream.errc <- nil

		// reset and return chunk to stream chunk pool
		cnk.reset()
		cnk.upstream.chunkpool.Put(cnk)
	}

	vol.errc <- ErrIO{err, cnk}
}
