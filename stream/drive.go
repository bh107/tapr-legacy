package stream

import (
	"syscall"

	"golang.org/x/net/context"
)

type DriveRequest func(*Drive) error

type Drive struct {
	// buffer chunk
	chunk *Chunk

	vol *Volume

	ctrl     chan DriveRequest
	released chan struct{}

	attached int

	wr *Writer
}

func (wr *Writer) NewDrive() *Drive {
	drv := &Drive{
		ctrl:     make(chan DriveRequest),
		released: make(chan struct{}),

		wr: wr,
	}

	// add as unused
	go func() { wr.unused <- drv }()

	return drv
}

func (drv *Drive) run() {
	for {
		select {
		case err := <-drv.vol.errc:
			// if err was nil, it was a normal exit, mount a new volume and
			// mark as filled.
			if errIO, ok := err.(ErrIO); ok {
				if errIO.err == syscall.ENOSPC {
					// The volume is full.

					// Start two asynchronous operations. Try to offload the
					// stream to another drive (either exclusive or shared, it
					// doesn't matter, Get will do the right thing). Also start
					// a mount of a new volume.

					cnk := errIO.cnk

					// Get a cancellable context
					ctx, cancelGet := context.WithCancel(context.Background())

					// Start the request for another drive
					getNewDrive := make(chan *Drive)
					go func() {
						newdrv, err := drv.wr.Get(ctx, cnk.upstream.pol)
						if err != nil {
							return
						}

						getNewDrive <- newdrv
					}()

					// No context needed, should not be cancelled in any case.
					getNewVolume := make(chan *Volume)
					go func() {
						// XXX
					}()

					for {
						select {
						case newdrv := <-getNewDrive:
							// update the stream
							cnk.upstream.out = newdrv.vol.in

							// Send the chunk that wasn't written to the new drive.
							// The drive will report back to the stream which
							// will continue on the new drive.
							newdrv.vol.in <- cnk

							// wait for mount
							continue
						case newvol := <-getNewVolume:
							// cancel the GetDrive request and break the loop.
							cancelGet()

							_ = newvol

							break
						}
					}
				} else {
					// XXX other error, report to stream. Mark volume as
					// suspicious and mount new one.
					errIO.cnk.upstream.errc <- errIO.err
				}
			}

		case req := <-drv.ctrl:
			if err := req(drv); err != nil {
				// request failed.
			}
		}
	}
}
