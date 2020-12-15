module github.com/SUSE/metabroker/operator

go 1.13

require (
	github.com/SUSE/metabroker/operator/helm v0.0.0-00010101000000-000000000000
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/google/uuid v1.1.1
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/prometheus/client_golang v1.3.0 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0
	go.opencensus.io v0.22.2 // indirect
	go.uber.org/zap v1.13.0 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/tools v0.0.0-20200103221440-774c71fcf114 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	helm.sh/helm/v3 v3.1.3
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	sigs.k8s.io/controller-runtime v0.5.0
)

replace github.com/SUSE/metabroker/operator/helm => ./helm
