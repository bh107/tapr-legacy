package stream

import (
	"github.com/bh107/tapr/stream/policy"
	"golang.org/x/net/context"
)

// Mover abstracts shared (multi-plexed), exclusive and parallel access to
// backend storage.
type Writer struct {
	root string

	unused chan *Drive
	shared chan *Drive

	exclusiveWaiter chan struct{}
}

// NewWriter creates a new Writer, anchored at the file system denoted by root.
func NewWriter(root string) *Writer {
	return &Writer{
		root: root,

		unused: make(chan *Drive),
		shared: make(chan *Drive),
	}
}

// Attach attaches a Stream to the Writer.
func (wr *Writer) Attach(ctx context.Context, s *Stream) error {
	drv, err := wr.Get(ctx, s.pol)
	if err != nil {
		return err
	}

	s.out = drv.vol.in

	return nil
}

// Get returns a shared/exclusive writer or nil if operation is cancelled
// before one could be acquired.
func (wr *Writer) Get(ctx context.Context, pol *policy.Policy) (*Drive, error) {
	var drv *Drive
	var err error

	if pol.Exclusive {
		drv, err = wr.GetExclusive(ctx)
	} else {
		drv, err = wr.GetShared(ctx)
	}

	if err != nil {
		return nil, err
	}

	drv.ctrl <- func(drv *Drive) error {
		drv.attached++

		return nil
	}

	return drv, nil
}

// GetShared returns a shared writer or nil if operation is cancelled before
// one could be acquired.
func (wr *Writer) GetShared(ctx context.Context) (*Drive, error) {
	var drv *Drive

	select {
	case <-wr.exclusiveWaiter:
		// don't grab anything at the buffet, wait for for the exclusive writer
		// to get a chance at the table and leave.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case drv = <-wr.unused:
		}
	default:
		// try either
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case drv = <-wr.unused:
		case drv = <-wr.shared:
		}
	}

	// send it back to the shared channel if anyone wants one from it, or to
	// the unused, if it is released before we have a chance to send it.
	go func() {
		select {
		case <-drv.released:
			// return to unused channel
			wr.unused <- drv
		case wr.shared <- drv:
		}
	}()

	return drv, nil
}

// GetExclusive returns a writer or nil if timeout was closed before one could
// be acquired.
func (wr *Writer) GetExclusive(ctx context.Context) (*Drive, error) {
	// tell the communal hippies that we want the table for our selves and
	// don't wanna starve!
	go func() { wr.exclusiveWaiter <- struct{}{} }()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case drv := <-wr.unused:
		// wait until released, then put back as unused
		go func() { <-drv.released; wr.unused <- drv }()

		return drv, nil
	}
}

// Release returns the drive as unused if after release, no streams are
// attached.
func (wr *Writer) Release(drv *Drive) {
	drv.ctrl <- func(drv *Drive) error {
		drv.attached--

		if drv.attached == 0 {
			// release it (go back to unused)
			drv.released <- struct{}{}
		}

		return nil
	}
}
