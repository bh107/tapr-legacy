package stream

import "errors"

type Stream struct {
	archive    []byte
	partial    *Chunk
	cnkCounter int

	writer *Writer

	errc chan error
}

func New(wr *Writer) *Stream {
	return &Stream{
		writer: wr,

		errc: make(chan error),
	}
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

			s.partial = s.writer.chunkpool.Get()

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

	select {
	case s.writer.in <- cnk:
		return <-s.errc
	case <-s.writer.terminating:
		return errors.New("writer terminated unexpectedly")
	}
}
