makefile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
makefile_dir := $(patsubst %/,%,$(dir $(makefile_path)))

IMG ?= operator:latest

all: operator

install-tools:
	@grep _ tools.go | tr -d '[:space:]_"' | env GOBIN=$(makefile_dir)/.bin xargs go install

generate: generate-rbac generate-webhook

generate-rbac: install-tools
	$(makefile_dir)/.bin/controller-gen rbac:roleName=manager-role paths="./..."

generate-webhook: install-tools
	$(makefile_dir)/.bin/controller-gen webhook paths="./..."

test: generate fmt vet
	go test ./... -coverprofile cover.out

operator: generate fmt vet
	CGO_ENABLED=0 go build -ldflags="-s -w" -o "$(makefile_dir)/.bin/operator" main.go

run: generate fmt vet
	go run ./main.go

fmt:
	go fmt ./...

vet:
	go vet ./...

docker-build: test
	docker build . -t ${IMG}

docker-push:
	docker push ${IMG}
