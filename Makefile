DIST    := dist
BINARY 	:= localtest

VERSION := $(shell git describe --tags --always --dirty)
COMMIT  := $(shell git rev-parse --short HEAD)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

PREFIX 	?= /usr/local/bin

LDFLAGS := -X 'main.buildVersion=$(VERSION)' \
           -X 'main.buildCommit=$(COMMIT)' \
           -X 'main.buildDate=$(DATE)'

all: build

build:
	@mkdir -p $(DIST)
	@echo ">> Running go generate ..."
	go generate ./...
	@echo ">> Building $(DIST)/$(BINARY) binary ..."
	go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) .
.PHONY: build

install:
	@echo ">> Installing $(DIST)/$(BINARY) to $(PREFIX) ..."
	install -m 755 $(DIST)/$(BINARY) $(PREFIX)/$(BINARY)
.PHONY: install

clean:
	@echo ">> Cleaning..."
	rm -rf $(DIST)
	rm -f *_gen.go
.PHONY: clean
