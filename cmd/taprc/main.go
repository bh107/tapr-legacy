package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"time"
)

type blob struct {
	size int
	read int
	buf  []byte
}

func (b *blob) Read(p []byte) (n int, err error) {
	if b.read >= b.size {
		log.Print("eof")
		return 0, io.EOF
	}

	p = b.buf[:len(p)]

	b.read += len(p)

	return len(p), nil
}

func (b *blob) Close() error {
	log.Print("closed")
	return nil
}

func main() {
	out, err := exec.Command("uuidgen").Output()
	if err != nil {
		log.Fatal(err)
	}

	buf, err := ioutil.ReadFile("64M-dense")
	if err != nil {
		log.Fatal(err)
	}

	blob := &blob{buf: buf, size: len(buf) * 32}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}

	url := fmt.Sprintf("https://localhost:8080/obj/%s", out)
	req, err := http.NewRequest(
		http.MethodPut, url, blob,
	)

	req.Header.Set("Write-Group", "parallel-write")

	begin := time.Now()
	resp, err := client.Do(req)
	delta := time.Since(begin)

	log.Print(resp.Status)
	log.Printf("request took: %v", delta)
}
