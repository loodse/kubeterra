module github.com/loodse/kubeterra

go 1.12

require (
	github.com/go-logr/logr v0.1.0
	github.com/hashicorp/go-uuid v1.0.1
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/prometheus/client_golang v1.1.0 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.4.0
	gomodules.xyz/jsonpatch/v2 v2.0.1 // indirect
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/kubernetes v1.14.1
	sigs.k8s.io/controller-runtime v0.2.0-rc.0
	sigs.k8s.io/controller-tools v0.2.0-rc.0
)
