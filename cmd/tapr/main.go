package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/bh107/tapr"
	"github.com/bh107/tapr/api"
	"github.com/golang/glog"
)

/*
type api struct {
	lib *tapr.Library
}

func (api api) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// PUT request
	// 	parse request as /archive/<id>/
	// 	allocate scratch volume or use existing volume with free space
	//	mount scratch volume in /ltfs/<serial>/ (if not mounted already)
	//  chunk incoming stream into <id>XXXX where XXXX is a running number
	//   record each chunk in chunkdb as key:<id>XXXX value:<serial>
	//   write chunk as <id>XXXX in /ltfs/<serial>/

	// GET request
	//  parse request as /archive/<id>/
	//  lookup chunks in chunkdb to get list of volume serials (sorted in chunk order)
	//  for each volume/serial
	//   mount volume in /ltfs/<serial>/
	//   read chunks and transfer to client
	//   unmount volume

	log.Println(req.URL.Path)
}
*/

func main() {
	flag.Parse()

	config := "./config.toml"

	tapr, err := tapr.New(config)
	if err != nil {
		glog.Fatal(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c

		tapr.Shutdown()
		os.Exit(0)
	}()

	// start the API
	glog.Fatal(api.Start(tapr))
}
