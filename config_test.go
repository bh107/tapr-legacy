package tapr

import (
	"reflect"
	"testing"
)

var testConfig = `
mock = true

ltfs {
        root = "/ltfs"
}

library "primary" {
        changer = "/dev/sg4"

        drives "read" {
                devices  = ["/dev/st0"]
        }

        drives "write" {
                devices = ["/dev/st1"]
        }
}

library "secondary" {
        changer = "/dev/sg7"

        drives "read" {
                devices = ["/dev/st2"]
        }

        drives "write" {
                devices  = ["/dev/st3"]
        }
}
`

func TestConfigParsing(t *testing.T) {
	expected := &Config{
		Mock: true,
		LTFS: LTFSConfig{
			Root: "/ltfs",
		},
		Libraries: []LibraryConfig{
			LibraryConfig{
				Name:    "primary",
				Changer: "/dev/sg4",
				Drives: []DrivesConfig{
					DrivesConfig{
						DriveType: "read",
						Devices:   []string{"/dev/st0"},
					},
					DrivesConfig{
						DriveType: "write",
						Devices:   []string{"/dev/st1"},
					},
				},
			},
			LibraryConfig{
				Name:    "secondary",
				Changer: "/dev/sg7",
				Drives: []DrivesConfig{
					DrivesConfig{
						DriveType: "read",
						Devices:   []string{"/dev/st2"},
					},
					DrivesConfig{
						DriveType: "write",
						Devices:   []string{"/dev/st3"},
					},
				},
			},
		},
	}

	config, err := ParseConfig(testConfig)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(config, expected) {
		t.Error("parsed not equal to expected")
	}
}
