package main

import (
	"fmt"
	_ "net/http/pprof"
	"os"

	"github.com/bh107/tapr/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
