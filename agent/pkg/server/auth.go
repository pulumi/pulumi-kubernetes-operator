/*
Copyright © 2024 Pulumi Corporation

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

package server

import (
	"context"
	"fmt"
	"time"

	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/token/cache"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	webhooktoken "k8s.io/apiserver/plugin/pkg/authenticator/token/webhook"
	authenticationv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
)

type AuthClient interface {
	authenticationv1.AuthenticationV1Interface
	authorizationv1.AuthorizationV1Interface
}

type KubeAuthOptions struct {
	WorkspaceName types.NamespacedName
}

// withKubeAuth provides a gRPC auth.AuthFunc for authentication and authorization.
// Requests will be authenticated (via TokenReviews) and authorized (via SubjectAccessReviews) with the
// kube-apiserver.
// For the authentication and authorization the agent needs a role
// with the following rules:
// * apiGroups: authentication.k8s.io, resources: tokenreviews, verbs: create
// * apiGroups: authorization.k8s.io, resources: subjectaccessreviews, verbs: create
//
// To make RPC requests e.g. as the Operator the client needs a role
// with the following rule:
// * apiGroups: auto.pulumi.com, resources: workspaces/rpc, verbs: use
func NewKubeAuth(rootLogger *zap.Logger, config *rest.Config, opts KubeAuthOptions) (grpc_auth.AuthFunc, error) {
	authenticationV1Client, err := authenticationv1.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create authentication client: %w", err)
	}
	authorizationV1Client, err := authorizationv1.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create authorization client: %w", err)
	}

	tokenAuth, err := webhooktoken.NewFromInterface(
		authenticationV1Client,
		nil,
		wait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   1.5,
			Jitter:   0.2,
			Steps:    5,
		},
		10*time.Second, /* requestTimeout */
		webhooktoken.AuthenticatorMetrics{
			RecordRequestTotal:   RecordRequestTotal,
			RecordRequestLatency: RecordRequestLatency,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook authenticator: %w", err)
	}
	delegatingAuthenticator := cache.New(tokenAuth, false, 1*time.Hour /* successTTL */, 1*time.Minute /* failureTTL */)

	authorizerConfig := authorizerfactory.DelegatingAuthorizerConfig{
		SubjectAccessReviewClient: authorizationV1Client,
		AllowCacheTTL:             5 * time.Minute,
		DenyCacheTTL:              5 * time.Second,
		// wait.Backoff is copied from: https://github.com/kubernetes/apiserver/blob/v0.29.0/pkg/server/options/authentication.go#L43-L50
		// options.DefaultAuthWebhookRetryBackoff is not used to avoid a dependency on "k8s.io/apiserver/pkg/server/options".
		WebhookRetryBackoff: &wait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   1.5,
			Jitter:   0.2,
			Steps:    5,
		},
	}
	delegatingAuthorizer, err := authorizerConfig.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create authorizer: %w", err)
	}

	a := &kubeAuth{
		log:           rootLogger.Named("grpc").Sugar(),
		authn:         delegatingAuthenticator,
		authz:         delegatingAuthorizer,
		workspaceName: opts.WorkspaceName,
	}
	return a.Authenticate, nil
}

type kubeAuth struct {
	log           *zap.SugaredLogger
	authn         authenticator.Token
	authz         authorizer.Authorizer
	workspaceName types.NamespacedName
}

func (a *kubeAuth) Authenticate(ctx context.Context) (context.Context, error) {
	tags := grpc_ctxtags.Extract(ctx)
	tags.Set("auth.mode", "kubernetes")

	token, err := grpc_auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		return nil, err
	}

	// Authenticate the user.

	res, ok, err := a.authn.AuthenticateToken(ctx, token)
	if err != nil {
		a.log.Errorw("authentication failed with an error", zap.Error(err))
		return nil, err
	}
	if !ok {
		a.log.Warn("authentication failed")
		return nil, status.Error(codes.Unauthenticated, "Request authentication failure")
	}
	a.log.Debugw("authenticated user", zap.String("name", res.User.GetName()), zap.String("uid", res.User.GetUID()))
	tags.Set("user.id", res.User.GetUID())
	tags.Set("user.name", res.User.GetName())

	// Authorize the user to use the workspace RPC endpoint.

	attributes := authorizer.AttributesRecord{
		User:            res.User,
		Namespace:       a.workspaceName.Namespace,
		Name:            a.workspaceName.Name,
		ResourceRequest: true,
		APIGroup:        "auto.pulumi.com",
		APIVersion:      "v1alpha1",
		Resource:        "workspaces",
		Subresource:     "rpc",
		Verb:            "use",
	}
	authorized, reason, err := a.authz.Authorize(ctx, attributes)
	if err != nil {
		a.log.Errorw("authorization failed with an error", zap.Error(err), "user", attributes.User.GetName())
		return nil, err
	}

	if authorized != authorizer.DecisionAllow {
		if reason == "" {
			reason = "RBAC: forbidden"
		}
		a.log.Warnw("authorization denied", zap.Error(err), zap.String("user", attributes.User.GetName()), zap.String("reason", reason))
		return nil, status.Error(codes.PermissionDenied, reason)
	}
	a.log.Debugw("authorization allowed", zap.String("reason", reason))

	return context.WithValue(ctx, "k8s.user", res.User), nil
}

// RecordRequestTotal increments the total number of requests for the delegated authentication.
func RecordRequestTotal(ctx context.Context, code string) {
}

// RecordRequestLatency measures request latency in seconds for the delegated authentication. Broken down by status code.
func RecordRequestLatency(ctx context.Context, code string, latency float64) {
}
