package tapr

import "github.com/kbj/mtx"

type Drive struct {
	path    string
	devtype string
	slot    int
	lib     *Library

	vol *mtx.Volume

	wr *ChunkWriter
}

func NewDrive(path string, devtype string, slot int, lib *Library) *Drive {
	return &Drive{
		path:    path,
		devtype: devtype,
		slot:    slot,
		lib:     lib,
	}
}

func (dev *Drive) String() string {
	return dev.path
}
