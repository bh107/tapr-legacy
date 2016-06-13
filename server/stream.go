package server

import (
	"log"

	"github.com/bh107/tapr/stream/policy"
	"golang.org/x/net/context"
)

// Stream represents a byte stream going to backend storage.
type Stream struct {
	archive    string
	tmp        *Chunk
	cnkCounter int
	pol        *policy.Policy
	drv        *Drive

	errc chan error

	out chan *Chunk

	chunkpool *ChunkPool
}

// New creates a new byte stream.
func NewStream(name string, pol *policy.Policy) *Stream {
	s := &Stream{
		archive: name,
		errc:    make(chan error),
		pol:     pol,

		chunkpool: NewChunkPool(DefaultChunkSize),
	}

	s.tmp = s.chunkpool.Get()

	return s
}

func (s *Stream) Policy() *policy.Policy {
	return s.pol
}

func (s *Stream) Errc() chan error {
	return s.errc
}

// Write writes bytes to the stream. Chunks are only flushed to backend storage
// when they reach DefaultChunkSize.
func (s *Stream) Write(ctx context.Context, p []byte) (n int, err error) {
	if s.out == nil {
		panic("s.out was nil")
	}

	// try to assemble a chunk
	for {
		if len(p) == 0 {
			break
		}

		n := s.tmp.add(p)

		if n != len(p) {
			// attempt to write chunk
			if err := s.writeChunk(ctx, s.tmp); err != nil {
				return n, err
			}

			s.tmp = s.chunkpool.Get()

			// load remaining bytes into next chunk
			p = p[n:]
			continue
		}

		break
	}

	return len(p), nil
}

// Close closes the current stream and flushed the partial chunk to backend
// storage.
func (s *Stream) Close(ctx context.Context) error {
	if err := s.writeChunk(ctx, s.tmp); err != nil {
		return err
	}

	log.Printf("closing, releasing %v", s.drv)
	s.drv.Release()

	return nil
}

func (s *Stream) writeChunk(ctx context.Context, cnk *Chunk) error {
	s.cnkCounter++
	s.tmp.id = s.cnkCounter

	cnk.upstream = s

	s.out <- cnk

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-s.errc:
		if err != nil {
			return err
		}
	}

	// reset and return chunk to stream chunk pool
	cnk.reset()
	s.chunkpool.Put(cnk)

	return nil
}
