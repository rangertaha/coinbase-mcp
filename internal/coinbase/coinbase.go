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
}

// NewClients builds the Coinbase API client for the given base URL.
//
// Public market-data endpoints (used by the read-only products toolset) need no
// authentication, so apiKey/apiSecret are optional. When supplied, they enable
// authenticated account/order endpoints; Coinbase's current scheme signs each
// request with a per-request ES256 JWT built from the CDP key — wire that into
// a client.Authorizer here when adding those toolsets (TODO).
func NewClients(baseURL, apiKey, apiSecret string, opts ...client.Option) (*Clients, error) {
	var auth client.Authorizer // nil => unauthenticated (public market data)
	if apiKey != "" && apiSecret != "" {
		// TODO: replace with a CDP JWT authorizer for authenticated endpoints.
		auth = client.NewBearerAuthorizer(apiKey)
	}
	base := append([]client.Option{client.WithUserAgent("coinbase-mcp")}, opts...)

	api, err := client.New(baseURL, auth, base...)
	if err != nil {
		return nil, fmt.Errorf("creating coinbase client: %w", err)
	}
	return &Clients{API: api}, nil
}
