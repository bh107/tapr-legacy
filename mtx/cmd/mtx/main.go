package main

import (
	"fmt"
	"log"

	"github.com/kbj/mtx"
	"github.com/kbj/mtx/mock"
)

func main() {
	mtx := mtx.NewChanger(mock.New(8, 32, 4, 16))

	status, err := mtx.Do("status")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s", status)

	if err := mtx.Load(1, 0); err != nil {
		log.Fatal(err)
	}
}
