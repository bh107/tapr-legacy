ifeq ($(origin VERSION), undefined)
  VERSION != git rev-parse --short HEAD
endif
REPOPATH = github.com/bh107/tapr

build:
	go build -ldflags "-X $(REPOPATH).Version=$(VERSION)" ./cmd/taprd

run:
	./taprd --version

clean:
	@rm -f ./taprd
