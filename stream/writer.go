package stream

import (
	"fmt"
	"os"
	"path"

	"github.com/bh107/tapr/mtx"
)

type ErrIO struct {
	Err   error
	Chunk *Chunk
}

func (e ErrIO) Error() string {
	return fmt.Sprintf("i/o error: %s", e.Err)
}

// Writer represents a writable media.
type Writer struct {
	root      string
	globalSeq int

	media *mtx.Volume

	in  chan *Chunk
	agg chan *Chunk

	errc chan error
}

// NewWriter returns a new Writer and starts the communicating process.
func NewWriter(root string, media *mtx.Volume, agg chan *Chunk) *Writer {
	wr := &Writer{
		root: path.Join(root, media.Serial),

		// in channel for direct/exclusive access
		in: make(chan *Chunk),

		// for parallel write
		agg: agg,
	}

	go wr.run()

	return wr
}

func (wr *Writer) Errc() chan error {
	return wr.errc
}

func (wr *Writer) Ingress() (in, agg chan *Chunk) {
	return wr.in, wr.agg
}

func (wr *Writer) run() {
	var err error
	var cnk *Chunk

	// Grab chunks from all streams
	//
	// If any error is detected, it is reported back to the drive process.
	// Success goes directly to the stream/client process.
	for {
		select {
		case cnk = <-wr.in:
		case cnk = <-wr.agg:
		}

		wr.globalSeq++

		// generate filename
		fname := fmt.Sprintf("%07d-%s.cnk%07d",
			wr.globalSeq, string(cnk.upstream.archive),
			cnk.id,
		)

		f, err := os.Create(path.Join(wr.root, fname))
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
	}

	// report error to the drive process
	wr.errc <- ErrIO{err, cnk}
}
