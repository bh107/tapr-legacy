// Package mock implements a mocked library auto changer that simulates 'mtx'.
package mock

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"

	"github.com/bh107/tapr/util/mtx"
)

// Changer represents a mock library auto changer.
type Changer struct {
	name string

	drives []*mtx.Slot
	slots  []*mtx.Slot

	numDrives       int
	numStorageSlots int
	numMailSlots    int
}

var DefaultSpec = &Spec{8, 32, 4, 16}

type Spec struct {
	NumDrives       int
	NumStorageSlots int
	NumMailSlots    int
	NumVolumes      int
}

var serialCounter int
var serialPrefixes = []rune{'A', 'B', 'C', 'D'}

func New(name string) *Changer {
	if serialCounter > len(serialPrefixes)-1 {
		panic("cannot allocate serial prefix")
	}
	serialPrefix := serialPrefixes[serialCounter]
	serialCounter++

	return NewWithSpec(name, serialPrefix, DefaultSpec)
}

// New returns a mock library auto changer initialized with numDrives slots for
// drives, numStorageSlots slots for volume storage and numVolumes slots as
// import/export mail slots. It populates the first numVolumes storage slots
// with mock volumes with serials starting at S00000L6. A cleaning cartridge
// with serial CLN000L1 is added to the last storage slot and an extra volume
// is added to the last import/export slot.
func NewWithSpec(name string, serialPrefix rune, spec *Spec) *Changer {
	chgr := &Changer{
		name:            name,
		drives:          make([]*mtx.Slot, spec.NumDrives),
		slots:           make([]*mtx.Slot, spec.NumStorageSlots+spec.NumMailSlots),
		numDrives:       spec.NumDrives,
		numStorageSlots: spec.NumStorageSlots,
		numMailSlots:    spec.NumMailSlots,
	}

	for i := range chgr.drives {
		chgr.drives[i] = &mtx.Slot{Num: i, Type: mtx.DataTransferSlot}
	}

	for i := range chgr.slots {
		chgr.slots[i] = &mtx.Slot{Num: i + 1, Type: mtx.StorageSlot}

		// fill half of the storage slots with volumes
		if i < spec.NumVolumes {
			chgr.slots[i].Vol = &mtx.Volume{
				Serial: fmt.Sprintf("%c%05dL6", serialPrefix, i),
				Home:   i + 1,
			}
		}

		// put a cleaning cartridge in the last storage slot for good measure
		if i == spec.NumStorageSlots-1 {
			chgr.slots[i].Vol = &mtx.Volume{
				Serial: fmt.Sprintf("CLN%03dL1", serialCounter),
				Home:   i + 1,
			}
		}

		if i >= spec.NumStorageSlots {
			chgr.slots[i].Type = mtx.MailSlot
		}

		// put a volume in the last mail slot
		if i == spec.NumStorageSlots+spec.NumMailSlots-1 {
			chgr.slots[i].Vol = &mtx.Volume{
				Serial: fmt.Sprintf("%c%05dL6", serialPrefix, spec.NumVolumes),
				Home:   i,
			}
		}
	}

	return chgr
}

func mtxSlotString(slot *mtx.Slot) string {
	if slot.Vol == nil {
		return "Empty"
	}

	if slot.Type == mtx.DataTransferSlot {
		return fmt.Sprintf("Full (Storage Element %d Loaded):VolumeTag = %s",
			slot.Vol.Home, slot.Vol.Serial,
		)
	}

	return fmt.Sprintf("Full :VolumeTag=%s", slot.Vol.Serial)
}

func (chgr *Changer) load(slotnum int, drivenum int) error {
	slot := chgr.slots[slotnum-1]
	chgr.drives[drivenum].Vol = slot.Vol
	slot.Vol = nil

	return nil
}

func (chgr *Changer) unload(slotnum int, drivenum int) error {
	drv := chgr.drives[drivenum]
	if slotnum == 0 {
		slotnum = drv.Vol.Home
	}

	chgr.slots[slotnum-1].Vol = drv.Vol
	drv.Vol = nil

	return nil
}

func (chgr *Changer) transfer(from, to int) error {
	from -= 1
	to -= 1
	if chgr.slots[from].Vol == nil {
		return errors.New("unable to transfer volume: no volume in slot")
	}

	if chgr.slots[to].Vol != nil {
		return errors.New("unable to transfer volume: slot already occupied")
	}

	chgr.slots[to].Vol = chgr.slots[from].Vol
	chgr.slots[from].Vol = nil

	return nil
}

// Do simulates performaing the given mtx command.
func (chgr *Changer) Do(args ...string) ([]byte, error) {
	if len(args) < 1 {
		return nil, errors.New("no command given")
	}

	cmd := args[0]

	if cmd == "status" {
		return chgr.status()
	}

	if len(args) != 3 {
		return nil, errors.New("wrong number of arguments")
	}

	a, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, err
	}

	b, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, err
	}

	switch cmd {
	case "load":
		return nil, chgr.load(a, b)
	case "unload":
		return nil, chgr.unload(a, b)
	case "transfer":
		return nil, chgr.transfer(a, b)
	}

	return nil, errors.New("mtx/mock: unknown or unsupported mtx command")
}

func (chgr *Changer) status() ([]byte, error) {
	var tmp string
	var buf bytes.Buffer

	// compose header
	tmp = fmt.Sprintf("  Storage Changer %s:%d Drives, %d Slots ( %d Import/Export )\n",
		"/dev/mock", chgr.numDrives, chgr.numStorageSlots+chgr.numMailSlots,
		chgr.numMailSlots,
	)

	// write header
	_, _ = buf.WriteString(tmp)

	// write data transfer elements
	for i, slot := range chgr.drives {
		tmp = fmt.Sprintf("Data Transfer Element %d:%s\n", i, mtxSlotString(slot))
		_, _ = buf.WriteString(tmp)
	}

	// write storage elements
	for _, slot := range chgr.slots {
		extra := ""
		if slot.Type == mtx.MailSlot {
			extra = " IMPORT/EXPORT"
		}

		tmp = fmt.Sprintf("      Storage Element %d%s:%s\n", slot.Num, extra, mtxSlotString(slot))
		_, _ = buf.WriteString(tmp)
	}

	return buf.Bytes(), nil
}
