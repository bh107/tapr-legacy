package stream

import (
	"github.com/bh107/tapr/stream/policy"
	"golang.org/x/net/context"
)

// Stream represents a byte stream going to backend storage.
type Stream struct {
	archive    []byte
	tmp        *Chunk
	cnkCounter int
	pol        *policy.Policy

	errc chan error

	out chan *Chunk

	chunkpool *ChunkPool
}

// New creates a new byte stream.
func New(pol *policy.Policy) *Stream {
	stream := &Stream{
		errc: make(chan error),
		pol:  pol,

		chunkpool: NewChunkPool(DefaultChunkSize),
	}

	return stream
}

func (s *Stream) Policy() *policy.Policy {
	return s.pol
}

func (s *Stream) Errc() chan error {
	return s.errc
}

func (s *Stream) Out() chan *Chunk {
	return s.out
}

func (s *Stream) SetOut(ch chan *Chunk) {
	s.out = ch
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
			s.cnkCounter++
			s.tmp.id = s.cnkCounter

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
	return s.writeChunk(ctx, s.tmp)
}

func (s *Stream) writeChunk(ctx context.Context, cnk *Chunk) error {
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
	cnk.upstream.chunkpool.Put(cnk)

	return nil
}
