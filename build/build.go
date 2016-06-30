package build

import (
	"fmt"
	"runtime"
)

var (
	tag      = "undefined"
	time     string
	platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

type Info struct {
	GoVersion string
	Tag       string
	Time      string
	Platform  string
}

func (b Info) Short() string {
	return fmt.Sprintf("taprd %s (%s, built %s, %s)", b.Tag, b.Platform, b.Time, b.GoVersion)
}

func GetInfo() Info {
	return Info{
		GoVersion: runtime.Version(),
		Tag:       tag,
		Time:      time,
		Platform:  platform,
	}
}
