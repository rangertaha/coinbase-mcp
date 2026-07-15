// SPDX-License-Identifier: MIT

package portfolios

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the portfolios toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "portfolios_list",
		Title:       "List portfolios",
		Description: "List the user's Coinbase portfolios, optionally filtered by type.",
	}, svc.list)

	server.Register(s, server.ToolDef{
		Name:        "portfolios_create",
		Title:       "Create portfolio",
		Description: "Create a new Coinbase portfolio with the given name.",
		Write:       true,
	}, svc.create)

	server.Register(s, server.ToolDef{
		Name:        "portfolios_get",
		Title:       "Get portfolio breakdown",
		Description: "Get a portfolio's breakdown: aggregate balances and spot positions.",
	}, svc.get)

	server.Register(s, server.ToolDef{
		Name:        "portfolios_edit",
		Title:       "Edit portfolio",
		Description: "Rename an existing Coinbase portfolio.",
		Write:       true,
	}, svc.edit)

	server.Register(s, server.ToolDef{
		Name:        "portfolios_delete",
		Title:       "Delete portfolio",
		Description: "Delete a Coinbase portfolio by UUID. This cannot be undone.",
		Write:       true,
		Destructive: true,
	}, svc.delete)

	server.Register(s, server.ToolDef{
		Name:        "portfolios_move_funds",
		Title:       "Move funds",
		Description: "Move an amount of a currency from one portfolio to another. Moves REAL funds.",
		Write:       true,
	}, svc.moveFunds)
}

// --- Tool input types (schemas are inferred from these structs) ---

// ListInput filters the portfolio list.
type ListInput struct {
	Type string `json:"type,omitempty" jsonschema:"filter by portfolio type: DEFAULT CONSUMER INTX or UNDEFINED (optional)"`
}

// CreateInput names the portfolio to create.
type CreateInput struct {
	Name string `json:"name" jsonschema:"name for the new portfolio"`
}

// GetInput identifies a single portfolio.
type GetInput struct {
	PortfolioID string `json:"portfolioId" jsonschema:"portfolio UUID"`
}

// EditInput identifies a portfolio and its new name.
type EditInput struct {
	PortfolioID string `json:"portfolioId" jsonschema:"portfolio UUID"`
	Name        string `json:"name" jsonschema:"new name for the portfolio"`
}

// DeleteInput identifies the portfolio to delete.
type DeleteInput struct {
	PortfolioID string `json:"portfolioId" jsonschema:"UUID of the portfolio to delete"`
}

// MoveFundsInput describes a funds transfer between two portfolios.
type MoveFundsInput struct {
	Value             string `json:"value" jsonschema:"amount to move as a decimal string, e.g. 100.50"`
	Currency          string `json:"currency" jsonschema:"currency of the amount, e.g. USD"`
	SourcePortfolioID string `json:"sourcePortfolioId" jsonschema:"UUID of the portfolio to move funds from"`
	TargetPortfolioID string `json:"targetPortfolioId" jsonschema:"UUID of the portfolio to move funds to"`
}

// --- Tool handlers ---

func (s *service) list(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, server.ListResult[Portfolio], error) {
	out, err := s.ListPortfolios(ctx, in.Type)
	return nil, server.List(out), err
}

func (s *service) create(ctx context.Context, _ *mcp.CallToolRequest, in CreateInput) (*mcp.CallToolResult, *Portfolio, error) {
	out, err := s.CreatePortfolio(ctx, in.Name)
	return nil, out, err
}

func (s *service) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, *Breakdown, error) {
	out, err := s.GetPortfolio(ctx, in.PortfolioID)
	return nil, out, err
}

func (s *service) edit(ctx context.Context, _ *mcp.CallToolRequest, in EditInput) (*mcp.CallToolResult, *Portfolio, error) {
	out, err := s.EditPortfolio(ctx, in.PortfolioID, in.Name)
	return nil, out, err
}

func (s *service) delete(ctx context.Context, _ *mcp.CallToolRequest, in DeleteInput) (*mcp.CallToolResult, *DeleteResult, error) {
	out, err := s.DeletePortfolio(ctx, in.PortfolioID)
	return nil, out, err
}

func (s *service) moveFunds(ctx context.Context, _ *mcp.CallToolRequest, in MoveFundsInput) (*mcp.CallToolResult, *MoveFundsResult, error) {
	out, err := s.MoveFunds(ctx, in.Value, in.Currency, in.SourcePortfolioID, in.TargetPortfolioID)
	return nil, out, err
}
