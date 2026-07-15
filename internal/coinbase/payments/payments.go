// SPDX-License-Identifier: MIT

// Package payments exposes the authenticated user's Coinbase payment methods:
// the list of linked funding sources and per-method details.
package payments

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "payments"

// service wraps the Coinbase clients for payment-method operations.
type service struct {
	c *coinbase.Clients
}

// PaymentMethod is a linked funding source (e.g. an ACH bank account), trimmed
// to the fields useful to an LLM.
type PaymentMethod struct {
	ID            string `json:"id"`
	Type          string `json:"type,omitempty"`
	Name          string `json:"name,omitempty"`
	Currency      string `json:"currency,omitempty"`
	Verified      bool   `json:"verified"`
	AllowBuy      bool   `json:"allow_buy"`
	AllowSell     bool   `json:"allow_sell"`
	AllowDeposit  bool   `json:"allow_deposit"`
	AllowWithdraw bool   `json:"allow_withdraw"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// listResponse is the envelope returned by the payment-methods list endpoint.
type listResponse struct {
	PaymentMethods []PaymentMethod `json:"payment_methods"`
}

// ListPaymentMethods returns the user's linked payment methods.
func (s *service) ListPaymentMethods(ctx context.Context) ([]PaymentMethod, error) {
	var out listResponse
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/payment_methods", nil, &out); err != nil {
		return nil, err
	}
	return out.PaymentMethods, nil
}

// GetPaymentMethod returns a single payment method by its ID.
func (s *service) GetPaymentMethod(ctx context.Context, paymentMethodID string) (*PaymentMethod, error) {
	paymentMethodID = strings.TrimSpace(paymentMethodID)
	if paymentMethodID == "" {
		// Without an ID the request would hit the list endpoint and silently
		// decode its envelope into an empty PaymentMethod.
		return nil, errors.New("paymentMethodId is required")
	}
	var out struct {
		PaymentMethod PaymentMethod `json:"payment_method"`
	}
	path := fmt.Sprintf("/api/v3/brokerage/payment_methods/%s", url.PathEscape(paymentMethodID))
	if err := s.c.API.GetJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out.PaymentMethod, nil
}
