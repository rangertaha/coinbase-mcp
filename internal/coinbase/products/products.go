// SPDX-License-Identifier: MIT

// Package products exposes Coinbase Advanced Trade public market data: the list
// of tradeable products and per-product details.
package products

import (
	"context"
	"fmt"
	"net/url"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "products"

// service wraps the Coinbase clients for product operations.
type service struct {
	c *coinbase.Clients
}

// Product is a tradeable market, trimmed to the fields useful to an LLM.
type Product struct {
	ProductID       string `json:"product_id"`
	Price           string `json:"price,omitempty"`
	PricePctChg24   string `json:"price_percentage_change_24h,omitempty"`
	Volume24h       string `json:"volume_24h,omitempty"`
	BaseName        string `json:"base_name,omitempty"`
	QuoteName       string `json:"quote_name,omitempty"`
	Status          string `json:"status,omitempty"`
	TradingDisabled bool   `json:"trading_disabled,omitempty"`
}

// listResponse is the envelope returned by the products list endpoint.
type listResponse struct {
	Products    []Product `json:"products"`
	NumProducts int       `json:"num_products"`
}

// ListProducts returns tradeable products, optionally limited and filtered by
// product type (e.g. "SPOT").
func (s *service) ListProducts(ctx context.Context, productType string, limit int) ([]Product, error) {
	q := url.Values{}
	if productType != "" {
		q.Set("product_type", productType)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	var out listResponse
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/market/products", q, &out); err != nil {
		return nil, err
	}
	return out.Products, nil
}

// GetProduct returns a single product by ID (e.g. "BTC-USD").
func (s *service) GetProduct(ctx context.Context, productID string) (*Product, error) {
	var p Product
	path := fmt.Sprintf("/api/v3/brokerage/market/products/%s", url.PathEscape(productID))
	if err := s.c.API.GetJSON(ctx, path, nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
