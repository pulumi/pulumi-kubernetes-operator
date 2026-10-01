// Copyright 2026, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
)

// deliveryLaunch is what a Pulumi Delivery pipeline says this stack should deploy. An empty
// Launch means the pipeline has no work for the stack right now.
type deliveryLaunch struct {
	Launch      string            `json:"launch"`
	Repository  string            `json:"repository"`
	Commit      string            `json:"commit"`
	Directory   string            `json:"directory,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

// deliveryLaunchMessagePrefix marks the update as belonging to one launch. Delivery reads it back
// off the finished update to decide which launch completed, so a stale update cannot be mistaken
// for the current one.
const deliveryLaunchMessagePrefix = "pulumi-delivery launch "

func deliveryLaunchMessage(launch string) string {
	return deliveryLaunchMessagePrefix + launch
}

// deliveryClient asks a Pulumi Delivery pipeline for pending work.
type deliveryClient struct {
	backend string
	token   string
	client  *http.Client
}

func newDeliveryClient(backend, token string) *deliveryClient {
	return &deliveryClient{
		backend: strings.TrimSuffix(backend, "/"),
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// PendingLaunch returns the launch the pipeline wants run for stack, or nil when it wants none.
// stack is "<organization>/<project>/<stack>".
func (c *deliveryClient) PendingLaunch(ctx context.Context, stack string) (*deliveryLaunch, error) {
	parts := strings.Split(stack, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("stack %q is not <organization>/<project>/<stack>", stack)
	}
	endpoint := c.backend + "/api/preview/delivery/launches/" +
		url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/" + url.PathEscape(parts[2])

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("asking %s for pending delivery work: %w", c.backend, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("asking %s for pending delivery work: %s", c.backend, resp.Status)
	}

	var launch deliveryLaunch
	if err := json.NewDecoder(resp.Body).Decode(&launch); err != nil {
		return nil, fmt.Errorf("decoding the pending delivery work: %w", err)
	}
	if launch.Launch == "" {
		return nil, nil
	}
	if launch.Commit == "" || launch.Repository == "" {
		return nil, fmt.Errorf("delivery launch %s names no commit to deploy", launch.Launch)
	}
	return &launch, nil
}

// deliveryPollInterval is how long to wait before asking the pipeline again when it has no work.
func deliveryPollInterval(src *shared.DeliverySource) time.Duration {
	if src != nil && src.PollIntervalSeconds != nil && *src.PollIntervalSeconds > 0 {
		return time.Duration(*src.PollIntervalSeconds) * time.Second
	}
	return 15 * time.Second
}

// resolveDeliveryLaunch asks the pipeline what this stack should deploy. It returns nil when the
// pipeline has no pending work.
func (sess *stackReconcilerSession) resolveDeliveryLaunch(ctx context.Context) (*deliveryLaunch, error) {
	src := sess.stack.DeliverySource
	backend := src.Backend
	if backend == "" {
		backend = sess.stack.Backend
	}
	if backend == "" {
		return nil, fmt.Errorf("deliverySource needs a backend, and the stack sets none")
	}
	stackName := src.Stack
	if stackName == "" {
		stackName = sess.stack.Stack
	}

	ref := src.AccessTokenRef
	if ref == nil {
		if envRef, ok := sess.stack.EnvRefs["PULUMI_ACCESS_TOKEN"]; ok {
			ref = &envRef
		}
	}
	if ref == nil {
		return nil, fmt.Errorf("deliverySource needs an accessTokenRef, and the stack has no PULUMI_ACCESS_TOKEN")
	}
	token, err := sess.resolveSecretResourceRef(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolving the delivery access token: %w", err)
	}
	return newDeliveryClient(backend, token).PendingLaunch(ctx, stackName)
}
