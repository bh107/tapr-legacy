package stream

type Stream struct {
	archive    []byte
	partial    *Chunk
	cnkCounter int

	errc chan error

	out chan *Chunk

	chunkpool *ChunkPool
}

func New(out chan *Chunk) *Stream {
	stream := &Stream{
		errc: make(chan error),
		out:  out,

		chunkpool: NewChunkPool(DefaultChunkSize),
	}

	return stream
}

func (s *Stream) AddWriter(wr *Writer) {

}

func (s *Stream) Add(p []byte) error {
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
			if err := s.Write(s.partial); err != nil {
				return err
			}

			s.partial = s.chunkpool.Get()

			// load remaining bytes into next chunk
			p = p[n:]
			continue
		}

		break
	}

	return nil
}

func (s *Stream) Close() error {
	return s.Write(s.partial)
}

func (s *Stream) Write(cnk *Chunk) error {
	cnk.upstream = s

	s.out <- cnk
	return <-s.errc
}
