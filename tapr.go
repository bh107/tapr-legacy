package tapr

import (
	"container/list"
	"fmt"

	"github.com/bh107/tapr/mtx"
)

var mtxCmd = "/usr/sbin/mtx"

var DefaultPolicy = &Policy{
	Exclusive: false,
}

type Policy struct {
	Exclusive bool
}

type Archive struct {
	name   string
	chunks *list.List
}

func NewArchive(name string) *Archive {
	return &Archive{
		name:   name,
		chunks: list.New(),
	}
}

func (ar *Archive) String() string {
	return fmt.Sprintf("%s(%d chunks)", ar.name, ar.chunks.Len())
}

func (ar *Archive) Volumes() []*mtx.Volume {
	var seen map[string]bool
	for e := ar.chunks.Front(); e != nil; e = e.Next() {
		seen[e.Value.(string)] = true
	}

	var vols []*mtx.Volume
	for serial := range seen {
		vols = append(vols, &mtx.Volume{Serial: serial})
	}

	return vols
}

type Chunk struct {
	id      int
	archive *Archive
	vol     *mtx.Volume

	buf  []byte
	size int
	used int

	last bool
}

func NewChunk(size int) *Chunk {
	cnk := &Chunk{
		buf:  make([]byte, size),
		size: size,
	}

	return cnk
}

func (cnk *Chunk) reset() {
	cnk.archive = nil
	cnk.vol = nil

	cnk.used = 0
	cnk.last = false
}

func (cnk *Chunk) add(p []byte) (n int) {
	free := cnk.size - cnk.used
	if len(p) > free {
		p = p[:free]
	}

	copy(cnk.buf[cnk.used:], p)

	cnk.used = cnk.used + len(p)

	return len(p)
}
