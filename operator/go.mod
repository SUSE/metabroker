module github.com/SUSE/metabroker/operator

go 1.14

require (
	github.com/SUSE/metabroker/apis v0.0.0-00010101000000-000000000000
	github.com/SUSE/metabroker/helm v0.0.0-00010101000000-000000000000
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.3.0
	github.com/google/uuid v1.1.1
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/xeipuuv/gojsonschema v1.2.0
	helm.sh/helm/v3 v3.4.2
	k8s.io/api v0.19.5
	k8s.io/apimachinery v0.19.5
	k8s.io/client-go v0.19.5
	sigs.k8s.io/controller-runtime v0.7.0
	sigs.k8s.io/controller-tools v0.4.1
)

replace (
	github.com/SUSE/metabroker/apis => ../apis
	github.com/SUSE/metabroker/helm => ../helm
)
