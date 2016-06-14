package server

import (
	"errors"
	"log"
	"path"
	"syscall"

	"github.com/bh107/tapr/mtx"
	"github.com/bh107/tapr/stream/policy"
	"golang.org/x/net/context"
)

type DriveRequest func()

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
	group   string

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

func (drv *Drive) Attach(s *Stream) {
	rep := make(chan struct{})

	drv.ctrl <- func() {
		s.out = drv.in
		s.drv = drv

		close(rep)
	}

	<-rep
	log.Printf("drive: stream attached to %v", drv)
}

func (drv *Drive) Takeover(cnk *Chunk) {
	drv.ctrl <- func() {
		log.Printf("drive %v: taking over stream", drv)

		cnk.upstream.out = drv.in
		cnk.upstream.drv = drv

		// Send the chunk that wasn't written to the new drive.
		// The drive will report back to the stream which
		// will continue on the new drive.
		go func() { drv.writer.in <- cnk }()
	}
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
			// if err was nil, it was a normal exit, mount a new volume and
			// mark as filled.
			if errIO, ok := err.(ErrIO); ok {
				cnk := errIO.Chunk

				if errIO.Err == syscall.ENOSPC {
					// The volume is full

					// Start two asynchronous operations. Try to offload the
					// stream to another drive (either exclusive or shared, it
					// doesn't matter, Get will do the right thing). Also start
					// a mount of a new volume.

					// Get a cancellable context
					ctx, cancelGet := context.WithCancel(context.Background())

					// Start the request for another drive
					getNewDrive := make(chan *Drive)
					go func(pol *policy.Policy) {
						defer cancelGet()

						newdrv, err := drv.srv.Get(ctx, pol)
						if err != nil {
							log.Print(err)
							return
						}

						getNewDrive <- newdrv
					}(cnk.upstream.pol)

					// No context needed, should not be cancelled in any case.
					getNewWriter := make(chan *Writer)
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

						getNewWriter <- NewWriter(mountpoint, vol, drv.in, nil)
					}()

					var handedoff bool
					for {
						select {
						case newdrv := <-getNewDrive:
							if newdrv != nil {
								// hand off stream
								go newdrv.Takeover(cnk)
								handedoff = true
							}

							// wait for mount
							continue
						case newwr := <-getNewWriter:
							// cancel the GetDrive request
							cancelGet()

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
			req()
		}
	}
}
