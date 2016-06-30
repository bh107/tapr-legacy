# mtx
Package mtx provides functions for working with an automated library changer.

YMMV.

## Example usage

This shows how to use the mock changer.

```go
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
```
