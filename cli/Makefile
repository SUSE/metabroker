makefile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
makefile_dir := $(patsubst %/,%,$(dir $(makefile_path)))

all: cli

cli: fmt vet
	CGO_ENABLED=0 go build -ldflags="-s -w" -o "$(makefile_dir)/.bin/metabrokerctl" main.go

fmt:
	go fmt ./...

vet:
	go vet ./...
