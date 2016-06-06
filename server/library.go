package server

import "github.com/bh107/tapr/changer"

type Library struct {
	name   string
	chgr   *changer.Changer
	drives map[string][]*Drive
}

func NewLibrary(name string) *Library {
	return &Library{
		name:   name,
		drives: make(map[string][]*Drive),
	}
}

func (lib *Library) String() string {
	return lib.name
}
