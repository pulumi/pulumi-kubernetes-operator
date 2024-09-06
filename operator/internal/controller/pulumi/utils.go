/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pulumi

import (
	"fmt"
	"os"
	"time"
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

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func max(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func exactlyOneOf(these ...bool) bool {
	var found bool
	for _, b := range these {
		if found && b {
			return false
		}
		found = found || b
	}
	return found
}
