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
	"errors"
	"testing"

	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"go.uber.org/zap"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/utils/ptr"
)

func TestKubernetes(t *testing.T) {
	t.Parallel()
	log := zap.L().Named("TestKubernetes").Sugar()

	unavailable := func() authenticator.TokenFunc {
		return func(ctx context.Context, token string) (*authenticator.Response, bool, error) {
			return nil, false, apierrors.NewForbidden(schema.GroupResource{}, "", errors.New("testing"))
		}
	}

	authenticate := func(knownToken string, knownUser user.DefaultInfo) authenticator.TokenFunc {
		return func(ctx context.Context, token string) (*authenticator.Response, bool, error) {
			switch token {
			case "":
				return nil, false, errors.New("Request unauthenticated with Bearer")
			case "b4d":
				return nil, false, errors.New("invalid bearer token")
			case knownToken:
				return &authenticator.Response{
					User: &knownUser,
				}, true, nil
			default:
				return nil, false, nil
			}
		}
	}

	authorize := func(knownUser user.DefaultInfo, knownWorkspace types.NamespacedName) authorizer.AuthorizerFunc {
		return func(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
			if !(a.GetUser() != nil && a.GetUser().GetName() == knownUser.Name) {
				return authorizer.DecisionDeny, "", nil
			}
			if !(a.GetVerb() == "use" && a.GetResource() == "workspaces" && a.GetSubresource() == "rpc") {
				return authorizer.DecisionDeny, "", nil
			}
			if !(a.GetNamespace() == knownWorkspace.Namespace && a.GetName() == knownWorkspace.Name) {
				return authorizer.DecisionDeny, "", nil
			}
			return authorizer.DecisionAllow, "", nil
		}
	}

	testWorkspace := types.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}
	otherWorkspace := types.NamespacedName{
		Namespace: "default",
		Name:      "other",
	}
	testUser := user.DefaultInfo{
		Name: "system:serviceaccount:default:test",
		UID:  "81be050c-9ad4-4708-9a52-413064700747",
	}
	otherUser := user.DefaultInfo{
		Name: "system:serviceaccount:default:other",
		UID:  "71be050c-9ad4-4708-9a52-413064700747",
	}

	tests := []struct {
		name            string
		resourceName    types.NamespacedName
		authHeaderValue *string
		authn           authenticator.TokenFunc
		authz           authorizer.AuthorizerFunc
		wantStatus      *grpc_status.Status
		wantTags        gstruct.Keys
	}{
		{
			name:            "Unavailable (AuthN)",
			resourceName:    testWorkspace,
			authHeaderValue: ptr.To("Bearer g00d"),
			authn:           unavailable(),
			wantStatus:      grpc_status.New(grpc_codes.Unauthenticated, "TokenReview API is unavailable"),
			wantTags: gstruct.Keys{
				"auth.mode": gomega.Equal("kubernetes"),
			},
		},
		{
			name:         "NoToken",
			resourceName: testWorkspace,
			wantStatus:   grpc_status.New(grpc_codes.Unauthenticated, "Request unauthenticated with Bearer"),
			wantTags: gstruct.Keys{
				"auth.mode": gomega.Equal("kubernetes"),
			},
		},
		{
			name:            "NotBearerToken",
			resourceName:    testWorkspace,
			authHeaderValue: ptr.To("basic dXNlcm5hbWU6cGFzc3dvcmQ="),
			wantStatus:      grpc_status.New(grpc_codes.Unauthenticated, "Request unauthenticated with Bearer"),
			wantTags: gstruct.Keys{
				"auth.mode": gomega.Equal("kubernetes"),
			},
		},
		{
			name:            "InvalidToken",
			resourceName:    testWorkspace,
			authHeaderValue: ptr.To("Bearer b4d"),
			authn:           authenticate("g00d", testUser),
			wantStatus:      grpc_status.New(grpc_codes.Unauthenticated, "invalid bearer token"),
			wantTags: gstruct.Keys{
				"auth.mode": gomega.Equal("kubernetes"),
			},
		},
		{
			name:            "AuthenticationFailure",
			resourceName:    testWorkspace,
			authHeaderValue: ptr.To("Bearer 0ther"),
			authn:           authenticate("g00d", testUser),
			wantStatus:      grpc_status.New(grpc_codes.Unauthenticated, "unauthenticated"),
			wantTags: gstruct.Keys{
				"auth.mode": gomega.Equal("kubernetes"),
			},
		},
		{
			name:            "Denied_User",
			resourceName:    testWorkspace,
			authHeaderValue: ptr.To("Bearer g00d"),
			authn:           authenticate("g00d", testUser),
			authz:           authorize(otherUser, testWorkspace),
			wantStatus:      grpc_status.New(grpc_codes.PermissionDenied, "forbidden"),
			wantTags: gstruct.Keys{
				"auth.mode": gomega.Equal("kubernetes"),
				"user.id":   gomega.Equal(testUser.UID),
				"user.name": gomega.Equal(testUser.Name),
			},
		},
		{
			name:            "Denied_ResourceName",
			resourceName:    testWorkspace,
			authHeaderValue: ptr.To("Bearer g00d"),
			authn:           authenticate("g00d", testUser),
			authz:           authorize(testUser, otherWorkspace),
			wantStatus:      grpc_status.New(grpc_codes.PermissionDenied, "forbidden"),
			wantTags: gstruct.Keys{
				"auth.mode": gomega.Equal("kubernetes"),
				"user.id":   gomega.Equal(testUser.UID),
				"user.name": gomega.Equal(testUser.Name),
			},
		},
		{
			name:            "Allowed",
			resourceName:    testWorkspace,
			authHeaderValue: ptr.To("Bearer g00d"),
			authn:           authenticate("g00d", testUser),
			authz:           authorize(testUser, testWorkspace),
			wantTags: gstruct.Keys{
				"auth.mode": gomega.Equal("kubernetes"),
				"user.id":   gomega.Equal(testUser.UID),
				"user.name": gomega.Equal(testUser.Name),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			log := log.Named(t.Name())

			if tt.authn == nil {
				tt.authn = func(ctx context.Context, token string) (*authenticator.Response, bool, error) {
					t.Error("unexpected call to AuthenticateToken")
					return nil, false, nil
				}
			}
			if tt.authz == nil {
				tt.authz = func(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
					t.Error("unexpected call to Authorize")
					return authorizer.DecisionNoOpinion, "", nil
				}
			}

			kubeAuth := &kubeAuth{
				log:           log,
				authn:         tt.authn,
				authz:         tt.authz,
				workspaceName: tt.resourceName,
			}

			// prepare the context
			md := metautils.NiceMD{}
			if tt.authHeaderValue != nil {
				md.Add("authorization", *tt.authHeaderValue)
			}
			tags := grpc_ctxtags.NewTags()
			ctx := md.ToIncoming(context.Background())
			ctx = grpc_ctxtags.SetInContext(ctx, tags)

			// execute the auth function
			ctx, err := kubeAuth.Authenticate(ctx)

			// validate the tags, some of which are set even if the function fails
			if tt.wantTags != nil {
				g.Expect(tags.Values()).To(gstruct.MatchAllKeys(tt.wantTags))
			}

			// validate the resultant status
			status := grpc_status.Convert(err)
			if tt.wantStatus != nil {
				g.Expect(status).To(gomega.HaveValue(gomega.Equal(*tt.wantStatus)), "an unexpected status")
			}
			if err == nil {
				g.Expect(ctx.Value("k8s.user")).ToNot(gomega.BeNil())
			}
		})
	}
}
