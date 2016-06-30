package server

// Specification describes a server.
type Specification struct {
	// Debug is true if debugging has been enabled.
	Debug bool

	// InitialAudit is true if the server should perform an initial audit of
	// the connected libraries.
	InitialAudit bool

	// Mocking is true if mocking of libraries, drives and volumes as been
	// enabled.
	Mocking bool

	// Libraries is a list of library specifications
	//Libraries []library.Specification
}

func MakeSpecification() Specification {
	return Specification{
		Debug:        false,
		InitialAudit: false,
		Mocking:      false,
	}
}

func (spec *Specification) InitLibraries() error {
	return nil
}

func NewServer(spec *Specification) *Server {
	return nil
}
