package tapr

import (
	"io/ioutil"
	"os"

	"github.com/naoina/toml"
)

type config struct {
	Libraries map[string]libraryConf

	Invdb   string
	Chunkdb string

	Mountroot string
}

type libraryConf struct {
	Changer string
	Drives  []string
}

func loadConfig(path string) (*config, error) {
	// load config
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var config config
	if err := toml.Unmarshal(buf, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
