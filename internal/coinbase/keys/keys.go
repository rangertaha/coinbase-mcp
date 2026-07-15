// SPDX-License-Identifier: MIT

// Package keys exposes the permissions granted to the Coinbase API key the
// server is running with, so the model can tell which operations are possible.
package keys

import (
	"context"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "keys"

// service wraps the Coinbase clients for API-key operations.
type service struct {
	c *coinbase.Clients
}

// Permissions describes what the current API key may do and which portfolio it
// is scoped to.
type Permissions struct {
	CanView       bool   `json:"can_view"`
	CanTrade      bool   `json:"can_trade"`
	CanTransfer   bool   `json:"can_transfer"`
	PortfolioUUID string `json:"portfolio_uuid,omitempty"`
	PortfolioType string `json:"portfolio_type,omitempty"`
}

// GetPermissions returns the permissions of the API key in use.
func (s *service) GetPermissions(ctx context.Context) (*Permissions, error) {
	return Lookup(ctx, s.c)
}

// Lookup fetches the API key's permissions. Exposed so the entrypoint can
// scope-filter tools at startup (e.g. force read-only for a view-only key).
func Lookup(ctx context.Context, c *coinbase.Clients) (*Permissions, error) {
	var p Permissions
	if err := c.API.GetJSON(ctx, "/api/v3/brokerage/key_permissions", nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
