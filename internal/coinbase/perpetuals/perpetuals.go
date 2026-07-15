// SPDX-License-Identifier: MIT

// Package perpetuals exposes the Coinbase Advanced Trade INTX (international
// exchange) perpetual futures endpoints: portfolio summary, positions,
// balances, collateral allocation, and multi-asset collateral opt-in.
package perpetuals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "perpetuals"

// service wraps the Coinbase clients for perpetuals operations.
type service struct {
	c *coinbase.Clients
}

// Amount is a money value paired with its currency.
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// Number is a decimal that tolerates both JSON number and JSON string
// encodings — the INTX endpoints document some ratio fields inconsistently
// between the two, and a hard type would fail the whole call.
type Number string

// UnmarshalJSON accepts a JSON number, string, or null.
func (n *Number) UnmarshalJSON(b []byte) error {
	switch {
	case len(b) == 0:
		return errors.New("empty JSON value")
	case b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*n = Number(s)
	case b[0] == '{' || b[0] == '[':
		return fmt.Errorf("cannot decode %s into a number", b)
	case string(b) == "null":
		*n = ""
	default:
		*n = Number(b)
	}
	return nil
}

// Portfolio is the INTX perpetuals portfolio summary, trimmed to the fields
// useful to an LLM. Monetary values are decimal strings; the margin ratios use
// Number because the API documents them inconsistently (number vs string).
type Portfolio struct {
	PortfolioUUID         string `json:"portfolio_uuid"`
	Collateral            string `json:"collateral,omitempty"`
	PositionNotional      string `json:"position_notional,omitempty"`
	OpenPositionNotional  string `json:"open_position_notional,omitempty"`
	PendingFees           string `json:"pending_fees,omitempty"`
	Borrow                string `json:"borrow,omitempty"`
	AccruedInterest       string `json:"accrued_interest,omitempty"`
	RollingDebt           string `json:"rolling_debt,omitempty"`
	InitialMargin         Number `json:"portfolio_initial_margin,omitempty"`
	IMNotional            Amount `json:"portfolio_im_notional"`
	MaintenanceMargin     Number `json:"portfolio_maintenance_margin,omitempty"`
	MMNotional            Amount `json:"portfolio_mm_notional"`
	LiquidationPercentage string `json:"liquidation_percentage,omitempty"`
	LiquidationBuffer     string `json:"liquidation_buffer,omitempty"`
	MarginType            string `json:"margin_type,omitempty"`
	LiquidationStatus     string `json:"liquidation_status,omitempty"`
	UnrealizedPnL         Amount `json:"unrealized_pnl"`
	TotalBalance          Amount `json:"total_balance"`
}

// GetPortfolio returns the perpetuals portfolio summary for one portfolio.
func (s *service) GetPortfolio(ctx context.Context, portfolioID string) (*Portfolio, error) {
	portfolioID = strings.TrimSpace(portfolioID)
	if portfolioID == "" {
		return nil, errors.New("portfolioId is required")
	}
	// The API returns an array under "portfolios" even for this single-
	// portfolio endpoint (confirmed against the official Python SDK).
	var out struct {
		Portfolios []Portfolio `json:"portfolios"`
	}
	path := fmt.Sprintf("/api/v3/brokerage/intx/portfolio/%s", url.PathEscape(portfolioID))
	if err := s.c.API.GetJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	if len(out.Portfolios) == 0 {
		return nil, fmt.Errorf("no perpetuals portfolio found for %q", portfolioID)
	}
	return &out.Portfolios[0], nil
}

// Position is one open perpetuals position, trimmed to the fields useful to an
// LLM. Sizes and leverage are decimal strings.
type Position struct {
	ProductID        string `json:"product_id"`
	Symbol           string `json:"symbol,omitempty"`
	PositionSide     string `json:"position_side,omitempty"`
	NetSize          string `json:"net_size,omitempty"`
	Leverage         string `json:"leverage,omitempty"`
	MarginType       string `json:"margin_type,omitempty"`
	UnrealizedPnL    Amount `json:"unrealized_pnl"`
	MarkPrice        Amount `json:"mark_price"`
	LiquidationPrice Amount `json:"liquidation_price"`
}

// ListPositions returns all open perpetuals positions in a portfolio.
func (s *service) ListPositions(ctx context.Context, portfolioID string) ([]Position, error) {
	portfolioID = strings.TrimSpace(portfolioID)
	if portfolioID == "" {
		return nil, errors.New("portfolioId is required")
	}
	var out struct {
		Positions []Position `json:"positions"`
	}
	path := fmt.Sprintf("/api/v3/brokerage/intx/positions/%s", url.PathEscape(portfolioID))
	if err := s.c.API.GetJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Positions, nil
}

// GetPosition returns the open perpetuals position for one symbol.
func (s *service) GetPosition(ctx context.Context, portfolioID, symbol string) (*Position, error) {
	portfolioID = strings.TrimSpace(portfolioID)
	if portfolioID == "" {
		return nil, errors.New("portfolioId is required")
	}
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New(`symbol is required (e.g. "BTC-PERP-INTX")`)
	}
	var out struct {
		Position Position `json:"position"`
	}
	path := fmt.Sprintf("/api/v3/brokerage/intx/positions/%s/%s", url.PathEscape(portfolioID), url.PathEscape(symbol))
	if err := s.c.API.GetJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out.Position, nil
}

// Asset identifies the asset of a balance entry.
type Asset struct {
	AssetID   string `json:"asset_id"`
	AssetName string `json:"asset_name,omitempty"`
}

// Balance is one asset balance in a perpetuals portfolio. All quantities are
// decimal strings.
type Balance struct {
	Asset             Asset  `json:"asset"`
	Quantity          string `json:"quantity,omitempty"`
	Hold              string `json:"hold,omitempty"`
	CollateralValue   string `json:"collateral_value,omitempty"`
	MaxWithdrawAmount string `json:"max_withdraw_amount,omitempty"`
	Loan              string `json:"loan,omitempty"`
}

// PortfolioBalances is the balances snapshot for a perpetuals portfolio.
type PortfolioBalances struct {
	PortfolioUUID        string    `json:"portfolio_uuid"`
	Balances             []Balance `json:"balances"`
	IsMarginLimitReached bool      `json:"is_margin_limit_reached"`
}

// GetBalances returns the asset balances of a perpetuals portfolio.
func (s *service) GetBalances(ctx context.Context, portfolioID string) (*PortfolioBalances, error) {
	portfolioID = strings.TrimSpace(portfolioID)
	if portfolioID == "" {
		return nil, errors.New("portfolioId is required")
	}
	// The API returns an array under "portfolio_balances" even for this
	// single-portfolio endpoint (confirmed against the official Python SDK).
	var out struct {
		PortfolioBalances []PortfolioBalances `json:"portfolio_balances"`
	}
	path := fmt.Sprintf("/api/v3/brokerage/intx/balances/%s", url.PathEscape(portfolioID))
	if err := s.c.API.GetJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	if len(out.PortfolioBalances) == 0 {
		return nil, fmt.Errorf("no balances found for perpetuals portfolio %q", portfolioID)
	}
	return &out.PortfolioBalances[0], nil
}

// Allocated confirms a collateral allocation to an isolated position.
type Allocated struct {
	Allocated bool   `json:"allocated"`
	Symbol    string `json:"symbol"`
	Amount    string `json:"amount"`
	Currency  string `json:"currency"`
}

// Allocate moves collateral from a portfolio to an isolated perpetuals
// position.
func (s *service) Allocate(ctx context.Context, portfolioID, symbol, amount, currency string) (*Allocated, error) {
	portfolioID = strings.TrimSpace(portfolioID)
	if portfolioID == "" {
		return nil, errors.New("portfolioId is required")
	}
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New(`symbol is required (e.g. "BTC-PERP-INTX")`)
	}
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return nil, errors.New(`amount is required (e.g. "100.00")`)
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return nil, errors.New(`currency is required (e.g. "USDC")`)
	}
	body := struct {
		PortfolioUUID string `json:"portfolio_uuid"`
		Symbol        string `json:"symbol"`
		Amount        string `json:"amount"`
		Currency      string `json:"currency"`
	}{PortfolioUUID: portfolioID, Symbol: symbol, Amount: amount, Currency: currency}
	var out struct{}
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/intx/allocate", nil, body, &out); err != nil {
		return nil, err
	}
	return &Allocated{Allocated: true, Symbol: symbol, Amount: amount, Currency: currency}, nil
}

// MultiAssetCollateral reports the multi-asset collateral opt-in state.
type MultiAssetCollateral struct {
	Enabled bool `json:"multi_asset_collateral_enabled"`
}

// SetMultiAssetCollateral toggles multi-asset collateral for a portfolio and
// returns the resulting state.
func (s *service) SetMultiAssetCollateral(ctx context.Context, portfolioID string, enabled bool) (*MultiAssetCollateral, error) {
	portfolioID = strings.TrimSpace(portfolioID)
	if portfolioID == "" {
		return nil, errors.New("portfolioId is required")
	}
	body := struct {
		PortfolioUUID string `json:"portfolio_uuid"`
		Enabled       bool   `json:"multi_asset_collateral_enabled"`
	}{PortfolioUUID: portfolioID, Enabled: enabled}
	var out MultiAssetCollateral
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/intx/multi_asset_collateral", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
