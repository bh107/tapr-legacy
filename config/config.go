package config

import (
	"bytes"
	"errors"
	"io"

	"github.com/hashicorp/hcl"
)

type Config struct {
	Debug      DebugConfig     `hcl:"debug"`
	Chunkstore DBConfig        `hcl:"chunkstore"`
	Inventory  DBConfig        `hcl:"inventory"`
	LTFS       LTFSConfig      `hcl:"ltfs"`
	Libraries  []LibraryConfig `hcl:"library"`
}

type DebugConfig struct {
	Mocking MockingConfig `hcl:"mocking"`
}

type MockingConfig struct {
	ChunkSize int           `hcl:"chunksize"`
	Timings   TimingsConfig `hcl:"timings"`
}

type TimingsConfig struct {
	Unmount string `hcl:"unmount"`
	Unload  string `hcl:"unload"`
	Load    string `hcl:"load"`
	Mount   string `hcl:"mount"`
	Format  string `hcl:"format"`
}

type DBConfig struct {
	Type string `hcl:"type"`
	Path string `hcl:"path"`
}

type LTFSConfig struct {
	Root string `hcl:"root"`
}

type DriveConfig struct {
	Path  string `hcl:",key"`
	Type  string `hcl:"type"`
	Slot  int    `hcl:"slot"`
	Group string `hcl:"group"`
}

type ChangerConfig struct {
	Path string `hcl:",key"`
	Type string `hcl:"type"`
}

type LibraryConfig struct {
	Name     string          `hcl:",key"`
	Changers []ChangerConfig `hcl:"changer"`
	Drives   []DriveConfig   `hcl:"drive"`
}

func Parse(r io.Reader) (*Config, error) {
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

	if result.Chunkstore.Type != "boltdb" {
		return nil, errors.New("unknown chunkstore database type")
	}

	if result.Inventory.Type != "sqlite3" {
		return nil, errors.New("unknown inventory database type")
	}

	return result, nil
}
