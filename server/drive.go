package server

import (
	"fmt"
	"log"
	"path"
	"syscall"

	"github.com/bh107/tapr/mtx"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type Drive struct {
	writer *Writer

	ctrl     chan DriveRequest
	released chan struct{}
	in       chan *Chunk

	handoff bool

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

		ctrl:     make(chan DriveRequest),
		released: make(chan struct{}),
		in:       make(chan *Chunk),

		srv: srv,
	}

	return drv
}

func (drv *Drive) String() string {
	return drv.path
}

type DriveRequest interface {
	fmt.Stringer

	Execute(*Drive) error
}

type Takeover struct {
	Cnk  *Chunk
	From *Drive
}

func (req Takeover) String() string {
	return fmt.Sprintf("takeover %v from %v", req.Cnk.upstream, req.From)
}

func (req Takeover) Execute(drv *Drive) error {
	cnk := req.Cnk

	// do not update the out channel if the stream is parallel
	if !cnk.upstream.Parallel() {
		cnk.upstream.out = drv.in
	}

	// Send the chunk that wasn't written to the new drive.
	// The drive will report back to the stream which
	// will continue on the new drive.
	go func() { drv.writer.in <- cnk }()

	return nil
}

func (drv *Drive) Ctrl(req DriveRequest) {
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

						new, err := drv.srv.GetDrive(ctx, cnk.upstream.pol)
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
								go newdrv.Ctrl(Takeover{cnk, drv})
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
			if err := req.Execute(drv); err != nil {
				log.Print(err)
			}
		}
	}
}
