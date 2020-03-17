module github.com/keikoproj/active-monitor

go 1.13

require (
	github.com/argoproj/argo v2.3.0+incompatible
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.1.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/prometheus/client_golang v1.0.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.4.2
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	k8s.io/api v0.17.0
	k8s.io/apiextensions-apiserver v0.17.0 // indirect
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.17.0
	k8s.io/utils v0.0.0-20191218082557-f07c713de883 // indirect
	sigs.k8s.io/controller-runtime v0.4.0
)
