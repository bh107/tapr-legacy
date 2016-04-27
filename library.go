package tapr

type Library struct {
	name   string
	chgr   *Changer
	drives []*Drive
}

func NewLibrary(name string, chgr *Changer) *Library {
	return &Library{
		name:   name,
		chgr:   chgr,
		drives: []*Drive{},
	}
}

func (lib *Library) String() string {
	return lib.name
}
