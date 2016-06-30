package main

import (
	"fmt"
	"log"

	"github.com/bh107/tapr/mtx"
	"github.com/bh107/tapr/mtx/mock"
)

func main() {
	mtx := mtx.NewChanger(mock.New("/dev/mock"))

	status, err := mtx.Do("status")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s", status)

	if err := mtx.Load(1, 0); err != nil {
		log.Fatal(err)
	}
}
