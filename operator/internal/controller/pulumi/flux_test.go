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

package pulumi

import (
	"reflect"
	"strings"
	"testing"
	"time"

	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// yamlToUnstructured is a helper to convert a YAML string to an unstructured object.
func yamlToUnstructured(t *testing.T, yml string) unstructured.Unstructured {
	t.Helper()
	m := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yml), &m)
	if err != nil {
		t.Fatalf("error parsing yaml: %v", err)
	}
	return unstructured.Unstructured{Object: m}
}

var replayGitRepository = []string{`
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  generation: 1
  name: example
  namespace: default
  resourceVersion: "2323206"
spec:
  interval: 30s
  ref:
    branch: main
  timeout: 60s
  url: https://github.com/EronWright/simple-random.git
status:
  observedGeneration: -1
`, `
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  generation: 1
  name: example
  namespace: default
  resourceVersion: "2323213"
spec:
  interval: 30s
  ref:
    branch: main
  timeout: 60s
  url: https://github.com/EronWright/simple-random.git
status:
  conditions:
  - lastTransitionTime: "2025-05-28T00:45:47Z"
    message: 'building artifact: new upstream revision ''main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de'''
    observedGeneration: 1
    reason: Progressing
    status: "True"
    type: Reconciling
  - lastTransitionTime: "2025-05-28T00:45:47Z"
    message: 'building artifact: new upstream revision ''main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de'''
    observedGeneration: 1
    reason: Progressing
    status: Unknown
    type: Ready
  observedGeneration: -1
`, `
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  generation: 1
  name: example
  namespace: default
  resourceVersion: "2323215"
spec:
  interval: 30s
  ref:
    branch: main
  timeout: 60s
  url: https://github.com/EronWright/simple-random.git
status:
  conditions:
  - lastTransitionTime: "2025-05-28T00:45:47Z"
    message: stored artifact for revision 'main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de'
    observedGeneration: 1
    reason: Succeeded
    status: "True"
    type: Ready
  - lastTransitionTime: "2025-05-28T00:45:47Z"
    message: stored artifact for revision 'main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de'
    observedGeneration: 1
    reason: Succeeded
    status: "True"
    type: ArtifactInStorage
  observedGeneration: -1
`, `
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  generation: 1
  name: example
  namespace: default
  resourceVersion: "2323216"
spec:
  interval: 30s
  ref:
    branch: main
  timeout: 60s
  url: https://github.com/EronWright/simple-random.git
status:
  artifact:
    digest: sha256:c84370fd81474ea743d030a86b74b847c714af74d87b038de5fa41d21336e0e6
    lastUpdateTime: "2025-05-28T00:45:47Z"
    path: gitrepository/default/example/9eb98f3e74333ae8abadf0d73356a1bca9d3b9de.tar.gz
    revision: main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de
    size: 585
    url: http://source-controller.flux-system.svc.cluster.local./gitrepository/default/example/9eb98f3e74333ae8abadf0d73356a1bca9d3b9de.tar.gz
  conditions:
  - lastTransitionTime: "2025-05-28T00:45:47Z"
    message: stored artifact for revision 'main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de'
    observedGeneration: 1
    reason: Succeeded
    status: "True"
    type: Ready
  - lastTransitionTime: "2025-05-28T00:45:47Z"
    message: stored artifact for revision 'main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de'
    observedGeneration: 1
    reason: Succeeded
    status: "True"
    type: ArtifactInStorage
  observedGeneration: 1
`,
	`
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  generation: 2
  name: example
  namespace: default
  resourceVersion: "2323573"
spec:
  interval: 32s
  ref:
    branch: main
  timeout: 60s
  url: https://github.com/EronWright/simple-random.git
status:
  artifact:
    digest: sha256:c84370fd81474ea743d030a86b74b847c714af74d87b038de5fa41d21336e0e6
    lastUpdateTime: "2025-05-28T00:45:47Z"
    path: gitrepository/default/example/9eb98f3e74333ae8abadf0d73356a1bca9d3b9de.tar.gz
    revision: main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de
    size: 585
    url: http://source-controller.flux-system.svc.cluster.local./gitrepository/default/example/9eb98f3e74333ae8abadf0d73356a1bca9d3b9de.tar.gz
  conditions:
  - lastTransitionTime: "2025-05-28T00:45:47Z"
    message: stored artifact for revision 'main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de'
    observedGeneration: 1
    reason: Succeeded
    status: "True"
    type: Ready
  - lastTransitionTime: "2025-05-28T00:45:47Z"
    message: stored artifact for revision 'main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de'
    observedGeneration: 1
    reason: Succeeded
    status: "True"
    type: ArtifactInStorage
  observedGeneration: 1
`, `
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  generation: 2
  name: example
  namespace: default
  resourceVersion: "2323790"
spec:
  interval: 32s
  ref:
    branch: main
  timeout: 60s
  url: https://github.com/EronWright/simple-random.git
status:
  artifact:
    digest: sha256:c84370fd81474ea743d030a86b74b847c714af74d87b038de5fa41d21336e0e6
    lastUpdateTime: "2025-05-28T00:45:47Z"
    path: gitrepository/default/example/9eb98f3e74333ae8abadf0d73356a1bca9d3b9de.tar.gz
    revision: main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de
    size: 585
    url: http://source-controller.flux-system.svc.cluster.local./gitrepository/default/example/9eb98f3e74333ae8abadf0d73356a1bca9d3b9de.tar.gz
  conditions:
  - lastTransitionTime: "2025-05-28T00:49:21Z"
    message: stored artifact for revision 'main@sha1:00ca386d43834c41d9626b6d93137d396fd771ed'
    observedGeneration: 2
    reason: Succeeded
    status: "True"
    type: Ready
  - lastTransitionTime: "2025-05-28T00:49:21Z"
    message: stored artifact for revision 'main@sha1:00ca386d43834c41d9626b6d93137d396fd771ed'
    observedGeneration: 2
    reason: Succeeded
    status: "True"
    type: ArtifactInStorage
  observedGeneration: 2
`, `
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  generation: 2
  name: example
  namespace: default
  resourceVersion: "2323791"
spec:
  interval: 32s
  ref:
    branch: main
  timeout: 60s
  url: https://github.com/EronWright/simple-random.git
status:
  artifact:
    digest: sha256:617e0171fd6c8d0b01c04ca9ef788b6447fab174c48411fb9a1cb857a9b4aae5
    lastUpdateTime: "2025-05-28T00:49:21Z"
    path: gitrepository/default/example/00ca386d43834c41d9626b6d93137d396fd771ed.tar.gz
    revision: main@sha1:00ca386d43834c41d9626b6d93137d396fd771ed
    size: 585
    url: http://source-controller.flux-system.svc.cluster.local./gitrepository/default/example/00ca386d43834c41d9626b6d93137d396fd771ed.tar.gz
  conditions:
  - lastTransitionTime: "2025-05-28T00:49:21Z"
    message: stored artifact for revision 'main@sha1:00ca386d43834c41d9626b6d93137d396fd771ed'
    observedGeneration: 2
    reason: Succeeded
    status: "True"
    type: Ready
  - lastTransitionTime: "2025-05-28T00:49:21Z"
    message: stored artifact for revision 'main@sha1:00ca386d43834c41d9626b6d93137d396fd771ed'
    observedGeneration: 2
    reason: Succeeded
    status: "True"
    type: ArtifactInStorage
  observedGeneration: 2
`,
}

func TestGetArtifact(t *testing.T) {

	parseTime := func(s string) metav1.Time {
		t.Helper()
		pt, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("error parsing time %q: %v", s, err)
		}
		return metav1.NewTime(pt.Local())
	}

	tests := []struct {
		name, source string
		want         *pulumiv1.Artifact
		wantErr      bool
	}{
		{
			name:   "before-reconciliation",
			source: replayGitRepository[0],
			want:   nil,
		},
		{
			name:   "progressing",
			source: replayGitRepository[1],
			want:   nil,
		},
		{
			name:   "updating status",
			source: replayGitRepository[2],
			want:   nil,
		},
		{
			name:   "ready",
			source: replayGitRepository[3],
			want: &pulumiv1.Artifact{
				Digest:         "sha256:c84370fd81474ea743d030a86b74b847c714af74d87b038de5fa41d21336e0e6",
				LastUpdateTime: parseTime("2025-05-28T00:45:47Z"),
				Path:           "gitrepository/default/example/9eb98f3e74333ae8abadf0d73356a1bca9d3b9de.tar.gz",
				Revision:       "main@sha1:9eb98f3e74333ae8abadf0d73356a1bca9d3b9de",
				Size:           ptr.To(int64(585)),
				URL:            "http://source-controller.flux-system.svc.cluster.local./gitrepository/default/example/9eb98f3e74333ae8abadf0d73356a1bca9d3b9de.tar.gz",
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			obj := yamlToUnstructured(t, strings.TrimSpace(tc.source))
			artifact, gotArtifact, err := getArtifact(obj)
			if (err != nil) != tc.wantErr {
				t.Fatalf("getField() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.want == nil && (gotArtifact || artifact != nil) {
				t.Errorf("getArtifact() gotArtifact = %v, artifact = %v, want nil", gotArtifact, artifact)
			}
			if tc.want != nil && (artifact == nil || !reflect.DeepEqual(*artifact, *tc.want)) {
				t.Errorf("getArtifact() = %v, want %v", artifact, tc.want)
			}
		})
	}
}

func TestSourceRevisionChangePredicate(t *testing.T) {
	want := []bool{
		false,
		false,
		false,
		true, // .status.artifact is set
		false,
		false,
		true, // .status.artifact is updated
	}

	var old *unstructured.Unstructured
	for i := 0; i < len(replayGitRepository); i++ {
		obj := yamlToUnstructured(t, strings.TrimSpace(replayGitRepository[i]))

		p := SourceRevisionChangePredicate{}
		got := p.Update(event.UpdateEvent{
			ObjectOld: old,
			ObjectNew: &obj,
		})
		if got != want[i] {
			t.Errorf("SourceRevisionChangePredicate.Update() = %v, want %v", got, want[i])
		}
		old = &obj
	}
}
