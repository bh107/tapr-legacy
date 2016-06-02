package tapr

import (
	"bytes"
	"io"

	"github.com/hashicorp/hcl"
)

type LTFSConfig struct {
	Root string `hcl:"root"`
}

type DrivesConfig struct {
	DriveType string   `hcl:",key"`
	Devices   []string `hcl:"devices"`
}

type LibraryConfig struct {
	Name    string         `hcl:",key"`
	Changer string         `hcl:"changer"`
	Drives  []DrivesConfig `hcl:"drives"`
}

type Config struct {
	Mock      bool            `hcl:"mock"`
	LTFS      LTFSConfig      `hcl:"ltfs"`
	Libraries []LibraryConfig `hcl:"library"`
}

func ParseConfig(r io.Reader) (*Config, error) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}

	hclTree, err := hcl.Parse(buf.String())
	if err != nil {
		return nil, err
	}

	result := new(Config)
	if err := hcl.DecodeObject(&result, hclTree); err != nil {
		return nil, err
	}

	return result, nil
}
