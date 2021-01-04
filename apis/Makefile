makefile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
makefile_dir := $(patsubst %/,%,$(dir $(makefile_path)))

install-tools:
	@export GOBIN="$(makefile_dir)/.bin"; \
		cat tools.go | grep _ | tr --delete '[:blank:]_"' | xargs go install

generate: generate-k8s generate-crd

generate-k8s: install-tools
	"$(makefile_dir)/.bin/controller-gen" object:headerFile="boilerplate.go.txt" paths="./servicebroker/..."
	"$(makefile_dir)/.bin/client-gen" \
		--clientset-name "versioned" \
		--input-base "github.com/SUSE/metabroker/apis" \
		--input "servicebroker/v1alpha1" \
		--output-package "github.com/SUSE/metabroker/apis/generated/clientset" \
		--output-base "$(makefile_dir)/.tmp" \
		--go-header-file "$(makefile_dir)/boilerplate.go.txt"
	rm -rf "$(makefile_dir)/generated"
	mv "$(makefile_dir)/.tmp/github.com/SUSE/metabroker/apis/generated" "$(makefile_dir)/generated"
	rm -rf "$(makefile_dir)/.tmp"

generate-crd: install-tools
	"$(makefile_dir)/.bin/controller-gen" crd rbac:roleName=manager-role webhook paths="./servicebroker/..." output:crd:artifacts:config=crd

install-crd: generate-crd
	kubectl apply --filename "$(makefile_dir)/crd"

uninstall-crd: generate-crd
	kubectl delete --filename "$(makefile_dir)/crd"

fmt:
	go fmt ./...

vet:
	go vet ./...
