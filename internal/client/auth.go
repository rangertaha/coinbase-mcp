// SPDX-License-Identifier: MIT

package client

import "net/http"

// APIKeyHeaderAuthorizer authenticates by sending an API key in a fixed request
// header (e.g. "X-Api-Key: <key>"), the scheme used by most coinbase providers.
type APIKeyHeaderAuthorizer struct {
	header string
	key    string
}

// NewAPIKeyHeaderAuthorizer builds an authorizer that sets header to key.
func NewAPIKeyHeaderAuthorizer(header, key string) *APIKeyHeaderAuthorizer {
	return &APIKeyHeaderAuthorizer{header: header, key: key}
}

// Authorize sets the configured API-key header.
func (a *APIKeyHeaderAuthorizer) Authorize(r *http.Request) {
	r.Header.Set(a.header, a.key)
}

// BearerAuthorizer authenticates using an OAuth-style bearer token.
type BearerAuthorizer struct {
	header string
}

// NewBearerAuthorizer builds a BearerAuthorizer for the given token.
func NewBearerAuthorizer(token string) *BearerAuthorizer {
	return &BearerAuthorizer{header: "Bearer " + token}
}

// Authorize sets the Authorization header for bearer auth.
func (a *BearerAuthorizer) Authorize(r *http.Request) {
	r.Header.Set("Authorization", a.header)
}
