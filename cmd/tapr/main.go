package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/bh107/tapr"
	"github.com/bh107/tapr/api"
	"github.com/golang/glog"
)

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
