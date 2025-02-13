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

package client

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/onsi/gomega"

	"golang.org/x/oauth2"
	authenticationv1 "k8s.io/api/authentication/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockServiceAccountInterface struct {
	token      string
	expiration time.Time
	err        error

	calls              int
	serviceAccountName string
	audiences          []string
}

var _ serviceAccountInterface = &mockServiceAccountInterface{}

func (m *mockServiceAccountInterface) CreateToken(ctx context.Context, serviceAccountName string, tokenRequest *authenticationv1.TokenRequest, opts metav1.CreateOptions) (*authenticationv1.TokenRequest, error) {
	m.calls++
	m.serviceAccountName = serviceAccountName
	m.audiences = tokenRequest.Spec.Audiences
	if m.err != nil {
		return nil, m.err
	}
	tokenRequest = tokenRequest.DeepCopy()
	tokenRequest.Status = authenticationv1.TokenRequestStatus{
		Token:               m.token,
		ExpirationTimestamp: metav1.NewTime(m.expiration),
	}
	return tokenRequest, nil
}

func TestServiceAccount_TokenSource(t *testing.T) {
	g := gomega.NewWithT(t)
	now := time.Now()
	sa := &ServiceAccount{
		sources:            map[string]*cacheEntry{},
		serviceAccountName: "test",
		creator: &mockServiceAccountInterface{
			err: apierrors.NewServiceUnavailable("unimplemented"),
		},
		now: func() time.Time { return now },
	}

	a1 := sa.TokenSource("a")
	g.Expect(a1).ToNot(gomega.BeNil())
	g.Expect(sa.sources).To(gomega.HaveKey("a"))
	g.Expect(sa.sources["a"].lastUsed).To(gomega.Equal(now))

	b := sa.TokenSource("b")
	g.Expect(b).ToNot(gomega.BeNil())
	g.Expect(b).ToNot(gomega.BeIdenticalTo(a1))

	now = now.Add(1 * time.Minute)
	a2 := sa.TokenSource("a")
	g.Expect(a2).To(gomega.BeIdenticalTo(a1))
	g.Expect(sa.sources["a"].lastUsed).To(gomega.Equal(now))
}

func TestServiceAccount_Prune(t *testing.T) {
	g := gomega.NewWithT(t)
	now := time.Now()
	sa := &ServiceAccount{
		sources:            map[string]*cacheEntry{},
		serviceAccountName: "test",
		creator: &mockServiceAccountInterface{
			err: apierrors.NewServiceUnavailable("unimplemented"),
		},
		now: func() time.Time { return now },
	}

	a := sa.TokenSource("a")
	now = now.Add(2 * time.Minute)
	b := sa.TokenSource("b")

	sa.Prune(now.Add(-1 * time.Minute))

	g.Expect(sa.sources).ToNot(gomega.HaveKey("a"))
	g.Expect(sa.TokenSource("a")).ToNot(gomega.BeIdenticalTo(a))
	g.Expect(sa.sources).To(gomega.HaveKey("b"))
	g.Expect(sa.TokenSource("b")).To(gomega.BeIdenticalTo(b))
}

func TestServiceAccountTokenSource(t *testing.T) {
	g := gomega.NewWithT(t)
	start := time.Now()
	tokA := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: "a",
		Expiry:      start.Add(10 * time.Minute),
	}

	tests := []struct {
		name string

		tok *oauth2.Token
		err error

		wantTok *oauth2.Token
		wantErr bool
	}{
		{
			name:    "valid token",
			tok:     tokA,
			wantTok: tokA,
		},
		{
			name:    "service account not found",
			err:     apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "serviceaccounts"}, "test"),
			wantErr: true,
		},
		{
			name:    "token resource forbidden",
			err:     apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "serviceaccounts/token"}, "test", errors.New("testing")),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			creator := &mockServiceAccountInterface{
				err: tc.err,
			}
			if tc.tok != nil {
				creator.token = tc.tok.AccessToken
				creator.expiration = tc.tok.Expiry
			}
			ts := &serviceAccountTokenSource{
				creator:            creator,
				serviceAccountName: "test",
				audience:           "audience",
			}

			got, err := ts.Token(context.Background())
			g.Expect(creator.serviceAccountName).To(gomega.Equal("test"))
			g.Expect(creator.audiences).To(gomega.ConsistOf("audience"))
			if tc.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(got).To(gomega.BeNil())
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(got).To(gomega.Equal(tc.wantTok))
			}
		})
	}
}

type testTokenSource struct {
	calls int
	tok   *oauth2.Token
	err   error
}

func (ts *testTokenSource) Token(_ context.Context) (*oauth2.Token, error) {
	ts.calls++
	return ts.tok, ts.err
}

func TestCachingTokenSource(t *testing.T) {
	start := time.Now()
	tokA := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: "a",
		Expiry:      start.Add(10 * time.Minute),
	}
	tokB := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: "b",
		Expiry:      start.Add(20 * time.Minute),
	}
	tests := []struct {
		name string

		tok   *oauth2.Token
		tsTok *oauth2.Token
		tsErr error
		wait  time.Duration

		wantTok     *oauth2.Token
		wantErr     bool
		wantTSCalls int
	}{
		{
			name:        "valid token returned from cache",
			tok:         tokA,
			wantTok:     tokA,
			wantTSCalls: 0,
		},
		{
			name:        "valid token returned from cache 1 minute before scheduled refresh",
			tok:         tokA,
			wait:        8 * time.Minute,
			wantTok:     tokA,
			wantTSCalls: 0,
		},
		{
			name:        "new token created when cache is empty",
			tsTok:       tokA,
			wantTok:     tokA,
			wantTSCalls: 1,
		},
		{
			name:        "new token created 1 minute after scheduled refresh",
			tok:         tokA,
			tsTok:       tokB,
			wait:        10 * time.Minute,
			wantTok:     tokB,
			wantTSCalls: 1,
		},
		{
			name:        "error on create token returns error",
			tsErr:       fmt.Errorf("error"),
			wantErr:     true,
			wantTSCalls: 1,
		},
		{
			name:        "error on create token returns previous token",
			tok:         tokA,
			tsErr:       fmt.Errorf("error"),
			wait:        10 * time.Minute,
			wantTok:     tokA,
			wantTSCalls: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tts := &testTokenSource{
				tok: tc.tsTok,
				err: tc.tsErr,
			}

			ts := &cachingTokenSource{
				base:   tts,
				tok:    tc.tok,
				leeway: 1 * time.Minute,
				now:    func() time.Time { return start.Add(tc.wait) },
			}

			gotTok, gotErr := ts.Token(context.Background())
			if got, want := gotTok, tc.wantTok; !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected token:\n\tgot:\t%#v\n\twant:\t%#v", got, want)
			}
			if got, want := tts.calls, tc.wantTSCalls; got != want {
				t.Errorf("unexpected number of Token() calls: got %d, want %d", got, want)
			}
			if gotErr == nil && tc.wantErr {
				t.Errorf("wanted error but got none")
			}
			if gotErr != nil && !tc.wantErr {
				t.Errorf("unexpected error: %v", gotErr)
			}
		})
	}
}

func TestTokenCredentials(t *testing.T) {
	start := time.Now()
	tokA := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: "a",
		Expiry:      start.Add(10 * time.Minute),
	}
	tokExpired := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: "expired",
		Expiry:      start.Add(-10 * time.Minute),
	}
	tests := []struct {
		name    string
		tok     *oauth2.Token
		tsTok   *oauth2.Token
		tsErr   error
		wantErr bool
		wanted  map[string]string
	}{
		{
			name:  "no cached token and a fresh token from source",
			tok:   nil,
			tsTok: tokA,
			wanted: map[string]string{
				"authorization": "Bearer a",
			},
		},
		{
			name:    "no cached token and an error from source",
			tok:     nil,
			tsErr:   errors.New("testing"),
			wantErr: true,
		},
		{
			name:  "expired token and a fresh token from source",
			tok:   tokExpired,
			tsTok: tokA,
			wanted: map[string]string{
				"authorization": "Bearer a",
			},
		},
		{
			name:    "expired token and an error from source",
			tok:     tokExpired,
			tsErr:   errors.New("testing"),
			wantErr: true,
		},
		{
			name:  "cached token",
			tok:   tokA,
			tsErr: errors.New("unexpected"),
			wanted: map[string]string{
				"authorization": "Bearer a",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			ts := &testTokenSource{
				tok: tc.tsTok,
				err: tc.tsErr,
			}
			creds := &tokenCredentials{source: ts, t: tc.tok}

			g.Expect(creds.RequireTransportSecurity()).To(gomega.BeFalse())

			got, err := creds.GetRequestMetadata(context.Background())
			if tc.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(got).To(gomega.BeNil())
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(got).To(gomega.Equal(tc.wanted))
			}
		})
	}
}
