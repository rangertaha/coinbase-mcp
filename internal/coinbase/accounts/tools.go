// SPDX-License-Identifier: MIT

package accounts

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the accounts toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "accounts_list",
		Title:       "List accounts",
		Description: "List the authenticated user's Coinbase trading accounts (wallets) with available and held balances. Paginated: pass the returned cursor to fetch the next page.",
	}, svc.list)

	server.Register(s, server.ToolDef{
		Name:        "accounts_get",
		Title:       "Get account",
		Description: "Get a single Coinbase trading account (wallet) by its UUID.",
	}, svc.get)
}

// --- Tool input types (schemas are inferred from these structs) ---

// ListInput pages through the account list.
type ListInput struct {
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of accounts per page (optional)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"pagination cursor from a previous accounts_list response (optional)"`
}

// GetInput identifies a single account.
type GetInput struct {
	AccountID string `json:"accountId" jsonschema:"account UUID"`
}

// --- Tool handlers ---

func (s *service) list(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, *AccountsPage, error) {
	out, err := s.ListAccounts(ctx, in.Limit, in.Cursor)
	return nil, out, err
}

func (s *service) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, *Account, error) {
	out, err := s.GetAccount(ctx, in.AccountID)
	return nil, out, err
}
