ifeq ($(origin VERSION), undefined)
  VERSION != git rev-parse --short HEAD
endif
REPOPATH = github.com/bh107/tapr

build:
	go build -v -ldflags "-X $(REPOPATH).Version=$(VERSION)" ./cmd/taprd

buildrace:
	go build -race -v -ldflags "-X $(REPOPATH).Version=$(VERSION)" ./cmd/taprd

run: build
	./taprd --config ./tapr.conf --mock --debug --audit

devrun: cleanall init
	go run cmd/taprd/main.go --config ./tapr.conf --mock --debug --audit

racerun: buildrace
	./taprd --config ./tapr.conf --mock --debug --audit

init: cleanall
	sqlite3 ./inventory.db < ./init.sql

clean:
	rm -f ./taprd

cleanall:
	rm -rf /tmp/ltfs
	rm -f ./taprd ./inventory.db ./chunks.db
