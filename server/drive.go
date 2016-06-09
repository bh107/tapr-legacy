package server

import (
	"errors"
	"log"
	"path"
	"syscall"

	"github.com/bh107/tapr/mtx"
	"github.com/bh107/tapr/stream"
	"golang.org/x/net/context"
)

type DriveRequest func(*Drive) error

type Drive struct {
	// buffer chunk
	chunk *stream.Chunk

	writer *stream.Writer

	ctrl     chan DriveRequest
	released chan struct{}

	attached int

	path    string
	devtype string
	slot    int
	lib     *Library

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

		srv: srv,
	}

	// add as unused
	go func() { srv.drives[devtype].unused <- drv }()

	return drv
}

func (drv *Drive) String() string {
	return drv.path
}

func (drv *Drive) Mountpoint() (string, error) {
	if drv.vol == nil {
		return "", errors.New("no volume")
	}

	return path.Join(drv.srv.cfg.LTFS.Root, drv.devtype, drv.vol.Serial), nil
}

func (drv *Drive) run() {
	for {
		select {
		case err := <-drv.writer.Errc():
			// if err was nil, it was a normal exit, mount a new volume and
			// mark as filled.
			if errIO, ok := err.(stream.ErrIO); ok {
				cnk := errIO.Chunk

				if errIO.Err == syscall.ENOSPC {
					// The volume is full.

					// Start two asynchronous operations. Try to offload the
					// stream to another drive (either exclusive or shared, it
					// doesn't matter, Get will do the right thing). Also start
					// a mount of a new volume.

					// Get a cancellable context
					ctx, cancelGet := context.WithCancel(context.Background())

					// Start the request for another drive
					getNewDrive := make(chan *Drive)
					go func() {
						newdrv, err := drv.srv.Get(ctx, cnk.Upstream().Policy())
						if err != nil {
							log.Print(err)
							getNewDrive <- nil
							return
						}

						cancelGet()
						getNewDrive <- newdrv
					}()

					// No context needed, should not be cancelled in any case.
					getNewWriter := make(chan *stream.Writer)
					go func() {
						vol, err := drv.srv.GetScratch(drv)
						if err != nil {
							log.Print(err)
							getNewWriter <- nil
							return
						}

						mountpoint, err := drv.Mountpoint()
						if err != nil {
							log.Print(err)
							getNewWriter <- nil
							return
						}

						getNewWriter <- stream.NewWriter(mountpoint, vol, nil)
					}()

					for {
						select {
						case newdrv := <-getNewDrive:
							if newdrv != nil {
								// update the stream
								in, _ := newdrv.writer.Ingress()
								cnk.Upstream().SetOut(in)

								// Send the chunk that wasn't written to the new drive.
								// The drive will report back to the stream which
								// will continue on the new drive.
								in <- cnk
							}

							// wait for mount
							continue
						case newwr := <-getNewWriter:
							// cancel the GetDrive request and break the loop.
							cancelGet()

							_ = newwr

							break
						}
					}
				} else {
					// XXX other error, report to stream. Mark volume as
					// suspicious and mount new one.
					cnk.Upstream().Errc() <- errIO.Err
				}
			}

		case req := <-drv.ctrl:
			if err := req(drv); err != nil {
				// request failed.
			}
		}
	}
}
