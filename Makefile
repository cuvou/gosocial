SHELL := /bin/bash

VERSION=$(shell egrep -e 'Version\s+=' pkg/branding/branding.go | head -n 1 | cut -d '"' -f 2)
BUILD=$(shell git describe --always)
BUILD_DATE=$(shell date +"%Y-%m-%dT%H:%M:%S%z")
CURDIR=$(shell curdir)

# Inject the build version (commit hash) into the executable.
LDFLAGS := -ldflags "-X main.Build=$(BUILD) -X main.BuildDate=$(BUILD_DATE)"

all: build

.PHONY: setup
setup:
	go get ./...

.PHONY: build
build:
	go build $(LDFLAGS) -o gosocial cmd/gosocial/main.go

.PHONY: test
test:
	go test ./...

.PHONY: run
run:
	go run cmd/gosocial/main.go web --debug
