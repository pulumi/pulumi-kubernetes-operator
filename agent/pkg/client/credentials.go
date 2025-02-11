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

package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/credentials"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// serviceAccountInterface is the necessary subset of clientcorev1.ServiceAccountInterface.
type serviceAccountInterface interface {
	CreateToken(ctx context.Context, serviceAccountName string, tokenRequest *authenticationv1.TokenRequest, opts metav1.CreateOptions) (*authenticationv1.TokenRequest, error)
}

// TokenSource is an interface for requesting a token, similar to oauth2.TokenSource but accepting a context.
type TokenSource interface {
	Token(ctx context.Context) (*oauth2.Token, error)
}

// TokenSourceFactory is an interface for creating TokenSources for a given audience.
type TokenSourceFactory interface {
	TokenSource(audience string) TokenSource
}

// ServiceAccount impersonates a service account using the Kubernetes TokenRequest API.
type ServiceAccount struct {
	creator serviceAccountInterface
	serviceAccountName string
	mu     sync.Mutex
	sources map[string]TokenSource
}

// NewServiceAccount returns a new ServiceAccount with the given name and using the given Kubernetes client.
func NewServiceAccount(client serviceAccountInterface, serviceAccountName string) *ServiceAccount {
	return &ServiceAccount{
		creator: client,
		serviceAccountName: serviceAccountName,
		sources: map[string]TokenSource{},
	}
}

var _ TokenSourceFactory = &ServiceAccount{}

func (m *ServiceAccount) TokenSource(audience string) TokenSource {
	m.mu.Lock()
	defer m.mu.Unlock()
	if src, ok := m.sources[audience]; ok {
		return src
	}
	src := &cachingTokenSource{
		base: &serviceAccountTokenSource{
			creator: m.creator,
			serviceAccountName: m.serviceAccountName,
			audience: audience,
		},
		leeway: 5 * time.Minute,
		now: time.Now,
	}
	m.sources[audience] = src
	return src
}

// serviceAccountTokenSource produces tokens representing the given service account,
// as provided by the Kubernetes TokenRequest API. 
// Resultant tokens are scoped to the given audience (i.e. a workspace) to be used as bearer tokens for that audience.
// The underlying Kubernetes client has original credentials to authenticate to the TokenRequest API.
type serviceAccountTokenSource struct {
	creator serviceAccountInterface
	serviceAccountName string
	audience string
}

var _ TokenSource = &serviceAccountTokenSource{}

func (k *serviceAccountTokenSource) Token(ctx context.Context) (*oauth2.Token, error) {
	// Create an identity token representing the service account
	// and scoped to a particular audience.
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences: []string{k.audience},
		},
	}
	tokenRequest, err := k.creator.CreateToken(ctx, k.serviceAccountName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		TokenType: "Bearer",
		AccessToken: tokenRequest.Status.Token,
		Expiry: tokenRequest.Status.ExpirationTimestamp.Time,
	}, nil
}

// cachingTokenSource implements a token cache around an underlying token source,
// refreshing the token as it nears expiration.
type cachingTokenSource struct {
	base TokenSource
	leeway time.Duration

	sync.RWMutex
	tok *oauth2.Token
	t   time.Time

	// for testing
	now func() time.Time
}

func (ts *cachingTokenSource) Token(ctx context.Context) (*oauth2.Token, error) {
	l := logr.FromContextOrDiscard(ctx)
	
	now := ts.now()
	// fast path
	ts.RLock()
	tok := ts.tok
	ts.RUnlock()

	if tok != nil && tok.Expiry.Add(-1*ts.leeway).After(now) {
		return tok, nil
	}

	// slow path
	ts.Lock()
	defer ts.Unlock()
	if tok := ts.tok; tok != nil && tok.Expiry.Add(-1*ts.leeway).After(now) {
		return tok, nil
	}

	tok, err := ts.base.Token(ctx)
	if err != nil {
		if ts.tok == nil {
			return nil, err
		}
		l.Error(err, "Unable to rotate token")
		return ts.tok, nil
	}
	l.V(1).Info("token rotated", "expiry", tok.Expiry)

	ts.t = ts.now()
	ts.tok = tok
	return tok, nil
}


func (ts *cachingTokenSource) ResetTokenOlderThan(t time.Time) {
	ts.Lock()
	defer ts.Unlock()
	if ts.t.Before(t) {
		ts.tok = nil
		ts.t = time.Time{}
	}
}

// tokenCredentials implements credentials.PerRPCCredentials to attach a bearer token to each request.
type tokenCredentials struct {
	mu     sync.Mutex
	source TokenSource
	t      *oauth2.Token
}

// NewTokenCredentials creates a new tokenCredentials instance for the given token source.
func NewTokenCredentials(source TokenSource) *tokenCredentials {
	return &tokenCredentials{source: source}
}

var _ credentials.PerRPCCredentials = &tokenCredentials{}

// GetRequestMetadata gets the current request metadata, refreshing tokens
// if required. This should be called by the transport layer on each
// request, and the data should be populated in headers or other
// context. If a status code is returned, it will be used as the status for
// the RPC (restricted to an allowable set of codes as defined by gRFC
// A54). uri is the URI of the entry point for the request.  When supported
// by the underlying implementation, ctx can be used for timeout and
// cancellation. Additionally, RequestInfo data will be available via ctx
// to this call.
func (s *tokenCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.t.Valid() {
		var err error
		s.t, err = s.source.Token(ctx)
		if err != nil {
			return nil, err
		}
	}
	ri, _ := credentials.RequestInfoFromContext(ctx)
	if err := credentials.CheckSecurityLevel(ri.AuthInfo, credentials.NoSecurity); err != nil {
		return nil, fmt.Errorf("unable to transfer serviceAccount PerRPCCredentials: %v", err)
	}
	return map[string]string{
		"authorization": s.t.Type() + " " + s.t.AccessToken,
	}, nil
}

func (s *tokenCredentials) RequireTransportSecurity() bool {
	return false
}
