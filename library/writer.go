package library

import (
	"github.com/bh107/tapr/stream"
	"github.com/bh107/tapr/util/proc"
	"golang.org/x/net/context"
)

type Writer struct {
	*proc.Proc

	in chan *stream.Chunk
}

func NewWriter() *Writer {
	wr := &Writer{}

	wr.Proc = proc.Create(wr)

	return wr
}

func (wr *Writer) Handle(ctx context.Context, req proc.HandleFn) error {
	return req(ctx)
}

func (wr *Writer) ProcessName() string {
	return "writer"
}
