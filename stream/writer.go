package stream

import (
	"fmt"
	"os"
)

type Creatable interface {
	Create(string) (*os.File, error)
}

type Writer struct {
	handle      Creatable
	in          chan *Chunk
	terminating chan struct{}

	chunkpool *ChunkPool
}

func NewWriter(handle Creatable) *Writer {
	wr := &Writer{
		handle: handle,
		in:     make(chan *Chunk),
	}

	go wr.loop()

	return wr
}

func (wr *Writer) loop() {
	var err error
	var cnk *Chunk

	var volumeGlobalChunkId int

	// grab chunks from all streams
	for cnk = range wr.in {
		volumeGlobalChunkId++

		// generate filename
		fname := fmt.Sprintf("%07d-%s.cnk%07d",
			volumeGlobalChunkId, string(cnk.upstream.archive),
			cnk.id,
		)

		f, err := wr.handle.Create(fname)
		if err != nil {
			break
		}

		if _, err = f.Write(cnk.buf); err != nil {
			break
		}

		if err = f.Close(); err != nil {
			break
		}

		// report success (no error)
		cnk.upstream.errc <- nil

		// reset and return chunk to pool
		cnk.reset()
		wr.chunkpool.Put(cnk)
	}

	// report any possible error
	cnk.upstream.errc <- err

	// announce that we won't be receiving anymore values
	close(wr.terminating)

	return
}
