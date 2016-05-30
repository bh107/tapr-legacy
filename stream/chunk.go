package stream

import "sync"

const DefaultChunkSize = 64 * 1 << 20

type ChunkPool struct {
	pool *sync.Pool
}

func NewChunkPool(chunksize int) *ChunkPool {
	return &ChunkPool{
		pool: &sync.Pool{
			New: func() interface{} { return NewChunk(DefaultChunkSize) },
		},
	}
}

func (cnkpool *ChunkPool) Get() *Chunk {
	return cnkpool.pool.Get().(*Chunk)
}

func (cnkpool *ChunkPool) Put(cnk *Chunk) {
	cnkpool.pool.Put(cnk)
}

type Chunk struct {
	id       int
	upstream *Stream

	buf []byte
}

func NewChunk(size int) *Chunk {
	return &Chunk{
		buf: make([]byte, 0, size),
	}
}

func (cnk *Chunk) add(p []byte) (n int) {
	free := cap(cnk.buf) - len(cnk.buf)
	if len(p) > free {
		p = p[:free]
	}

	copy(cnk.buf[len(cnk.buf):], p)

	return len(p)
}

func (cnk *Chunk) reset() {
	cnk.upstream = nil

	cnk.buf = cnk.buf[:0]
}
