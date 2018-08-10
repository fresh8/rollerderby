SHELL := /bin/bash
GIT_HASH := $(shell git log --format='%H' -1)
GIT_ORIGIN := $(shell git remote get-url --push origin)

.PHONY: all
all:
	go build -v -ldflags "-X main.Version=${GIT_HASH} -X main.Source=${GIT_ORIGIN} -extldflags"
