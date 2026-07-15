// SPDX-License-Identifier: MIT

// Package coinbase holds the connection to the Coinbase Advanced Trade API that
// the per-area tool packages (products, …) share.
package coinbase

import (
	"fmt"

	"github.com/rangertaha/coinbase-mcp/internal/client"
)

// Clients bundles the REST clients needed to reach the Coinbase API.
type Clients struct {
	// API reaches the Coinbase Advanced Trade REST host.
	API *client.Client
	// Authenticated reports whether requests carry CDP credentials. Toolsets
	// use it to prefer the authenticated endpoint variants (e.g.
	// /api/v3/brokerage/products over /api/v3/brokerage/market/products),
	// which respect the key's portfolio scope.
	Authenticated bool
}

// NewClients builds the Coinbase API client for the given base URL.
//
// Public market-data endpoints (used by the read-only products toolset) need no
// authentication, so apiKey/apiSecret are optional. When supplied, they enable
// the authenticated toolsets: every request is signed with a short-lived CDP
// JWT (see NewJWTAuthorizer).
func NewClients(baseURL, apiKey, apiSecret string, opts ...client.Option) (*Clients, error) {
	var auth client.Authorizer // nil => unauthenticated (public market data)
	if apiKey != "" && apiSecret != "" {
		jwt, err := NewJWTAuthorizer(apiKey, apiSecret)
		if err != nil {
			return nil, fmt.Errorf("building coinbase authorizer: %w", err)
		}
		auth = jwt
	}
	base := append([]client.Option{client.WithUserAgent("coinbase-mcp")}, opts...)

	api, err := client.New(baseURL, auth, base...)
	if err != nil {
		return nil, fmt.Errorf("creating coinbase client: %w", err)
	}
	return &Clients{API: api, Authenticated: auth != nil}, nil
}
