// SPDX-License-Identifier: MIT

// Package app assembles the fully-configured coinbase-mcp server from
// configuration. It is shared by the command entry point (cmd/coinbase) so the
// exact server the binary runs is the one under test.
package app

import (
	"log"
	"os"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase/products"
	"github.com/rangertaha/coinbase-mcp/internal/config"
	"github.com/rangertaha/coinbase-mcp/internal/prompts"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Assemble builds the fully-configured server (all enabled toolsets and
// prompts) and returns it with a cleanup function. version is reported to
// clients.
func Assemble(cfg *config.Config, version string) (*server.Server, func(), error) {
	clients, err := coinbase.NewClients(cfg.BaseURL, cfg.APIKey, cfg.APISecret)
	if err != nil {
		return nil, nil, err
	}

	srv := server.New("coinbase-mcp", version, cfg.ReadOnly)

	for _, ts := range toolsets() {
		if cfg.ToolsetEnabled(ts.Name) {
			ts.Register(srv, clients)
		}
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
	}
}
