SHELL := /bin/bash
GIT_HASH :=$(git log --format='%H' -1)
GIT_ORIGIN :=$(git remote get-url --push origin)

.PHONY: all
all:
	go build -v -a -ldflags "-X main.Version=${GIT_HASH} -X main.Source=${GIT_ORIGIN} -extldflags"
