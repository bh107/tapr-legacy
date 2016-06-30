package library

import (
	"fmt"
	"syscall"

	"github.com/bh107/tapr/stream"
	"github.com/bh107/tapr/util/mtx"
	"github.com/bh107/tapr/util/proc"
	"golang.org/x/net/context"
)

type Drive struct {
	*proc.Proc

	writer *stream.Writer

	in chan *stream.Chunk

	path    string
	devtype string
	slot    int

	vol *mtx.Volume
}

func NewDrive(path string, devtype string, slot int) *Drive {
	drv := &Drive{
		path:    path,
		devtype: devtype,
		slot:    slot,

		in: make(chan *stream.Chunk),
	}

	drv.Proc = proc.Create(drv)

	return drv
}

func (drv *Drive) ProcessName() string {
	return fmt.Sprintf("[drive: %s]", drv.path)
}

func (drv *Drive) Handle(ctx context.Context, req proc.HandleFn) error {
	return req(ctx)
}

func (drv *Drive) Takeover(ctx context.Context, cnk *stream.Chunk, from *Drive) error {
	req := func(ctx context.Context) error {
		upstream := cnk.Upstream()

		if !upstream.Parallel() {
			upstream.SetOut(drv.in)
		}

		return nil
	}

	return drv.Wait(ctx, req)
}

func Write(cnk *stream.Chunk) {
	libproc.Wait(context.Background(), func(ctx context.Context) error {
		return &ErrIO{syscall.ENOSPC, cnk}
	})
}
