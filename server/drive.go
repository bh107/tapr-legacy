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
	// buffer chunk
	//chunk *Chunk

	writer *Writer

	ctrl      chan DriveRequest
	released  chan struct{}
	in        chan *Chunk
	retracted chan bool

	attached int
	handoff  bool

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

		ctrl:      make(chan DriveRequest),
		released:  make(chan struct{}),
		in:        make(chan *Chunk),
		retracted: make(chan bool),

		srv: srv,
	}

	return drv
}

func (drv *Drive) String() string {
	return drv.path
}

func (drv *Drive) Attach(s *Stream) {
	rep := make(chan error)

	drv.ctrl <- func() {
		drv.attached++

		s.out = drv.in
		s.drv = drv

		rep <- nil
	}

	<-rep
	log.Printf("drive: stream attached to %v", drv)
}

func (drv *Drive) Takeover(cnk *Chunk) {
	drv.ctrl <- func() {
		log.Printf("drive %v: taking over stream", drv)
		drv.attached++

		cnk.upstream.out = drv.in
		cnk.upstream.drv = drv

		// Send the chunk that wasn't written to the new drive.
		// The drive will report back to the stream which
		// will continue on the new drive.
		drv.writer.in <- cnk
	}
}

func (drv *Drive) Mountpoint() (string, error) {
	if drv.vol == nil {
		return "", errors.New("no volume")
	}

	return path.Join(drv.srv.cfg.LTFS.Root, drv.devtype, drv.vol.Serial), nil
}

// Release returns the drive as unused if after release, no streams are
// attached.
func (drv *Drive) Release() {
	drv.ctrl <- func() {
		drv.attached--

		if drv.attached == 0 {
			// release it (go back to unused)
			go func() { drv.released <- struct{}{}; log.Printf("%v: RELEASED", drv) }()
		}
	}
}

func (drv *Drive) Run() {
	for {
		log.Printf("drive: %v selecting", drv)
		select {
		case err := <-drv.writer.Errc():
			// if err was nil, it was a normal exit, mount a new volume and
			// mark as filled.
			if errIO, ok := err.(ErrIO); ok {
				cnk := errIO.Chunk

				if errIO.Err == syscall.ENOSPC {
					// The volume is full.
					go func() { drv.retracted <- true }()

					// Start two asynchronous operations. Try to offload the
					// stream to another drive (either exclusive or shared, it
					// doesn't matter, Get will do the right thing). Also start
					// a mount of a new volume.

					// Get a cancellable context
					ctx, cancelGet := context.WithCancel(context.Background())

					// Start the request for another drive
					getNewDrive := make(chan *Drive)
					go func(pol *policy.Policy) {
						newdrv, err := drv.srv.Get(ctx, pol)
						if err != nil {
							log.Print(err)
							return
						}

						cancelGet()
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

					for {
						select {
						case newdrv := <-getNewDrive:
							if newdrv != nil {
								drv.handoff = true
								// stream no longer attached to this drive
								drv.attached--
								newdrv.Takeover(cnk)
							}

							// wait for mount
							continue
						case newwr := <-getNewWriter:
							// cancel the GetDrive request and break the loop.
							cancelGet()

							// update our writer
							drv.writer = newwr

							if drv.handoff {
								drv.handoff = false
							} else {
								drv.writer.in <- cnk
							}

							// if there isn't anything else attached here, release it
							if drv.attached == 0 {
								log.Printf("%v: releasing because there are noone attached", drv)
								go func(drv *Drive) {
									drv.released <- struct{}{}
								}(drv)
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
			log.Printf("%v: got request", drv)
			req()
		}
	}
}
