module github.com/pulumi/pulumi-kubernetes-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/mod v0.3.0 // indirect
	golang.org/x/tools v0.0.0-20200617161249-6222995d070a // indirect
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
)
