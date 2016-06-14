package config

import (
	"bytes"
	"reflect"
	"testing"
)

var testConfig = `
chunkstore {
        type = "boltdb"
}

inventory {
        type = "sqlite3"
}

ltfs {
        root = "/ltfs"
}

library "primary" {
        changer "/dev/sg4" {
                type = "mtx"
        }

        drive "/dev/st0" {
                type = "write"
                slot = 1
				group = "parallel-write"
        }

        drive "/dev/st1" {
                type = "read"
                slot = 0
        }

}

library "secondary" {
        changer "/dev/sg7" {
                type = "mtx"
        }

        drive "/dev/st2" {
                type = "write"
                slot = 1
				group = "parallel-write"
        }

        drive "/dev/st3" {
                type = "read"
                slot = 0
        }
}
`

func TestConfigParsing(t *testing.T) {
	expected := &Config{
		Chunkstore: DBConfig{Type: "boltdb"},
		Inventory:  DBConfig{Type: "sqlite3"},
		LTFS: LTFSConfig{
			Root: "/ltfs",
		},
		Libraries: []LibraryConfig{
			LibraryConfig{
				Name: "primary",
				Changers: []ChangerConfig{
					ChangerConfig{Path: "/dev/sg4", Type: "mtx"},
				},
				Drives: []DriveConfig{
					DriveConfig{
						Path: "/dev/st0", Type: "write",
						Slot: 1, Group: "parallel-write",
					},
					DriveConfig{
						Path: "/dev/st1", Type: "read", Slot: 0},
				},
			},
			LibraryConfig{
				Name: "secondary",
				Changers: []ChangerConfig{
					ChangerConfig{Path: "/dev/sg7", Type: "mtx"},
				},
				Drives: []DriveConfig{
					DriveConfig{
						Path: "/dev/st2", Type: "write",
						Slot: 1, Group: "parallel-write",
					},
					DriveConfig{Path: "/dev/st3", Type: "read", Slot: 0},
				},
			},
		},
	}

	buf := bytes.NewBufferString(testConfig)

	config, err := Parse(buf)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(config, expected) {
		t.Error("parsed not equal to expected")
	}
}
