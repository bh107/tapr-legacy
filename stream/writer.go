package stream

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/bh107/tapr/mtx"
)

// A writer takes chunks from a channel and writes them to a given location.
type Writer struct {
	root string
	in   chan *Chunk
	agg  chan *Chunk

	mu       sync.RWMutex
	attached int

	released chan struct{}
}

type WriteManager struct {
	root string

	unused chan *Writer
	shared chan *Writer

	agg chan *Chunk

	exclusiveWaiter chan struct{}
}

func NewWriteManager(root string, numWriters int) *WriteManager {
	manager := &WriteManager{
		root: root,

		unused: make(chan *Writer),
		shared: make(chan *Writer),

		agg: make(chan *Chunk),
	}

	return manager
}

func (m *WriteManager) NewWriter(vol *mtx.Volume) {
	wr := &Writer{
		root: path.Join(m.root, vol.Serial),

		// in channel for direct/exclusive access
		in: make(chan *Chunk),

		// aggregate channel for parallel write
		agg: m.agg,
	}

	go wr.loop()

	// add as unused
	go func() { m.unused <- wr }()
}

func (m *WriteManager) Get(timeout <-chan struct{}, exclusive bool) *Writer {
	var wr *Writer

	if exclusive {
		wr = m.GetExclusive(timeout)
	} else {
		wr = m.GetShared(timeout)
	}

	if wr == nil {
		return nil
	}

	wr.mu.Lock()
	defer wr.mu.Unlock()

	wr.attached++

	return wr
}

// GetShared returns a shared writer or nil if timeout was closed before one
// could be acquired.
func (m *WriteManager) GetShared(timeout <-chan struct{}) *Writer {
	var wr *Writer

	select {
	case <-m.exclusiveWaiter:
		// don't grab a shared channel, wait for unused so the exclusive writer
		// can get a chance.
		select {
		case <-timeout:
			return nil
		case wr = <-m.unused:
		}
	default:
		// try either
		select {
		case <-timeout:
			return nil
		case wr = <-m.unused:
		case wr = <-m.shared:
		}
	}

	// send it back to the shared channel if anyone wants one from it, or to
	// the unused, if it is released before we have a chance to send it.
	go func() {
		select {
		case <-wr.released:
			m.unused <- wr
		case m.shared <- wr:
		}
	}()

	return wr
}

// GetExclusive returns a writer or nil if timeout was closed before one could
// be acquired.
func (m *WriteManager) GetExclusive(timeout <-chan struct{}) *Writer {
	// tell the communal hippies that we want the table for our selves and
	// don't wanna starve!
	go func() { m.exclusiveWaiter <- struct{}{} }()

	select {
	case <-timeout:
		return nil
	case wr := <-m.unused:
		// wait until released, then put back as unused
		go func() { <-wr.released; m.unused <- wr }()

		return wr
	}
}

func (wr *Writer) In() chan *Chunk {
	return wr.in
}

func (m *WriteManager) Release(wr *Writer) {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	wr.attached--

	if wr.attached == 0 {
		wr.released <- struct{}{}
	}
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

		f, err := os.Create(path.Join(wr.root, fname))
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

		// reset and return chunk to stream chunk pool
		cnk.reset()
		cnk.upstream.chunkpool.Put(cnk)
	}

	// report any possible error
	cnk.upstream.errc <- err
}
