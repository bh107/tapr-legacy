GO ?= go

PKG       := ./...
BUILDMODE := install
REPOPATH  := github.com/bh107/tapr

ifeq ($(origin VERSION), undefined)
  VERSION != git rev-parse --short HEAD
endif

.PHONY: all
all: build

.PHONY: release
release: build

.PHONY: build
build: GOFLAGS += -i
build: BUILDMODE = build
build: install

.PHONY: install
install: LDFLAGS += $(shell build/ldflags.sh)
install:
	@echo "GOPATH set to $$GOPATH"
	@echo $(GO) $(BUILDMODE) -v $(GOFLAGS) -ldflags '$(LDFLAGS)'
	@$(GO) $(BUILDMODE) -v $(GOFLAGS) -ldflags '$(LDFLAGS)'

.PHONY: clean
clean:
	$(GO) clean $(GOFLAGS) -i $(REPOPATH)
	find . -name '*.test' -type f -exec rm -f {} \;
