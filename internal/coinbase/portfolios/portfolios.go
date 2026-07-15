// SPDX-License-Identifier: MIT

// Package portfolios exposes Coinbase Advanced Trade portfolio management:
// listing, creating, inspecting, renaming, and deleting portfolios, plus
// moving funds between them.
package portfolios

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "portfolios"

// service wraps the Coinbase clients for portfolio operations.
type service struct {
	c *coinbase.Clients
}

// Amount is a money value paired with its currency (e.g. {"1000.50","USD"}).
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// Portfolio is a Coinbase portfolio, trimmed to the fields useful to an LLM.
type Portfolio struct {
	Name    string `json:"name"`
	UUID    string `json:"uuid"`
	Type    string `json:"type"`
	Deleted bool   `json:"deleted"`
}

// listResponse is the envelope returned by the portfolios list endpoint.
type listResponse struct {
	Portfolios []Portfolio `json:"portfolios"`
}

// portfolioResponse is the envelope returned by the create and edit endpoints.
type portfolioResponse struct {
	Portfolio Portfolio `json:"portfolio"`
}

// ListPortfolios returns the user's portfolios, optionally filtered by type
// (DEFAULT, CONSUMER, INTX, or UNDEFINED).
func (s *service) ListPortfolios(ctx context.Context, portfolioType string) ([]Portfolio, error) {
	q := url.Values{}
	if portfolioType = strings.TrimSpace(portfolioType); portfolioType != "" {
		q.Set("portfolio_type", portfolioType)
	}
	var out listResponse
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/portfolios", q, &out); err != nil {
		return nil, err
	}
	return out.Portfolios, nil
}

// CreatePortfolio creates a new portfolio with the given name.
func (s *service) CreatePortfolio(ctx context.Context, name string) (*Portfolio, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	body := struct {
		Name string `json:"name"`
	}{Name: name}
	var out portfolioResponse
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/portfolios", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.Portfolio, nil
}

// Balances summarizes the portfolio's total balances across asset classes.
type Balances struct {
	TotalBalance               Amount `json:"total_balance"`
	TotalFuturesBalance        Amount `json:"total_futures_balance"`
	TotalCashEquivalentBalance Amount `json:"total_cash_equivalent_balance"`
	TotalCryptoBalance         Amount `json:"total_crypto_balance"`
	FuturesUnrealizedPNL       Amount `json:"futures_unrealized_pnl"`
	PerpUnrealizedPNL          Amount `json:"perp_unrealized_pnl"`
}

// SpotPosition is one spot asset holding inside a portfolio. The fiat/crypto
// balance fields are JSON numbers in the API response, not decimal strings.
type SpotPosition struct {
	Asset                string  `json:"asset"`
	AccountUUID          string  `json:"account_uuid"`
	TotalBalanceFiat     float64 `json:"total_balance_fiat"`
	TotalBalanceCrypto   float64 `json:"total_balance_crypto"`
	AvailableToTradeFiat float64 `json:"available_to_trade_fiat"`
	Allocation           float64 `json:"allocation"`
	CostBasis            Amount  `json:"cost_basis"`
	IsCash               bool    `json:"is_cash"`
}

// Breakdown is the detailed view of a single portfolio: identity, aggregate
// balances, and spot positions. Perpetuals and futures position arrays are
// intentionally omitted.
type Breakdown struct {
	Portfolio     Portfolio      `json:"portfolio"`
	Balances      Balances       `json:"portfolio_balances"`
	SpotPositions []SpotPosition `json:"spot_positions"`
}

// GetPortfolio returns the breakdown for a portfolio by UUID.
func (s *service) GetPortfolio(ctx context.Context, portfolioID string) (*Breakdown, error) {
	portfolioID = strings.TrimSpace(portfolioID)
	if portfolioID == "" {
		return nil, errors.New("portfolioId is required")
	}
	var out struct {
		Breakdown Breakdown `json:"breakdown"`
	}
	path := fmt.Sprintf("/api/v3/brokerage/portfolios/%s", url.PathEscape(portfolioID))
	if err := s.c.API.GetJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out.Breakdown, nil
}

// EditPortfolio renames a portfolio.
func (s *service) EditPortfolio(ctx context.Context, portfolioID, name string) (*Portfolio, error) {
	portfolioID = strings.TrimSpace(portfolioID)
	if portfolioID == "" {
		return nil, errors.New("portfolioId is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	body := struct {
		Name string `json:"name"`
	}{Name: name}
	var out portfolioResponse
	path := fmt.Sprintf("/api/v3/brokerage/portfolios/%s", url.PathEscape(portfolioID))
	if err := s.c.API.PutJSON(ctx, path, nil, body, &out); err != nil {
		return nil, err
	}
	return &out.Portfolio, nil
}

// DeleteResult confirms a portfolio deletion (the API returns an empty body).
type DeleteResult struct {
	Deleted       bool   `json:"deleted"`
	PortfolioUUID string `json:"portfolio_uuid"`
}

// DeletePortfolio deletes a portfolio by UUID.
func (s *service) DeletePortfolio(ctx context.Context, portfolioID string) (*DeleteResult, error) {
	portfolioID = strings.TrimSpace(portfolioID)
	if portfolioID == "" {
		return nil, errors.New("portfolioId is required")
	}
	path := fmt.Sprintf("/api/v3/brokerage/portfolios/%s", url.PathEscape(portfolioID))
	if err := s.c.API.Delete(ctx, path, nil, nil); err != nil {
		return nil, err
	}
	return &DeleteResult{Deleted: true, PortfolioUUID: portfolioID}, nil
}

// MoveFundsResult echoes the source and target portfolios of a funds transfer.
type MoveFundsResult struct {
	SourcePortfolioUUID string `json:"source_portfolio_uuid"`
	TargetPortfolioUUID string `json:"target_portfolio_uuid"`
}

// MoveFunds transfers an amount of a currency from one portfolio to another.
func (s *service) MoveFunds(ctx context.Context, value, currency, sourceID, targetID string) (*MoveFundsResult, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("value is required")
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return nil, errors.New("currency is required")
	}
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return nil, errors.New("sourcePortfolioId is required")
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil, errors.New("targetPortfolioId is required")
	}
	body := struct {
		Funds               Amount `json:"funds"`
		SourcePortfolioUUID string `json:"source_portfolio_uuid"`
		TargetPortfolioUUID string `json:"target_portfolio_uuid"`
	}{
		Funds:               Amount{Value: value, Currency: currency},
		SourcePortfolioUUID: sourceID,
		TargetPortfolioUUID: targetID,
	}
	var out MoveFundsResult
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/portfolios/move_funds", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
