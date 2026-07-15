// SPDX-License-Identifier: MIT

// Package accounts exposes the authenticated user's Coinbase Advanced Trade
// accounts (wallets): the paginated account list and per-account details.
package accounts

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "accounts"

// service wraps the Coinbase clients for account operations.
type service struct {
	c *coinbase.Clients
}

// Amount is a monetary value paired with its currency. The value is a decimal
// string, never a JSON number.
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// Account is a trading account (wallet), trimmed to the fields useful to an
// LLM.
type Account struct {
	UUID              string `json:"uuid"`
	Name              string `json:"name,omitempty"`
	Currency          string `json:"currency,omitempty"`
	AvailableBalance  Amount `json:"available_balance"`
	Default           bool   `json:"default"`
	Active            bool   `json:"active"`
	CreatedAt         string `json:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
	Type              string `json:"type,omitempty"`
	Ready             bool   `json:"ready"`
	Hold              Amount `json:"hold"`
	RetailPortfolioID string `json:"retail_portfolio_id,omitempty"`
}

// AccountsPage is one page of accounts plus the pagination state needed to
// fetch the next page.
type AccountsPage struct {
	Accounts []Account `json:"accounts" jsonschema:"the accounts on this page"`
	HasNext  bool      `json:"has_next" jsonschema:"whether another page exists"`
	Cursor   string    `json:"cursor,omitempty" jsonschema:"cursor to pass to the next accounts_list call"`
	Size     int       `json:"size" jsonschema:"number of accounts on this page"`
}

// ListAccounts returns a page of the authenticated user's accounts. limit caps
// the page size (optional); cursor resumes a previous listing (optional).
func (s *service) ListAccounts(ctx context.Context, limit int, cursor string) (*AccountsPage, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if cursor = strings.TrimSpace(cursor); cursor != "" {
		q.Set("cursor", cursor)
	}
	var out AccountsPage
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/accounts", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAccount returns a single account by its UUID.
func (s *service) GetAccount(ctx context.Context, accountID string) (*Account, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		// Without a UUID the request would hit the list endpoint and silently
		// decode its envelope into an empty Account.
		return nil, errors.New("accountId is required")
	}
	var out struct {
		Account Account `json:"account"`
	}
	path := fmt.Sprintf("/api/v3/brokerage/accounts/%s", url.PathEscape(accountID))
	if err := s.c.API.GetJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out.Account, nil
}
