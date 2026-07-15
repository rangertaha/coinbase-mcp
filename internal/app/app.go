// SPDX-License-Identifier: MIT

// Package app assembles the fully-configured coinbase-mcp server from
// configuration. It is shared by the command entry point (cmd/coinbase) so the
// exact server the binary runs is the one under test.
package app

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/accounts"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/convert"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/fees"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/futures"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/keys"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/orders"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/payments"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/perpetuals"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/portfolios"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/products"
	"github.com/rangertaha/coinbase-mcp/internal/config"
	"github.com/rangertaha/coinbase-mcp/internal/prompts"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// instructions is surfaced to MCP clients at initialization to guide tool
// selection.
const instructions = `Coinbase Advanced Trade tools.

- products_* tools are public market data and need no credentials; everything
  else requires the configured CDP API key.
- Write tools move real money on a real account: orders_create places live
  orders, orders_cancel/orders_close_position cancel or close live positions,
  and convert_commit executes a conversion. ALWAYS call orders_preview before
  orders_create, and prefer asking the user before any write tool.
- Monetary values are decimal strings (e.g. "0.001"), never floats.
- List tools paginate with a cursor: request small pages (limit 10-25) and pass
  the returned cursor to continue.`

// scopeCheckTimeout bounds the startup key-permissions lookup.
const scopeCheckTimeout = 5 * time.Second

// Assemble builds the fully-configured server (all enabled toolsets and
// prompts) and returns it with a cleanup function. version is reported to
// clients.
func Assemble(ctx context.Context, cfg *config.Config, version string) (*server.Server, func(), error) {
	clients, err := coinbase.NewClients(cfg.BaseURL, cfg.APIKey, cfg.APISecret)
	if err != nil {
		return nil, nil, err
	}

	authenticated := cfg.APIKey != ""
	readOnly := cfg.ReadOnly

	// Scope filtering: a view-only CDP key can never execute write tools, so
	// don't advertise them. Best-effort — on lookup failure the configured
	// policy stands.
	if authenticated && !readOnly {
		lctx, cancel := context.WithTimeout(ctx, scopeCheckTimeout)
		perms, err := keys.Lookup(lctx, clients)
		cancel()
		switch {
		case err != nil:
			log.Printf("coinbase-mcp: key permissions lookup failed (%v); keeping configured read-only=%v", err, readOnly)
		case !perms.CanTrade:
			log.Printf("coinbase-mcp: API key cannot trade (can_trade=false); hiding write tools")
			readOnly = true
		}
	}

	srv := server.New("coinbase-mcp", version, readOnly, instructions)
	srv.AllowTools(cfg.Tools)

	for _, ts := range toolsets() {
		if !cfg.ToolsetEnabled(ts.Name) {
			continue
		}
		if ts.Auth && !authenticated {
			log.Printf("coinbase-mcp: toolset %q needs %s/%s; skipping", ts.Name, config.EnvAPIKey, config.EnvAPISecret)
			continue
		}
		ts.Register(srv, clients)
	}

	// Diagnostics go to stderr; stdout is reserved for the MCP protocol.
	log.SetOutput(os.Stderr)

	prompts.Register(srv)

	return srv, func() {}, nil
}

// toolsets returns every toolset registrar, in registration order. New service
// areas are added here.
func toolsets() []server.Toolset {
	return []server.Toolset{
		{Name: products.Name, Register: products.Register},
		// Authenticated-only additions to the products toolset (same name, so
		// COINBASE_TOOLSETS=products enables both halves).
		{Name: products.Name, Register: products.RegisterAuth, Auth: true},
		{Name: accounts.Name, Register: accounts.Register, Auth: true},
		{Name: orders.Name, Register: orders.Register, Auth: true},
		{Name: portfolios.Name, Register: portfolios.Register, Auth: true},
		{Name: convert.Name, Register: convert.Register, Auth: true},
		{Name: fees.Name, Register: fees.Register, Auth: true},
		{Name: payments.Name, Register: payments.Register, Auth: true},
		{Name: futures.Name, Register: futures.Register, Auth: true},
		{Name: perpetuals.Name, Register: perpetuals.Register, Auth: true},
		{Name: keys.Name, Register: keys.Register, Auth: true},
	}
}
