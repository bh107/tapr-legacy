// Package proc generalizes the loop of a process.
package proc

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"golang.org/x/net/context"
)

var tracked struct {
	sync.Mutex
	procs []*Proc
}

func handleDebug(w http.ResponseWriter, r *http.Request) {
	tracked.Lock()
	defer tracked.Unlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	for _, proc := range tracked.procs {
		fmt.Fprintf(w, "%p: %v\n", proc, proc)
	}
}

func init() {
	http.Handle("/debug/procs", http.HandlerFunc(handleDebug))
}

type HandleFn func(context.Context) error

// Runner is an interface that runnable processes must implement.
type Process interface {
	ProcessName() string
	Handle(context.Context, HandleFn) error
}

type request struct {
	ctx  context.Context
	fn   HandleFn
	errc chan error
}

type Proc struct {
	proc Process
	ch   chan request
}

func Create(proc Process) *Proc {
	tracked.Lock()
	defer tracked.Unlock()

	p := &Proc{
		proc: proc,
		ch:   make(chan request),
	}

	tracked.procs = append(tracked.procs, p)

	go p.run(proc)

	return p
}

// Wait sends a request to the process and waits for it to complete or for the
// context to be cancelled.
func (p *Proc) Wait(ctx context.Context, req HandleFn) error {
	errc := make(chan error)

	// send the request to the process
	p.ch <- request{ctx, req, errc}

	// wait for response or context cancel
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errc:
		return err
	}
}

// Post sends a request to the process and returns immediately
func (p *Proc) Post(req HandleFn) {
	p.ch <- request{context.Background(), req, nil}
}

// run loops and calls the process handler for each message.
func (p *Proc) run(proc Process) {
	for req := range p.ch {
		var err error
		if err = proc.Handle(req.ctx, req.fn); err != nil {
			log.Print(err)
		}

		// report error to waiter
		if req.errc != nil {
			req.errc <- err
		}
	}
}
