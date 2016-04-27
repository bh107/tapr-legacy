package main

import (
	"flag"
	"io/ioutil"
	"path"
	"path/filepath"
	"time"

	"github.com/golang/glog"
)

func main() {
	flag.Parse()

	/*
		f, err := os.Open("/ltfs/S00000L6")
		if err != nil {
			glog.Fatal(err)
		}

		chunks, err := f.Readdirnames(0)
		if err != nil {
			glog.Fatal(err)
		}

		prefix := "/ltfs/S00000L6"
		// warmup
		readChunks(prefix, chunks[15:16])

		timeit(func() { readChunks(prefix, chunks[16:191]) })

		//timeit(func() { readChunks(prefix, chunks[16:32]) })

		//timeit(func() { readChunks(prefix, chunks[32:]) })

	*/
	chunks, err := filepath.Glob("/ltfs/S00000L6/*-archive2*")
	if err != nil {
		glog.Fatal(err)
	}

	// warmup
	readChunks("", []string{"/ltfs/S00000L6/0000016-foo.cnk0000016"})
	timeit(func() { readChunks("", chunks) })

}

func timeit(fn func()) {
	start := time.Now()
	fn()
	delta := time.Since(start)

	glog.Infof("fn took %v", delta)
}

func readChunks(prefix string, chunks []string) {
	var start time.Time
	var elapsed time.Duration

	for _, cnk := range chunks {
		start = time.Now()
		buf, err := ioutil.ReadFile(path.Join(prefix, cnk))
		if err != nil {
			glog.Fatal(err)
		}
		elapsed = time.Since(start)
		glog.Infof("reading file %s took %v", cnk, elapsed)

		_ = buf
	}

	glog.Infof("read %d chunks", len(chunks))
}
