package apis

import (
	v1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, v1.SchemeBuilder.AddToScheme)
}
