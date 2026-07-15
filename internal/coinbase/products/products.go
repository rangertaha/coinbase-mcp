// SPDX-License-Identifier: MIT

// Package products exposes Coinbase Advanced Trade public market data: the list
// of tradeable products and per-product details.
package products

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "products"

// service wraps the Coinbase clients for product operations.
type service struct {
	c *coinbase.Clients
}

// basePath returns the market-data path prefix. Authenticated keys use the
// /api/v3/brokerage endpoints (scoped to the key's portfolio); public keys use
// the equivalent /api/v3/brokerage/market endpoints.
func (s *service) basePath() string {
	if s.c.Authenticated {
		return "/api/v3/brokerage"
	}
	return "/api/v3/brokerage/market"
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
	if err := s.c.API.GetJSON(ctx, s.basePath()+"/products", q, &out); err != nil {
		return nil, err
	}
	return out.Products, nil
}

// GetProduct returns a single product by ID (e.g. "BTC-USD").
func (s *service) GetProduct(ctx context.Context, productID string) (*Product, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		// Without an ID the request would hit the list endpoint and silently
		// decode its envelope into an empty Product.
		return nil, errors.New(`productId is required (e.g. "BTC-USD")`)
	}
	var p Product
	path := fmt.Sprintf("%s/products/%s", s.basePath(), url.PathEscape(productID))
	if err := s.c.API.GetJSON(ctx, path, nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Candle is one OHLCV bucket. All numeric fields are decimal strings; Start is
// a UNIX timestamp in seconds.
type Candle struct {
	Start  string `json:"start"`
	Low    string `json:"low"`
	High   string `json:"high"`
	Open   string `json:"open"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

// GetCandles returns OHLCV history for a product. start/end are UNIX seconds;
// granularity is one of the API's bucket sizes (e.g. ONE_MINUTE, ONE_HOUR,
// ONE_DAY). limit caps the number of buckets (optional).
func (s *service) GetCandles(ctx context.Context, productID, start, end, granularity string, limit int) ([]Candle, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, errors.New(`productId is required (e.g. "BTC-USD")`)
	}
	q := url.Values{}
	q.Set("start", start)
	q.Set("end", end)
	q.Set("granularity", granularity)
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	var out struct {
		Candles []Candle `json:"candles"`
	}
	path := fmt.Sprintf("%s/products/%s/candles", s.basePath(), url.PathEscape(productID))
	if err := s.c.API.GetJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return out.Candles, nil
}

// Trade is a single market trade from the ticker endpoint.
type Trade struct {
	TradeID   string `json:"trade_id"`
	ProductID string `json:"product_id"`
	Price     string `json:"price"`
	Size      string `json:"size"`
	Time      string `json:"time"`
	Side      string `json:"side"`
}

// Ticker is the recent-trades snapshot for a product.
type Ticker struct {
	Trades  []Trade `json:"trades"`
	BestBid string  `json:"best_bid,omitempty"`
	BestAsk string  `json:"best_ask,omitempty"`
}

// GetTicker returns recent market trades plus the current best bid/ask.
func (s *service) GetTicker(ctx context.Context, productID string, limit int) (*Ticker, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, errors.New(`productId is required (e.g. "BTC-USD")`)
	}
	q := url.Values{}
	if limit <= 0 {
		limit = 10
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	var t Ticker
	path := fmt.Sprintf("%s/products/%s/ticker", s.basePath(), url.PathEscape(productID))
	if err := s.c.API.GetJSON(ctx, path, q, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// BookLevel is one price level of the order book.
type BookLevel struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

// Book is the order book snapshot for a product.
type Book struct {
	ProductID string      `json:"product_id"`
	Bids      []BookLevel `json:"bids"`
	Asks      []BookLevel `json:"asks"`
	Time      string      `json:"time,omitempty"`
}

// GetBook returns the order book for a product, optionally limited to the top
// N levels per side.
func (s *service) GetBook(ctx context.Context, productID string, limit int) (*Book, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, errors.New(`productId is required (e.g. "BTC-USD")`)
	}
	q := url.Values{}
	q.Set("product_id", productID)
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	var out struct {
		Pricebook Book `json:"pricebook"`
	}
	if err := s.c.API.GetJSON(ctx, s.basePath()+"/product_book", q, &out); err != nil {
		return nil, err
	}
	return &out.Pricebook, nil
}

// BestBidAsk returns the current best bid/ask (top of book) for one or more
// products in a single call. Authenticated endpoint.
func (s *service) BestBidAsk(ctx context.Context, productIDs []string) ([]Book, error) {
	ids := make([]string, 0, len(productIDs))
	for _, id := range productIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("productIds is required (at least one product ID)")
	}
	q := url.Values{"product_ids": ids}
	var out struct {
		Pricebooks []Book `json:"pricebooks"`
	}
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/best_bid_ask", q, &out); err != nil {
		return nil, err
	}
	return out.Pricebooks, nil
}

// ServerTime is the Coinbase API clock reading.
type ServerTime struct {
	ISO          string `json:"iso"`
	EpochSeconds string `json:"epochSeconds"`
	EpochMillis  string `json:"epochMillis"`
}

// GetTime returns the API server time.
func (s *service) GetTime(ctx context.Context) (*ServerTime, error) {
	var t ServerTime
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/time", nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
