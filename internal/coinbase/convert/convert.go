// SPDX-License-Identifier: MIT

// Package convert exposes Coinbase Advanced Trade convert trades: quoting,
// committing, and inspecting conversions between accounts (e.g. USD to USDC).
package convert

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "convert"

// service wraps the Coinbase clients for convert operations.
type service struct {
	c *coinbase.Clients
}

// Amount is a money value paired with its currency (e.g. {"100.00","USD"}).
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// Trade is a convert trade, trimmed to the fields useful to an LLM.
type Trade struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	UserEnteredAmount Amount `json:"user_entered_amount"`
	Amount            Amount `json:"amount"`
	Subtotal          Amount `json:"subtotal"`
	Total             Amount `json:"total"`
	ExchangeRate      Amount `json:"exchange_rate"`
}

// tradeResponse is the envelope returned by every convert endpoint.
type tradeResponse struct {
	Trade Trade `json:"trade"`
}

// accountsBody is the request body carrying the source and target accounts.
type accountsBody struct {
	FromAccount string `json:"from_account"`
	ToAccount   string `json:"to_account"`
}

// CreateQuote creates a convert quote to move an amount from one account's
// currency to another (e.g. USD to USDC). The quote must be committed with
// CommitTrade before it expires.
func (s *service) CreateQuote(ctx context.Context, fromAccount, toAccount, amount string) (*Trade, error) {
	fromAccount = strings.TrimSpace(fromAccount)
	if fromAccount == "" {
		return nil, errors.New("fromAccount is required")
	}
	toAccount = strings.TrimSpace(toAccount)
	if toAccount == "" {
		return nil, errors.New("toAccount is required")
	}
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return nil, errors.New("amount is required")
	}
	body := struct {
		accountsBody
		Amount string `json:"amount"`
	}{
		accountsBody: accountsBody{FromAccount: fromAccount, ToAccount: toAccount},
		Amount:       amount,
	}
	var out tradeResponse
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/convert/quote", nil, body, &out); err != nil {
		return nil, err
	}
	return &out.Trade, nil
}

// CommitTrade commits a previously quoted convert trade, executing the
// conversion.
func (s *service) CommitTrade(ctx context.Context, tradeID, fromAccount, toAccount string) (*Trade, error) {
	tradeID = strings.TrimSpace(tradeID)
	if tradeID == "" {
		return nil, errors.New("tradeId is required")
	}
	fromAccount = strings.TrimSpace(fromAccount)
	if fromAccount == "" {
		return nil, errors.New("fromAccount is required")
	}
	toAccount = strings.TrimSpace(toAccount)
	if toAccount == "" {
		return nil, errors.New("toAccount is required")
	}
	body := accountsBody{FromAccount: fromAccount, ToAccount: toAccount}
	var out tradeResponse
	path := fmt.Sprintf("/api/v3/brokerage/convert/trade/%s", url.PathEscape(tradeID))
	if err := s.c.API.PostJSON(ctx, path, nil, body, &out); err != nil {
		return nil, err
	}
	return &out.Trade, nil
}

// GetTrade returns the status of a convert trade. The API requires the source
// and target accounts alongside the trade ID.
func (s *service) GetTrade(ctx context.Context, tradeID, fromAccount, toAccount string) (*Trade, error) {
	tradeID = strings.TrimSpace(tradeID)
	if tradeID == "" {
		return nil, errors.New("tradeId is required")
	}
	fromAccount = strings.TrimSpace(fromAccount)
	if fromAccount == "" {
		return nil, errors.New("fromAccount is required")
	}
	toAccount = strings.TrimSpace(toAccount)
	if toAccount == "" {
		return nil, errors.New("toAccount is required")
	}
	q := url.Values{}
	q.Set("from_account", fromAccount)
	q.Set("to_account", toAccount)
	var out tradeResponse
	path := fmt.Sprintf("/api/v3/brokerage/convert/trade/%s", url.PathEscape(tradeID))
	if err := s.c.API.GetJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out.Trade, nil
}
