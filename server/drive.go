package server

import (
	"fmt"
	"log"
	"path"
	"syscall"

	"github.com/bh107/tapr/mtx"
	"github.com/bh107/tapr/stream/policy"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type Request interface {
	fmt.Stringer

	Execute(context.Context)
	Context() context.Context
}

type Drive struct {
	writer *Writer

	ctrl     chan Request
	in       chan *Chunk
	reserved chan struct{}
	waiting  chan struct{}

	handoff  bool
	attached int
	shared   bool

	path    string
	devtype string
	slot    int
	lib     *Library
	group   *driveGroup

	vol *mtx.Volume

	srv *Server
}

func (srv *Server) NewDrive(path string, devtype string, slot int, lib *Library) *Drive {
	drv := &Drive{
		path:    path,
		devtype: devtype,
		slot:    slot,
		lib:     lib,

		ctrl:     make(chan Request),
		in:       make(chan *Chunk),
		reserved: make(chan struct{}),
		waiting:  make(chan struct{}),

		srv: srv,

		shared: true,
	}

	return drv
}

func (drv *Drive) String() string {
	return drv.path
}

type ReleaseRequest struct {
	ctx context.Context
}

func (req ReleaseRequest) String() string {
	return "release"
}

func (req ReleaseRequest) Context() context.Context {
	return req.ctx
}

func (req ReleaseRequest) Execute(ctx context.Context) {
	drv := ctx.Value(driveKey).(*Drive)

	drv.attached--

	select {
	case <-drv.reserved:
		// someone was waiting for the drive
		drv.shared = false
		drv.attached++
	default:
		select {
		case <-drv.waiting:
			drv.attached++
		default:
		}

		// just return as shared
		drv.shared = true
	}

	log.Print(drv.attached)
}

func (drv *Drive) Release(ctx context.Context) {
	log.Printf("release called for %v", drv)
	ctx = context.WithValue(ctx, driveKey, drv)
	drv.ctrl <- &ReleaseRequest{ctx}
}

type UseRequest struct {
	ctx context.Context
	ok  chan struct{}
	pol *policy.Policy
}

func (req UseRequest) String() string {
	return "use"
}

func (req UseRequest) Context() context.Context {
	return req.ctx
}

func (req UseRequest) Execute(ctx context.Context) {
	drv := ctx.Value(driveKey).(*Drive)

	pol := req.pol

	if pol.Exclusive {
		if drv.attached == 0 {
			drv.shared = false
			drv.attached++

			close(req.ok)

			return
		}

		go func() {
			select {
			case <-ctx.Done():
				// request cancelled or timed out
				return
			case drv.reserved <- struct{}{}:
				// woop, we got the drive
				close(req.ok)
			}
		}()
	} else {
		if drv.shared {
			drv.attached++

			close(req.ok)

			return
		}
	}

	// reserve the drive for exclusive access when possible
	go func() {
		select {
		case <-ctx.Done():
			// request cancelled or timed out
			return
		case drv.waiting <- struct{}{}:
			// woop, we got the drive
			close(req.ok)
		}
	}()
}

type key int

const driveKey key = 0

func (drv *Drive) Use(ctx context.Context, pol *policy.Policy) error {
	ctx = context.WithValue(ctx, driveKey, drv)

	req := &UseRequest{
		ctx: ctx,
		pol: pol,
		ok:  make(chan struct{}),
	}

	drv.Ctrl(req)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-req.ok:
	}

	return nil
}

type Takeover struct {
	cnk  *Chunk
	from *Drive
	to   *Drive
}

func (req Takeover) String() string {
	return fmt.Sprintf("takeover %v from %v", req.cnk.upstream, req.from)
}

func (req Takeover) Context() context.Context {
	return context.TODO()
}

func (req Takeover) Execute(ctx context.Context) {
	cnk := req.cnk

	// do not update the out channel if the stream is parallel
	if !cnk.upstream.Parallel() {
		cnk.upstream.out = req.to.in
		cnk.upstream.onclose = func() { req.to.Release(context.Background()) }
	}

	// Send the chunk that wasn't written to the new drive.
	// The drive will report back to the stream which
	// will continue on the new drive.
	go func() { req.to.in <- cnk }()
}

func (drv *Drive) Takeover(cnk *Chunk, from *Drive) {
	drv.Ctrl(&Takeover{cnk, from, drv})
}

func (drv *Drive) Ctrl(req Request) {
	drv.ctrl <- req
}

func (drv *Drive) Agg() chan *Chunk {
	if drv.group != nil {
		return drv.group.in
	}

	return nil
}

func (drv *Drive) Mountpoint() (string, error) {
	if drv.vol == nil {
		return "", errors.New("no volume")
	}

	return path.Join(drv.srv.cfg.LTFS.Root, drv.devtype, drv.vol.Serial), nil
}

func (drv *Drive) Run() {
	for {
		select {
		case err := <-drv.writer.errc:
			if errIO, ok := err.(ErrIO); ok {
				cnk := errIO.Chunk

				if errIO.Err == syscall.ENOSPC {
					// The volume is full

					// Start two asynchronous operations. Try to offload the
					// stream to another drive (either exclusive or shared, it
					// doesn't matter, Get will do the right thing). Also start
					// a mount of a new volume.

					// Get a cancellable context
					ctx, cancel := context.WithCancel(context.Background())

					// Start the request for another drive
					reqDrive := make(chan *Drive)
					go func() {
						defer cancel()

						new, err := AcquireDrive(ctx, drv.srv.drives["write"], cnk.upstream.pol)
						if err != nil {
							log.Print(err)
							return
						}

						reqDrive <- new
					}()

					// No context needed, should not be cancelled in any case.
					reqWriter := make(chan *Writer)
					go func() {
						vol, err := drv.srv.GetScratch(drv)
						if err != nil {
							log.Print(err)
							reqWriter <- nil
							return
						}

						mountpoint, err := drv.Mountpoint()
						if err != nil {
							log.Print(err)
							reqWriter <- nil
							return
						}

						reqWriter <- NewWriter(mountpoint, vol, drv.in, drv.Agg(), drv)
					}()

					var handedoff bool
					for {
						select {
						case newdrv := <-reqDrive:
							if newdrv != nil {
								// hand off stream
								drv.attached--
								go newdrv.Takeover(cnk, drv)
								handedoff = true
							}

							// wait for mount
							continue
						case newwr := <-reqWriter:
							// cancel the GetDrive request
							cancel()

							// update our writer
							drv.writer = newwr

							// if this chunk's stream wasn't offloaded to
							// another drive, send the chunk to the new writer.
							if !handedoff {
								go func() { drv.in <- cnk }()
							}
						}

						break
					}
				} else {
					// XXX other error, report to stream. Mark volume as
					// suspicious and mount new one.
					cnk.upstream.errc <- errIO.Err
				}
			} else {
				log.Printf("%v: ERROR: %s", drv, err)
			}

		case req := <-drv.ctrl:
			log.Printf("%v ctrl request: %v", drv, req)
			req.Execute(req.Context())
		}
	}
}
