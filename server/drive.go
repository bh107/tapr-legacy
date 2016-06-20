package server

import (
	"fmt"
	"log"
	"path"
	"syscall"

	"github.com/bh107/tapr/mtx"
	"github.com/bh107/tapr/stream"
	"github.com/bh107/tapr/stream/policy"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type Request interface {
	fmt.Stringer

	// Code running in the Execute function will run in the main process, thus
	// it is safe to modify the process state.
	Execute(context.Context)
	Context() context.Context
}

type Drive struct {
	writer *stream.Writer

	ctrl     chan Request
	in       chan *stream.Chunk
	reserved chan struct{}
	waiting  chan struct{}

	handoff     bool
	attached    int
	shared      bool
	maxAttached int

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
		in:       make(chan *stream.Chunk),
		reserved: make(chan struct{}),
		waiting:  make(chan struct{}),

		srv: srv,

		shared: true,

		maxAttached: 4,
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
	drv := ctx.Value(DriveContextKey).(*Drive)

	drv.attached--

	if drv.attached == 0 {
		select {
		case <-drv.reserved:
			// someone was waiting for exclusive access
			drv.shared = false
			drv.attached++
		default:
		}

		return
	}

	// see if anyone is waiting in queue for shared access (read on the channel
	// to allow one process to proceed).
	select {
	case <-drv.waiting:
		drv.attached++
	default:
	}

	// just return as shared
	drv.shared = true
}

func (drv *Drive) Release() {
	ctx := context.WithValue(context.Background(), DriveContextKey, drv)
	drv.ctrl <- &ReleaseRequest{ctx}
}

type UseRequest struct {
	ctx context.Context
	ok  chan error
	pol *policy.Policy
}

func (req UseRequest) String() string {
	return "use"
}

func (req UseRequest) Context() context.Context {
	return req.ctx
}

func (req UseRequest) Execute(ctx context.Context) {
	drv := ctx.Value(DriveContextKey).(*Drive)

	pol := req.pol

	if pol.Exclusive {
		if drv.attached == 0 {
			select {
			case <-ctx.Done():
			case req.ok <- nil:
				drv.shared = false
				drv.attached++
			}

			return
		}

		go func() {
			select {
			case <-ctx.Done():
				// request cancelled or timed out
				return
			case drv.reserved <- struct{}{}:
				// woop, we got the drive
				select {
				case <-ctx.Done():
					// no one wanted it anyway, release it again
					drv.Release()
				case req.ok <- nil:
				}
			}
		}()

		return
	}

	if drv.shared && drv.attached < drv.maxAttached {
		select {
		case <-ctx.Done():
		case req.ok <- nil:
			drv.attached++
		}

		return
	}

	// wait for the drive to become available asynchronously
	go func() {
		select {
		case <-ctx.Done():
			// request cancelled or timed out
			return
		case drv.waiting <- struct{}{}:
			// woop, we got the drive
			select {
			case <-ctx.Done():
				// no one wanted it anyway, release it
				drv.Release()
			case req.ok <- nil:
			}
		}
	}()
}

type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "server context value " + k.name }

var DriveContextKey = &contextKey{"drive"}

func (drv *Drive) Use(ctx context.Context, pol *policy.Policy) error {
	ctx = context.WithValue(ctx, DriveContextKey, drv)

	req := &UseRequest{
		ctx: ctx,
		pol: pol,
		ok:  make(chan error),
	}

	drv.Ctrl(req)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-req.ok:
		return err
	}
}

type Takeover struct {
	cnk  *stream.Chunk
	from *Drive
	to   *Drive
}

func (req Takeover) String() string {
	return fmt.Sprintf("takeover %v from %v", req.cnk.Upstream(), req.from)
}

func (req Takeover) Context() context.Context {
	return context.TODO()
}

func (req Takeover) Execute(ctx context.Context) {
	cnk := req.cnk
	upstream := cnk.Upstream()

	// do not update the out channel if the stream is parallel
	if !upstream.Parallel() {
		upstream.SetOut(req.to.in)
		upstream.OnClose(func() {
			req.to.Release()
		})
	}

	// Send the chunk that wasn't written to the new drive.
	// The drive will report back to the stream which
	// will continue on the new drive.
	go func() { req.to.in <- cnk }()
}

func (drv *Drive) Takeover(cnk *stream.Chunk, from *Drive) {
	drv.Ctrl(&Takeover{cnk, from, drv})
}

func (drv *Drive) Ctrl(req Request) {
	drv.ctrl <- req
}

func (drv *Drive) Agg() chan *stream.Chunk {
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
		case err := <-drv.writer.Errc():
			if errIO, ok := err.(stream.ErrIO); ok {
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

						new, err := acquireDrive(ctx, drv.srv.drives["write"], cnk.Upstream().Policy())
						if err != nil {
							log.Print(err)
							return
						}

						reqDrive <- new
					}()

					// No context needed, should not be cancelled in any case.
					reqWriter := make(chan *stream.Writer)
					go func() {
						_, err := drv.srv.GetScratch(drv)
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

						reqWriter <- stream.NewWriter(mountpoint, drv.in, drv.Agg(), drv.path)
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
					cnk.Upstream().Errc() <- errIO.Err
				}
			} else {
				log.Printf("%v: ERROR: %s", drv, err)
			}

		case req := <-drv.ctrl:
			log.Printf("%v control request: %v", drv, req)
			req.Execute(req.Context())
		}
	}
}
