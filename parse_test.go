package tapr

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestMtxStatusParser(t *testing.T) {
	b, err := ioutil.ReadFile("./_test/mtx.out")
	if err != nil {
		t.Fatal(err)
	}

	status, err := mtxParseStatus(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}

	if status.hdr.devpath != "/dev/sg7" {
		t.Fatalf("expected /dev/sg7, got %s", status.hdr.devpath)
	}

	if status.hdr.maxDrives != 20 {
		t.Fatalf("expected 20, got %d", status.hdr.maxDrives)
	}

	if status.hdr.numSlots != 300 {
		t.Fatalf("expected 300, got %d", status.hdr.numSlots)
	}

	if status.hdr.numIEEs != 4 {
		t.Fatalf("expected 4, got %d", status.hdr.numIEEs)
	}

	elem := status.tapedevs[0]
	if elem.id != 0 || elem.vol.serial != "S00004L6" {
		t.Fatal(elem)
	}

	if len(status.tapedevs) != 20 {
		t.Fatal(status.tapedevs)
	}

	for _, elem := range status.tapedevs[1:] {
		if elem.vol != nil {
			t.Fatal(elem)
		}
	}

	if len(status.slots) != 300 {
		t.Fatal(status.slots)
	}

	elem = status.slots[299]
	if elem.id != 300 || elem.vol.serial != "CLN000L1" {
		t.Fatal(elem)
	}

	if len(status.iees) != 4 {
		t.Fatal(status.iees)
	}

	elem = status.iees[3]
	if elem.id != 304 || elem.vol.serial != "S00003L6" {
		t.Fatal(elem)
	}
}
