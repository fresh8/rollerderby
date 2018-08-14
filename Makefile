SHELL := /bin/bash
GIT_HASH := $(shell git log --format='%H' -1)
GIT_ORIGIN := $(shell git remote get-url --push origin)

.PHONY: all
all: build test

.PHONY: install
install: $(GOPATH)/bin/rollerderby

.PHONY: build
build: rollerderby

.PHONY: test
test:
	go test -v ./...

rollerderby: *.go
	go build -v -ldflags "-X main.Version=${GIT_HASH} -X main.Source=${GIT_ORIGIN} -extldflags"

$(GOPATH)/bin/rollerderby: *.go
	go install -v -ldflags "-X main.Version=${GIT_HASH} -X main.Source=${GIT_ORIGIN} -extldflags"

