module github.com/pulumi/pulumi-kubernetes-operator

go 1.16

require (
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/operator-framework/operator-lib v0.0.0-20200728190837-b76db547798d
	github.com/operator-framework/operator-sdk v0.19.0
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi/sdk/v3 v3.6.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/whilp/git-urls v1.0.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	k8s.io/api v0.18.19
	k8s.io/apiextensions-apiserver v0.18.19
	k8s.io/apimachinery v0.18.19
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.1
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.18.19 // Required by prometheus-operator
)

// This replaced version includes controller-runtime predicate utilities necessary for v1.0.0 that are still in master.
// Remove this and require the next minor/patch version of controller-runtime (>v0.6.1) when released.
replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.6.1-0.20200724132623-e50c7b819263
