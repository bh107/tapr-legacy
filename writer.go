package tapr

import (
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/golang/glog"
)

const DefaultChunkSize = 64 * 1 << 20

var chunkpool = &sync.Pool{
	New: func() interface{} { return NewChunk(DefaultChunkSize) },
}

type Stream struct {
	archive *Archive
	partial *Chunk
	wr      *ChunkWriter
}

func NewStream(archive string, wr *ChunkWriter) *Stream {
	ar := NewArchive(archive)

	s := &Stream{
		archive: ar,
		wr:      wr,
	}

	s.partial = s.GetChunk()

	return s
}

func (s *Stream) GetChunk() *Chunk {
	cnk := chunkpool.Get().(*Chunk)
	cnk.archive = s.archive

	s.archive.chunks.PushBack(cnk)

	return cnk
}

func (s *Stream) Close() error {
	glog.Info("closing stream")

	s.partial.last = true
	s.wr.Write(s.partial)

	return nil
}

func (s *Stream) Add(p []byte) error {
	for {
		if len(p) == 0 {
			return nil
		}

		n := s.partial.add(p)

		if n != len(p) {
			// chunk is full, send it to the chunk writer
			s.wr.Write(s.partial)

			// and grab a new chunk
			s.partial = s.GetChunk()

			p = p[n:]
			continue
		}

		break
	}

	return nil
}

type xferStats struct {
	begin      time.Time
	end        time.Time
	bytes      int
	total      int64
	totalBytes int
}

// ChunkWriter writes whole chunks to the LTFS mountpoint in one
// open-write-close operation to ensure a contiguous streaming write.
type ChunkWriter struct {
	srv *Server
	in  chan *Chunk

	ltfs *LTFS
	drv  *Drive

	xferStats map[string]*xferStats
}

// NewChunkWriter starts a new chunkwriter for the drive drv. bufferChunks is
// related to the max memory usage of the system: 2 * bufferChunks * chunksize.
func NewChunkWriter(srv *Server, drv *Drive, bufferChunks int) (*ChunkWriter, error) {
	format := false
	if drv.vol == nil {
		vol, err := inv.scratch(drv.lib.name)
		if err != nil {
			return nil, fmt.Errorf("could not acquire scratch volume: %s", err)
		}

		if err := srv.Load(drv, vol); err != nil {
			return nil, fmt.Errorf("could not load volume: %s", err)
		}

		format = true
	}

	ltfs, err := NewLTFS(drv, srv.ltfsRoot, LTFSSyncModeUnmount, format)
	if err != nil {
		return nil, fmt.Errorf("could not mount LTFS filesystem: %s", err)
	}

	w := &ChunkWriter{
		srv:       srv,
		in:        make(chan *Chunk, bufferChunks),
		ltfs:      ltfs,
		drv:       drv,
		xferStats: make(map[string]*xferStats),
	}

	drv.wr = w

	go runChunkWriter(w)

	return w, nil
}

func (w *ChunkWriter) Write(c *Chunk) {
	w.in <- c
}

func runChunkWriter(wr *ChunkWriter) {
	var begin time.Time
	var lastSeen bool
	var globalChunkId int
	first := true
	var globalBegin time.Time

	for chunk := range wr.in {
		if first && lastSeen {
			globalBegin = time.Now()
			first = false
		}
		// get a chunk id
		var chunkid int
		err := chunkdb.Update(func(tx *bolt.Tx) error {
			bkt := tx.Bucket([]byte(chunk.archive.name))

			id, err := bkt.NextSequence()
			if err != nil {
				return err
			}

			chunkid = int(id)

			return bkt.Put(itob(chunkid), []byte(wr.drv.vol.Serial))
		})

		if chunkid == 1 {
			// first chunk, lets time it
			wr.xferStats[chunk.archive.name] = &xferStats{
				begin: time.Now(),
			}
		}

		globalChunkId += 1

		xferStats := wr.xferStats[chunk.archive.name]

		xferStats.bytes = xferStats.bytes + chunk.used

		begin = time.Now()
		f, err := os.Create(path.Join(wr.ltfs.mountpoint, fmt.Sprintf("%07d-%s.cnk%07d", globalChunkId, chunk.archive.name, chunkid)))
		if err != nil {
			glog.Fatal(err)
		}

		if _, err := f.Write(chunk.buf); err != nil {
			// XXX unmount and mount new scratch volume?
			glog.Fatal(err)
		}

		if err := f.Close(); err != nil {
			glog.Fatal(err)
		}
		delta := time.Now().Sub(begin)

		// allow warmup of tape
		if lastSeen {
			wr.srv.xferStats.total += delta.Nanoseconds()
			wr.srv.xferStats.totalBytes += chunk.used
		}

		if chunk.last {
			lastSeen = true
			xferStats.end = time.Now()

			delta := xferStats.end.Sub(xferStats.begin)
			glog.Infof("%s (%d bytes) written in %v (%d B/s)",
				chunk.archive.name, xferStats.bytes, delta, (xferStats.bytes / int(delta.Seconds())),
			)
			glog.Infof("global duration: %v", xferStats.end.Sub(globalBegin))
		}

		// reset chunk and return to pool
		chunk.reset()
		chunkpool.Put(chunk)
	}
}
