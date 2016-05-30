package stream

import (
	"os"
	"path"
	"testing"
)

type fs struct{}

func (fs *fs) Create(fname string) (*os.File, error) {
	return os.Create(path.Join("/tmp", fname))
}

func TestStream(t *testing.T) {
	wr := NewWriter(&fs{})

	s, errc := New(wr)

	f, err := os.Open("/dev/urandom")
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 4096)
	for {
		select {
		case err := <-errc:
			t.Log(err)
		default:
			_, err := f.Read(buf)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
