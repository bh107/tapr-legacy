package server

import (
	"container/list"
	"fmt"

	"github.com/bh107/tapr/mtx"
)

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
