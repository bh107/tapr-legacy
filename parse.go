package tapr

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
)

var (
	ErrMtxStatusParseError = errors.New("mtx parse error")
)

type element struct {
	id  int
	vol *Volume
}

func (e *element) String() string {
	return fmt.Sprintf("element: id=%d, volume=%v", e.id, e.vol)
}

type elementRegexes struct {
	inner *regexp.Regexp
	outer *regexp.Regexp
}

var mtxHeaderRegex = regexp.MustCompile(`\s*Storage Changer\s*(.*):(\d*) Drives, (\d*) Slots \( (\d*) Import/Export \)`)
var mtxStatusRegexes = map[string]elementRegexes{
	"drive": elementRegexes{
		outer: regexp.MustCompile(`Data Transfer Element (\d*):(.*)`),
		inner: regexp.MustCompile(`Full \(Storage Element \d* Loaded\):VolumeTag = (.*)`),
	},

	"slot": elementRegexes{
		outer: regexp.MustCompile(`\s*Storage Element (\d*):(.*)`),
		inner: regexp.MustCompile(`Full :VolumeTag=(.*)`),
	},

	"iee": elementRegexes{
		outer: regexp.MustCompile(`\s*Storage Element (\d*) IMPORT/EXPORT:(.*)`),
		inner: regexp.MustCompile(`Full :VolumeTag=(.*)`),
	},
}

type mtxStatusHeader struct {
	devpath   string
	maxDrives int
	numSlots  int
	numIEEs   int
}

func (hdr *mtxStatusHeader) String() string {
	return fmt.Sprintf("mtx header: path=%s, maxdrives=%d, numslots=%d, numiees=%d",
		hdr.devpath, hdr.maxDrives, hdr.numSlots, hdr.numIEEs,
	)
}

type mtxStatus struct {
	hdr *mtxStatusHeader

	// The TransferSlots map enumerates all (data transfer) slots in the library and
	// includes the serial currently at that slot.
	tapedevs []*element

	// The Slots map enumerates all (storage) slots in the library and includes
	// the serial currently at that slot.
	slots []*element

	// The ImportExportSlots map enumerates all (import/export) slots in the
	// library and includes the serial currently at that slot.
	iees []*element
}

func mtxParseStatusHeader(scanner *bufio.Scanner) (*mtxStatusHeader, error) {
	var (
		err     error
		line    string
		matches []string
		header  mtxStatusHeader
	)

	// check if we have something to scan
	if scanner.Scan() {
		line = scanner.Text()

		matches = mtxHeaderRegex.FindStringSubmatch(line)
		if matches == nil {
			return nil, ErrMtxStatusParseError
		}

		header.devpath = matches[1]

		header.maxDrives, err = strconv.Atoi(matches[2])
		if err != nil {
			return nil, err
		}

		header.numIEEs, err = strconv.Atoi(matches[4])
		if err != nil {
			return nil, err
		}

		header.numSlots, err = strconv.Atoi(matches[3])
		if err != nil {
			return nil, err
		}

		// import/export slots are included in the total number of slots
		header.numSlots = header.numSlots - header.numIEEs

		return &header, nil
	}

	// something went wrong, return the possible error or (nil, nil)
	return nil, scanner.Err()
}

func mtxParseStatus(r io.Reader) (*mtxStatus, error) {
	scanner := bufio.NewScanner(r)

	hdr, err := mtxParseStatusHeader(scanner)
	if err != nil {
		return nil, err
	}

	drives, err := mtxParseElements(scanner, "drive", hdr.maxDrives)
	if err != nil {
		return nil, err
	}

	slots, err := mtxParseElements(scanner, "slot", hdr.numSlots)
	if err != nil {
		return nil, err
	}

	iees, err := mtxParseElements(scanner, "iee", hdr.numIEEs)
	if err != nil {
		return nil, err
	}

	return &mtxStatus{
		hdr,
		drives,
		slots,
		iees,
	}, nil
}

func mtxParseElement(line string, regexps elementRegexes) (*element, error) {
	matches := regexps.outer.FindStringSubmatch(line)
	if matches == nil {
		return nil, ErrMtxStatusParseError
	}

	elemnum, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, err
	}

	// fast-track empty slots
	if matches[2] == "Empty" {
		return &element{elemnum, nil}, nil
	}

	matches = regexps.inner.FindStringSubmatch(matches[2])
	if matches == nil {
		return nil, ErrMtxStatusParseError
	}

	return &element{elemnum, &Volume{Serial: matches[1]}}, nil
}

func mtxParseElements(scanner *bufio.Scanner, elementType string, n int) ([]*element, error) {
	elems := make([]*element, n)

	for i := range elems {
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()

		elem, err := mtxParseElement(line, mtxStatusRegexes[elementType])
		if err != nil {
			return nil, err
		}

		elems[i] = elem
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return elems, nil
}
