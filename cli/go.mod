module github.com/SUSE/metabroker/cli

go 1.14

require (
	github.com/SUSE/metabroker/apis v0.0.0-00010101000000-000000000000
	github.com/SUSE/metabroker/helm v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.0
	k8s.io/api v0.19.5
	k8s.io/apimachinery v0.19.5
	k8s.io/client-go v0.19.5
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/SUSE/metabroker/apis => ../apis
	github.com/SUSE/metabroker/helm => ../helm
)
