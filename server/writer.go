package server

import (
	"fmt"
	"log"
	"os"
	"path"
	"syscall"

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
	total     int

	errc chan error

	media *mtx.Volume
	drv   *Drive

	in  chan *Chunk
	agg chan *Chunk
}

// NewWriter returns a new Writer and starts the communicating process.
func NewWriter(root string, media *mtx.Volume, in chan *Chunk, agg chan *Chunk, drv *Drive) *Writer {
	wr := &Writer{
		root: root,

		// in channel for direct/exclusive access
		in:  in,
		agg: agg,

		errc: make(chan error),

		drv: drv,
	}

	go wr.run()

	return wr
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

		wr.total += len(cnk.buf)
		if wr.total > (1024 * 64) {
			wr.errc <- ErrIO{syscall.ENOSPC, cnk}
			break
		}

		var f *os.File
		f, err = os.Create(path.Join(wr.root, fname))
		if err != nil {
			wr.errc <- ErrIO{err, cnk}
			break
		}

		// write takes some time
		//time.Sleep(1 * time.Second)
		if _, err = f.Write(cnk.buf); err != nil {
			wr.errc <- ErrIO{err, cnk}
			break
		}

		if err = f.Close(); err != nil {
			wr.errc <- ErrIO{err, cnk}
			break
		}

		log.Printf("writer[%v]: succesfully wrote %s", wr.drv, fname)

		// report success (no error), bypassing drive
		cnk.upstream.errc <- nil
	}
}
