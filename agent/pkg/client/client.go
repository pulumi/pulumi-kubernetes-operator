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

	"golang.org/x/oauth2"
	"google.golang.org/grpc/credentials"
	"k8s.io/client-go/transport"
)

// NewTokenCredentials adds the provided bearer token to a request.
// If tokenFile is non-empty, it is periodically read,
// and the last successfully read content is used as the bearer token.
// If tokenFile is non-empty and bearer is empty, the tokenFile is read
// immediately to populate the initial bearer token.
func NewTokenCredentials(bearer string, tokenFile string) (*TokenCredentials, error) {
	if len(tokenFile) == 0 {
		return &TokenCredentials{bearer, nil}, nil
	}
	source := transport.NewCachedFileTokenSource(tokenFile)
	if len(bearer) == 0 {
		token, err := source.Token()
		if err != nil {
			return nil, err
		}
		bearer = token.AccessToken
	}
	return &TokenCredentials{bearer, source}, nil
}

type TokenCredentials struct {
	bearer string
	source oauth2.TokenSource
}

// GetRequestMetadata gets the current request metadata, refreshing tokens
// if required. This should be called by the transport layer on each
// request, and the data should be populated in headers or other
// context. If a status code is returned, it will be used as the status for
// the RPC (restricted to an allowable set of codes as defined by gRFC
// A54). uri is the URI of the entry point for the request.  When supported
// by the underlying implementation, ctx can be used for timeout and
// cancellation. Additionally, RequestInfo data will be available via ctx
// to this call.
func (k *TokenCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	token := k.bearer
	if k.source != nil {
		if refreshedToken, err := k.source.Token(); err == nil {
			token = refreshedToken.AccessToken
		}
	}
	return map[string]string{"authorization": "Bearer " + token}, nil
}

func (k *TokenCredentials) RequireTransportSecurity() bool {
	return false
}

var _ credentials.PerRPCCredentials = &TokenCredentials{}
