package changer

import (
	"golang.org/x/net/context"

	"github.com/bh107/tapr/util/mtx"
	"github.com/bh107/tapr/util/mtx/mock"
	"github.com/bh107/tapr/util/mtx/scsi"
	"github.com/bh107/tapr/util/proc"
)

type Changer struct {
	mtx.Interface
	*proc.Proc
}

func New(path string) *Changer {
	return newChanger(path, false)
}

func Mock(path string) *Changer {
	return newChanger(path, true)
}

func (chgr *Changer) ProcessName() string {
	return "changer"
}

func newChanger(path string, mocked bool) *Changer {
	var impl mtx.Interface

	if mocked {
		impl = mock.New(path)
	} else {
		impl = scsi.New(path)
	}

	chgr := &Changer{
		Interface: impl,
	}

	chgr.Proc = proc.Create(chgr)

	return chgr
}

func (chgr *Changer) Handle(ctx context.Context, req proc.HandleFn) error {
	return req(ctx)
}

func (chgr *Changer) Status(ctx context.Context) (*mtx.StatusInfo, error) {
	var status *mtx.StatusInfo

	req := func(ctx context.Context) error {
		var err error
		status, err = mtx.Status(chgr)
		if err != nil {
			return err
		}

		return nil
	}

	if err := chgr.Wait(ctx, req); err != nil {
		return nil, err
	}

	return status, nil
}

func (chgr *Changer) Load(ctx context.Context, slot, drivenum int) error {
	return chgr.Wait(ctx, func(ctx context.Context) error {
		return mtx.Load(chgr, slot, drivenum)
	})
}

func (chgr *Changer) Unload(ctx context.Context, slot, drivenum int) error {
	return chgr.Wait(ctx, func(ctx context.Context) error {
		return mtx.Unload(chgr, slot, drivenum)
	})
}

func (chgr *Changer) Transfer(ctx context.Context, from, to int) error {
	return chgr.Wait(ctx, func(ctx context.Context) error {
		return mtx.Transfer(chgr, from, to)
	})
}
