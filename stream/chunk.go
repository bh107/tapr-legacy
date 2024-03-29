package stream

import "sync"

// DefaultChunkSize defines the well.... default chunk size. By default it is
// 64 megabytes.
const DefaultChunkSize = 4 * 1 << 20

// ChunkPool abstracts a sync.Pool for Chunks.
type ChunkPool struct {
	pool *sync.Pool
}

// NewChunkPool returns a new ChunkPool.
func NewChunkPool(chunksize int) *ChunkPool {
	return &ChunkPool{
		pool: &sync.Pool{
			New: func() interface{} { return NewChunk(chunksize) },
		},
	}
}

// Get retrieves a possibly new Chunk from the ChunkPool.
func (cnkpool *ChunkPool) Get() *Chunk {
	return cnkpool.pool.Get().(*Chunk)
}

// Put returns a Chunk to the ChunkPool.
func (cnkpool *ChunkPool) Put(cnk *Chunk) {
	cnkpool.pool.Put(cnk)
}

// Chunk represents a block of data to be committed to backend store in one
// operation.
type Chunk struct {
	id       int
	upstream *Stream
	last     bool
	pool     *ChunkPool

	buf []byte
}

// NewChunk creates a new preallocated Chunk.
func NewChunk(size int) *Chunk {
	return &Chunk{
		buf: make([]byte, 0, size),
	}
}

func (cnk *Chunk) Upstream() *Stream {
	return cnk.upstream
}

func (cnk *Chunk) add(p []byte) (n int) {
	free := cap(cnk.buf) - len(cnk.buf)
	if len(p) > free {
		p = p[:free]
	}

	cnk.buf = append(cnk.buf, p...)

	return len(p)
}

func (cnk *Chunk) done() {
	cnk.upstream = nil
	cnk.buf = cnk.buf[:0]

	cnk.pool.Put(cnk)
}
