// Package mtx provides functions for working with an automated library
// changer.
//
// It includes two subpackages, scsi and mock. scsi calls the 'mtx' program and
// mock simulates the use of 'mtx' if no library changer is available doing
// testing/development.
package mtx

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

// SlotType defines the type of slot.
type SlotType int

//go:generate stringer -type=SlotType
const (
	DataTransferSlot SlotType = iota
	StorageSlot
	MailSlot
)

var (
	hdrRegexp          = regexp.MustCompile(`\s*Storage Changer\s*(.*):(\d*) Drives, (\d*) Slots \((\d*) Import/Export \)`)
	driveRegexp        = regexp.MustCompile(`Data Transfer Element (\d*):(.*)`)
	driveElementRegexp = regexp.MustCompile(`Full \(Storage Element (\d*) Loaded\):VolumeTag = (.*)`)
	slotRegexp         = regexp.MustCompile(`\s*Storage Element (\d*):(.*)`)
	mailSlotRegexp     = regexp.MustCompile(`\s*Storage Element (\d*) IMPORT/EXPORT:(.*)`)
	slotElementRegexp  = regexp.MustCompile(`Full :VolumeTag=(.*)`)
)

// The Interface interface describes operations supported by a library auto
// changer.
type Interface interface {
	// Do performs the raw operation identified by cmd.
	Do(args ...string) ([]byte, error)
}

type StatusInfo struct {
	MaxDrives       int
	NumSlots        int
	NumStorageSlots int
	NumMailSlots    int

	Drives []*Slot
	Slots  []*Slot
}

// Volume represents a tape.
type Volume struct {
	// The VOLSER of the tape.
	Serial string

	// The home slot of this volume.
	Home int
}

// String returns a textual representation of the volume.
func (vol *Volume) String() string {
	return vol.Serial
}

// Slot represents a slot in the library.
type Slot struct {
	// The Slot number inside the library.
	Num int

	// Type is the slot type.
	Type SlotType

	// If a volume is in the slot, Vol will be non-nil.
	Vol *Volume
}

// String returns a textual representation of the slot.
func (slot *Slot) String() string {
	return fmt.Sprintf("%s[%d]: %s", slot.Type, slot.Num, slot.Vol)
}

// Load drive with the volume from slot.
func Load(chgr Interface, slotnum, drivenum int) error {
	_, err := chgr.Do(
		"load", strconv.Itoa(slotnum), strconv.Itoa(drivenum),
	)

	return err
}

// Unload a volume from a drive and return it to a slot.
func Unload(chgr Interface, slotnum, drivenum int) error {
	_, err := chgr.Do(
		"unload", strconv.Itoa(slotnum), strconv.Itoa(drivenum),
	)

	return err
}

// Transfer moves a volume from one slot to another.
func Transfer(chgr Interface, slotnum, drivenum int) error {
	_, err := chgr.Do(
		"transfer", strconv.Itoa(slotnum), strconv.Itoa(drivenum),
	)

	return err
}

// MaxDrives returns the number of data transfer elements. Note that this
// does not necessary correspond to the number of actual drives present in
// the system.
func MaxDrives(chgr Interface) (int, error) {
	status, err := chgr.Do("status")
	if err != nil {
		return -1, err
	}

	params, err := params(status)
	if err != nil {
		return -1, err
	}

	return params["maxDrives"], nil
}

// NumSlots returns the number of storage and mail slots.
func NumSlots(chgr Interface) (int, error) {
	status, err := chgr.Do("status")
	if err != nil {
		return -1, err
	}

	params, err := params(status)
	if err != nil {
		return -1, err
	}

	return params["numSlots"], nil
}

// NumStorageSlots returns the number of storage slots.
func NumStorageSlots(chgr Interface) (int, error) {
	status, err := chgr.Do("status")
	if err != nil {
		return -1, err
	}

	params, err := params(status)
	if err != nil {
		return -1, err
	}

	return params["numSlots"] - params["numMailSlots"], nil
}

// NumMailSlots returns the number of mail slots.
func NumMailSlots(chgr Interface) (int, error) {
	status, err := chgr.Do("status")
	if err != nil {
		return -1, err
	}

	params, err := params(status)
	if err != nil {
		return -1, err
	}

	return params["numMailSlots"], nil
}

// Drives returns a slice of data transfer elements. Note that data transfer
// slots typically start with slot id 0.
func Drives(chgr Interface) ([]*Slot, error) {
	status, err := chgr.Do("status")
	if err != nil {
		return nil, err
	}

	elements, err := elements(status)
	if err != nil {
		return nil, err
	}

	return elements["transfer"], nil
}

// Slots returns a slice of storage and mail elements. Note that storage
// slots typically start with slot id 1 and not 0.
func Slots(chgr Interface) ([]*Slot, error) {
	status, err := chgr.Do("status")
	if err != nil {
		return nil, err
	}

	elems, err := elements(status)
	if err != nil {
		return nil, err
	}

	return append(elems["storage"], elems["mail"]...), nil
}

// StorageSlots returns a slice of storage elements. Note that storage
// slots typically start with slot id 1 and not 0.
func StorageSlots(chgr Interface) ([]*Slot, error) {
	status, err := chgr.Do("status")
	if err != nil {
		return nil, err
	}

	elems, err := elements(status)
	if err != nil {
		return nil, err
	}

	return elems["storage"], nil
}

// MailSlots returns a slice of storage elements. Note that mail slots
// typically start with slot ids counting from the id of the last storage
// slot.
func MailSlots(chgr Interface) ([]*Slot, error) {
	status, err := chgr.Do("status")
	if err != nil {
		return nil, err
	}

	elems, err := elements(status)
	if err != nil {
		return nil, err
	}

	return elems["mail"], nil
}

// Status returns a Status structure with combined information about the status
// of the library.
func Status(chgr Interface) (*StatusInfo, error) {
	status, err := chgr.Do("status")
	if err != nil {
		return nil, err
	}

	params, err := params(status)
	elems, err := elements(status)

	return &StatusInfo{
		MaxDrives:       params["maxDrives"],
		NumSlots:        params["numSlots"],
		NumStorageSlots: params["numSlots"] - params["numMailSlots"],
		NumMailSlots:    params["numMailSlots"],

		Drives: elems["transfer"],
		Slots:  append(elems["storage"], elems["mail"]...),
	}, nil
}

func elements(status []byte) (map[string][]*Slot, error) {
	elements := map[string][]*Slot{
		"transfer": make([]*Slot, 0),
		"storage":  make([]*Slot, 0),
		"mail":     make([]*Slot, 0),
	}

	scanner := bufio.NewScanner(bytes.NewReader(status))

	// skip header
	scanner.Scan()

	// scan elements
	var matches []string
	for scanner.Scan() {
		line := scanner.Text()

		// match data transfer elements
		matches = driveRegexp.FindStringSubmatch(line)
		if matches != nil {
			elemnum, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, err
			}

			slot := &Slot{Num: elemnum, Type: DataTransferSlot}

			if matches[2] != "Empty" {
				matches = driveElementRegexp.FindStringSubmatch(matches[2])
				if matches == nil {
					return nil, errors.New("failed to parse transfer element")
				}

				home, err := strconv.Atoi(matches[1])
				if err != nil {
					return nil, err
				}

				slot.Vol = &Volume{Serial: matches[2], Home: home}
			}

			elements["transfer"] = append(elements["transfer"], slot)

			continue
		}

		// match storage elements
		matches = slotRegexp.FindStringSubmatch(line)
		if matches != nil {
			elemnum, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, err
			}

			slot := &Slot{Num: elemnum, Type: StorageSlot}

			if matches[2] != "Empty" {
				match := slotElementRegexp.FindStringSubmatch(matches[2])
				if match == nil {
					return nil, errors.New("failed to parse slot element: " + matches[2])
				}

				slot.Vol = &Volume{Serial: match[1], Home: elemnum}
			}

			elements["storage"] = append(elements["storage"], slot)

			continue
		}

		// match mailslot elements
		matches = mailSlotRegexp.FindStringSubmatch(line)
		if matches != nil {
			elemnum, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, err
			}

			slot := &Slot{Num: elemnum, Type: MailSlot}

			if matches[2] != "Empty" {
				matches = slotElementRegexp.FindStringSubmatch(matches[2])
				if matches == nil {
					return nil, errors.New("failed to parse slot element")
				}

				slot.Vol = &Volume{Serial: matches[1], Home: elemnum}
			}

			elements["mail"] = append(elements["mail"], slot)

			continue
		}

		return nil, errors.New("failed to parse slot")
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return elements, nil
}

func params(status []byte) (map[string]int, error) {
	params := make(map[string]int)

	scanner := bufio.NewScanner(bytes.NewReader(status))

	var err error
	if scanner.Scan() {
		line := scanner.Text()

		matches := hdrRegexp.FindStringSubmatch(line)
		if matches == nil {
			return nil, errors.New("failed to match mtx status header")
		}

		params["maxDrives"], err = strconv.Atoi(matches[2])
		if err != nil {
			return nil, err
		}

		params["numSlots"], err = strconv.Atoi(matches[3])
		if err != nil {
			return nil, err
		}

		params["numMailSlots"], err = strconv.Atoi(matches[4])
		if err != nil {
			return nil, err
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return params, nil
}
