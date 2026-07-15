// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the convert toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "convert_quote",
		Title:       "Create convert quote",
		Description: "Create a quote to convert an amount between two accounts (e.g. USD to USDC). Commit the quote with convert_commit before it expires.",
		Write:       true,
	}, svc.quote)

	server.Register(s, server.ToolDef{
		Name:        "convert_commit",
		Title:       "Commit convert trade",
		Description: "Commit a previously quoted convert trade, executing the conversion. Moves REAL funds between accounts.",
		Write:       true,
	}, svc.commit)

	server.Register(s, server.ToolDef{
		Name:        "convert_get",
		Title:       "Get convert trade",
		Description: "Get the status of a convert trade by ID.",
	}, svc.get)
}

// --- Tool input types (schemas are inferred from these structs) ---

// QuoteInput describes the conversion to quote.
type QuoteInput struct {
	FromAccount string `json:"fromAccount" jsonschema:"account ID or currency to convert from, e.g. USD"`
	ToAccount   string `json:"toAccount" jsonschema:"account ID or currency to convert to, e.g. USDC"`
	Amount      string `json:"amount" jsonschema:"amount to convert as a decimal string, in the from-account currency"`
}

// CommitInput identifies the quoted trade to execute.
type CommitInput struct {
	TradeID     string `json:"tradeId" jsonschema:"ID of the convert trade returned by convert_quote"`
	FromAccount string `json:"fromAccount" jsonschema:"account ID or currency the quote converts from"`
	ToAccount   string `json:"toAccount" jsonschema:"account ID or currency the quote converts to"`
}

// GetInput identifies the convert trade to look up.
type GetInput struct {
	TradeID     string `json:"tradeId" jsonschema:"ID of the convert trade"`
	FromAccount string `json:"fromAccount" jsonschema:"account ID or currency the trade converts from"`
	ToAccount   string `json:"toAccount" jsonschema:"account ID or currency the trade converts to"`
}

// --- Tool handlers ---

func (s *service) quote(ctx context.Context, _ *mcp.CallToolRequest, in QuoteInput) (*mcp.CallToolResult, *Trade, error) {
	out, err := s.CreateQuote(ctx, in.FromAccount, in.ToAccount, in.Amount)
	return nil, out, err
}

func (s *service) commit(ctx context.Context, _ *mcp.CallToolRequest, in CommitInput) (*mcp.CallToolResult, *Trade, error) {
	out, err := s.CommitTrade(ctx, in.TradeID, in.FromAccount, in.ToAccount)
	return nil, out, err
}

func (s *service) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, *Trade, error) {
	out, err := s.GetTrade(ctx, in.TradeID, in.FromAccount, in.ToAccount)
	return nil, out, err
}
