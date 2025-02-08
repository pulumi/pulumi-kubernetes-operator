// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"strings"

	pb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	"google.golang.org/grpc/status"
)

type knownPulumiErrors map[string]*pb.PulumiErrorInfo

// knownErrors is a map where the keys are string snippets of the CLI error message,
// and the values are the structured error we want to return to the client.
var knownErrors = knownPulumiErrors{
	"Conflict: Another update is currently in progress": {
		Message: "Another update is currently in progress",
		Reason:  "UpdateConflict",
		Code:    409,
	},
}

// withPulumiErrorInfo iterates over known errors and checks if the provided error matches any of them.
// If it does, it appends the structured error to the status.
func withPulumiErrorInfo(st *status.Status, err error) *status.Status {
	if err == nil {
		return st
	}

	for errString, structuredErr := range knownErrors {
		if !strings.Contains(err.Error(), errString) {
			continue
		}

		ds, err := st.WithDetails(structuredErr)
		if err != nil {
			// If we can't add the structured error, return the original status.
			return st
		}
		return ds
	}
	return st
}
