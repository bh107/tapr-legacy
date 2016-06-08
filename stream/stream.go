package stream

import "github.com/bh107/tapr/stream/policy"

// Stream represents a byte stream going to backend storage.
type Stream struct {
	archive    []byte
	partial    *Chunk
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

// Write writes bytes to the stream. Chunks are only flushed to backend storage
// when they reach DefaultChunkSize.
func (s *Stream) Write(p []byte) (n int, err error) {
	if s.out == nil {
		panic("s.out was nil")
	}

	// try to assemble a chunk
	for {
		if len(p) == 0 {
			break
		}

		n := s.partial.add(p)

		if n != len(p) {
			s.cnkCounter++
			s.partial.id = s.cnkCounter

			// attempt to write chunk
			if err := s.writeChunk(s.partial); err != nil {
				return n, err
			}

			s.partial = s.chunkpool.Get()

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
func (s *Stream) Close() error {
	return s.writeChunk(s.partial)
}

func (s *Stream) writeChunk(cnk *Chunk) error {
	cnk.upstream = s

	s.out <- cnk
	return <-s.errc
}
