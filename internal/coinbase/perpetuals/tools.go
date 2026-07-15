// SPDX-License-Identifier: MIT

package perpetuals

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the perpetuals toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "perpetuals_portfolio",
		Title:       "Get perpetuals portfolio",
		Description: "Get the INTX perpetuals portfolio summary: collateral, margin, and liquidation state.",
	}, svc.portfolio)

	server.Register(s, server.ToolDef{
		Name:        "perpetuals_positions",
		Title:       "List perpetuals positions",
		Description: "List all open INTX perpetuals positions in a portfolio.",
	}, svc.positions)

	server.Register(s, server.ToolDef{
		Name:        "perpetuals_position",
		Title:       "Get perpetuals position",
		Description: "Get the open INTX perpetuals position for a single symbol.",
	}, svc.position)

	server.Register(s, server.ToolDef{
		Name:        "perpetuals_balances",
		Title:       "Get perpetuals balances",
		Description: "Get the asset balances of an INTX perpetuals portfolio.",
	}, svc.balances)

	server.Register(s, server.ToolDef{
		Name:        "perpetuals_allocate",
		Title:       "Allocate perpetuals collateral",
		Description: "Allocate collateral from an INTX portfolio to an isolated perpetuals position. Moves REAL collateral.",
		Write:       true,
	}, svc.allocate)

	server.Register(s, server.ToolDef{
		Name:        "perpetuals_multi_asset_collateral",
		Title:       "Set multi-asset collateral",
		Description: "Enable or disable multi-asset collateral for an INTX perpetuals portfolio.",
		Write:       true,
	}, svc.multiAssetCollateral)
}

// --- Tool input types (schemas are inferred from these structs) ---

// PortfolioInput identifies a perpetuals portfolio.
type PortfolioInput struct {
	PortfolioID string `json:"portfolioId" jsonschema:"perpetuals portfolio UUID"`
}

// PositionsInput identifies the portfolio whose positions to list.
type PositionsInput struct {
	PortfolioID string `json:"portfolioId" jsonschema:"perpetuals portfolio UUID"`
}

// PositionInput identifies a single position by portfolio and symbol.
type PositionInput struct {
	PortfolioID string `json:"portfolioId" jsonschema:"perpetuals portfolio UUID"`
	Symbol      string `json:"symbol" jsonschema:"perpetuals symbol, e.g. BTC-PERP-INTX"`
}

// BalancesInput identifies the portfolio whose balances to fetch.
type BalancesInput struct {
	PortfolioID string `json:"portfolioId" jsonschema:"perpetuals portfolio UUID"`
}

// AllocateInput describes a collateral allocation.
type AllocateInput struct {
	PortfolioID string `json:"portfolioId" jsonschema:"perpetuals portfolio UUID"`
	Symbol      string `json:"symbol" jsonschema:"perpetuals symbol, e.g. BTC-PERP-INTX"`
	Amount      string `json:"amount" jsonschema:"amount to allocate as a decimal string, e.g. 100.00"`
	Currency    string `json:"currency" jsonschema:"currency of the amount, e.g. USDC"`
}

// MultiAssetCollateralInput toggles multi-asset collateral.
type MultiAssetCollateralInput struct {
	PortfolioID string `json:"portfolioId" jsonschema:"perpetuals portfolio UUID"`
	Enabled     bool   `json:"enabled" jsonschema:"true to enable multi-asset collateral, false to disable"`
}

// --- Tool handlers ---

func (s *service) portfolio(ctx context.Context, _ *mcp.CallToolRequest, in PortfolioInput) (*mcp.CallToolResult, *Portfolio, error) {
	out, err := s.GetPortfolio(ctx, in.PortfolioID)
	return nil, out, err
}

func (s *service) positions(ctx context.Context, _ *mcp.CallToolRequest, in PositionsInput) (*mcp.CallToolResult, server.ListResult[Position], error) {
	out, err := s.ListPositions(ctx, in.PortfolioID)
	return nil, server.List(out), err
}

func (s *service) position(ctx context.Context, _ *mcp.CallToolRequest, in PositionInput) (*mcp.CallToolResult, *Position, error) {
	out, err := s.GetPosition(ctx, in.PortfolioID, in.Symbol)
	return nil, out, err
}

func (s *service) balances(ctx context.Context, _ *mcp.CallToolRequest, in BalancesInput) (*mcp.CallToolResult, *PortfolioBalances, error) {
	out, err := s.GetBalances(ctx, in.PortfolioID)
	return nil, out, err
}

func (s *service) allocate(ctx context.Context, _ *mcp.CallToolRequest, in AllocateInput) (*mcp.CallToolResult, *Allocated, error) {
	out, err := s.Allocate(ctx, in.PortfolioID, in.Symbol, in.Amount, in.Currency)
	return nil, out, err
}

func (s *service) multiAssetCollateral(ctx context.Context, _ *mcp.CallToolRequest, in MultiAssetCollateralInput) (*mcp.CallToolResult, *MultiAssetCollateral, error) {
	out, err := s.SetMultiAssetCollateral(ctx, in.PortfolioID, in.Enabled)
	return nil, out, err
}
