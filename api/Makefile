makefile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
makefile_dir := $(patsubst %/,%,$(dir $(makefile_path)))

install-tools:
	@export GOBIN=$(makefile_dir)/.bin; \
		cat tools.go | grep _ | tr -d '[:space:]_"' | xargs go install

generate: generate-go generate-crd

generate-go: install-tools
	$(makefile_dir)/.bin/controller-gen object:headerFile="boilerplate.go.txt" paths="./..."

generate-crd: install-tools
	$(makefile_dir)/.bin/controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=crd

install-crd: generate-crd
	kubectl apply --filename "$(makefile_dir)/crd"

uninstall-crd: generate-crd
	kubectl delete --filename "$(makefile_dir)/crd"

fmt:
	go fmt ./...

vet:
	go vet ./...
