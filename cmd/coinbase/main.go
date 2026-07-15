// SPDX-License-Identifier: MIT

// Command coinbase runs the Coinbase Model Context Protocol server
// (`coinbase mcp`) and checks connectivity (`coinbase test`).
//
// Configuration is read from the environment (see package config). The `mcp`
// command communicates over stdio, the transport expected by MCP clients such
// as Claude Desktop/Code and Cursor.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/urfave/cli/v3"

	"github.com/rangertaha/coinbase-mcp/internal"
	"github.com/rangertaha/coinbase-mcp/internal/app"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/config"
)

func main() {
	os.Exit(run(context.Background(), os.Args, os.Stderr))
}

// run executes the CLI and returns the process exit code. It is split from
// main so both outcomes are testable.
func run(ctx context.Context, args []string, errOut io.Writer) int {
	cmd := &cli.Command{
		Name:    "coinbase",
		Usage:   "Coinbase market data as an MCP server",
		Version: internal.Version(),
		// A bare `coinbase` (no subcommand) runs the MCP server.
		Action: runMCP,
		Commands: []*cli.Command{
			mcpCommand(),
			testCommand(),
		},
		// Print errors ourselves so the MCP stdio stream is never touched.
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
	}

	if err := cmd.Run(ctx, args); err != nil {
		_, _ = fmt.Fprintf(errOut, "coinbase: %v\n", err)
		return 1
	}
	return 0
}

// mcpCommand runs the MCP server over stdio.
func mcpCommand() *cli.Command {
	return &cli.Command{
		Name:   "mcp",
		Usage:  "Run the MCP server over stdio (for Claude Desktop/Code, Cursor, ...)",
		Action: runMCP,
	}
}

// runMCP assembles and serves the MCP server over stdio.
func runMCP(ctx context.Context, _ *cli.Command) error {
	if err := config.LoadEnvFile(config.EnvFile); err != nil {
		log.Printf("coinbase: reading %s: %v", config.EnvFile, err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error:\n%w", err)
	}

	ver := internal.Version()
	srv, cleanup, err := app.Assemble(ctx, cfg, ver)
	if err != nil {
		return err
	}
	defer cleanup()

	log.Printf("coinbase-mcp %s starting: %d tools, %d prompts across toolsets %v (read-only=%v)",
		ver, srv.ToolCount(), srv.PromptCount(), srv.Toolsets(), cfg.ReadOnly)

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return srv.Run(ctx, &mcp.StdioTransport{})
}

// testCommand verifies connectivity against the Coinbase API.
func testCommand() *cli.Command {
	return &cli.Command{
		Name:  "test",
		Usage: "Test connectivity against the Coinbase API",
		Action: func(ctx context.Context, _ *cli.Command) error {
			if err := config.LoadEnvFile(config.EnvFile); err != nil {
				log.Printf("coinbase: reading %s: %v", config.EnvFile, err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("configuration error:\n%w", err)
			}

			clients, err := coinbase.NewClients(cfg.BaseURL, cfg.APIKey, cfg.APISecret)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			n, err := coinbase.Check(ctx, clients)
			if err != nil {
				return fmt.Errorf("connecting to %s: %w", cfg.BaseURL, err)
			}

			fmt.Printf("OK  connected to %s (%d products available)\n", cfg.BaseURL, n)
			fmt.Printf("    authenticated=%v read-only=%v\n", cfg.APIKey != "", cfg.ReadOnly)
			return nil
		},
	}
}
