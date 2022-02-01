// Copyright 2021, Pulumi Corporation.  All rights reserved.
package stack

import (
	"fmt"
	"os"
)

// Environment variable to toggle namespace behavior
const INFERNS = "PULUMI_INFER_NAMESPACE"

// inferNamespace returns the namespace that is passed in when
// the environment variable is set.
// This is used to maintain the old behavior of not using
// the service-account namespace when using in-cluster config.
// Ideally, this env will be removed and become the default in
// the future.
func inferNamespace(namespace string) string {
	if os.Getenv(INFERNS) != "" {
		return fmt.Sprintf("namespace: %s", namespace)
	}

	return ""
}
