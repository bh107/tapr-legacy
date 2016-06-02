package server

import "github.com/bh107/tapr/changer"

type Library struct {
	name   string
	chgr   *changer.Changer
	drives []*Drive
}

func NewLibrary(name string, chgr *changer.Changer) *Library {
	return &Library{
		name:   name,
		chgr:   chgr,
		drives: []*Drive{},
	}
}

func (lib *Library) String() string {
	return lib.name
}
