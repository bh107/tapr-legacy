package stream

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

	onclose func()

	errc chan error

	out chan *Chunk

	chunkpool *ChunkPool
}

// New creates a new byte stream.
func New(name string, pol *policy.Policy) *Stream {
	s := &Stream{
		archive: name,
		errc:    make(chan error),
		pol:     pol,

		chunkpool: NewChunkPool(DefaultChunkSize),
	}

	s.tmp = s.chunkpool.Get()

	return s
}

func (s *Stream) String() string {
	return s.archive
}

func (s *Stream) Parallel() bool {
	return s.pol.WriteGroup != ""
}

func (s *Stream) Policy() *policy.Policy {
	return s.pol
}

func (s *Stream) Errc() chan<- error {
	return s.errc
}

func (s *Stream) SetOut(out chan *Chunk) {
	s.out = out
}

func (s *Stream) OnClose(fn func()) {
	s.onclose = fn
}

// Write writes bytes to the stream. Chunks are only flushed to backend storage
// when they reach DefaultChunkSize.
func (s *Stream) Write(ctx context.Context, p []byte, ack bool) (n int, err error) {
	// try to assemble a chunk
	for {
		if len(p) == 0 {
			break
		}

		n := s.tmp.add(p)

		if n != len(p) {
			// attempt to write chunk
			if err := s.writeChunk(ctx, s.tmp, ack); err != nil {
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
	s.tmp.last = true

	// write the partial chunk, if any
	if err := s.writeChunk(ctx, s.tmp, true); err != nil {
		return err
	}

	log.Print("closing stream")

	go s.onclose()

	return nil
}

func (s *Stream) writeChunk(ctx context.Context, cnk *Chunk, ack bool) error {
	s.cnkCounter++
	s.tmp.id = s.cnkCounter

	cnk.upstream = s

	// send chunk
	s.out <- cnk

	if ack {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-s.errc:
			if err != nil {
				return err
			}
		}
	}

	return nil
}
