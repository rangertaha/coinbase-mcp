// SPDX-License-Identifier: MIT

package keys

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the keys toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "keys_permissions",
		Title:       "Get API key permissions",
		Description: "Get the permissions of the Coinbase API key in use (view, trade, transfer) and its portfolio scope.",
	}, svc.permissions)
}

// --- Tool input types (schemas are inferred from these structs) ---

// PermissionsInput has no fields; the key permissions take no parameters.
type PermissionsInput struct{}

// --- Tool handlers ---

func (s *service) permissions(ctx context.Context, _ *mcp.CallToolRequest, _ PermissionsInput) (*mcp.CallToolResult, *Permissions, error) {
	out, err := s.GetPermissions(ctx)
	return nil, out, err
}
