// SPDX-License-Identifier: MIT

package products

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the products toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "products_list",
		Title:       "List products",
		Description: "List tradeable Coinbase products (markets) with current price and 24h stats.",
	}, svc.list)

	server.Register(s, server.ToolDef{
		Name:        "products_get",
		Title:       "Get product",
		Description: "Get a single Coinbase product by ID, e.g. BTC-USD.",
	}, svc.get)

	server.Register(s, server.ToolDef{
		Name:        "products_candles",
		Title:       "Get candles",
		Description: "Get OHLCV price history (candles) for a product over a time range.",
	}, svc.candles)

	server.Register(s, server.ToolDef{
		Name:        "products_ticker",
		Title:       "Get ticker",
		Description: "Get recent market trades and the current best bid/ask for a product.",
	}, svc.ticker)

	server.Register(s, server.ToolDef{
		Name:        "products_book",
		Title:       "Get order book",
		Description: "Get the order book (bids and asks) for a product.",
	}, svc.book)

	server.Register(s, server.ToolDef{
		Name:        "products_time",
		Title:       "Get server time",
		Description: "Get the Coinbase API server time (ISO and UNIX epoch).",
	}, svc.time)
}

// RegisterAuth adds the products tools that exist only as authenticated
// endpoints. Wired as a separate Auth toolset entry under the same name, so it
// is skipped without credentials; it intentionally does not call NoteToolset.
func RegisterAuth(s *server.Server, c *coinbase.Clients) {
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "products_best_bid_ask",
		Title:       "Get best bid/ask",
		Description: "Get the current best bid and ask (top of book) for one or more products in a single call.",
	}, svc.bestBidAsk)
}

// --- Tool input types (schemas are inferred from these structs) ---

// ListInput filters the product list.
type ListInput struct {
	ProductType string `json:"productType,omitempty" jsonschema:"filter by product type, e.g. SPOT or FUTURE (optional)"`
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum number of products to return (optional)"`
}

// GetInput identifies a single product.
type GetInput struct {
	ProductID string `json:"productId" jsonschema:"product ID, e.g. BTC-USD"`
}

// CandlesInput selects a product, time range, and bucket size.
type CandlesInput struct {
	ProductID   string `json:"productId" jsonschema:"product ID, e.g. BTC-USD"`
	Start       string `json:"start" jsonschema:"range start as a UNIX timestamp in seconds"`
	End         string `json:"end" jsonschema:"range end as a UNIX timestamp in seconds"`
	Granularity string `json:"granularity" jsonschema:"candle size: ONE_MINUTE FIVE_MINUTE FIFTEEN_MINUTE THIRTY_MINUTE ONE_HOUR TWO_HOUR SIX_HOUR or ONE_DAY"`
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum number of candles to return (optional)"`
}

// TickerInput selects a product and how many trades to return.
type TickerInput struct {
	ProductID string `json:"productId" jsonschema:"product ID, e.g. BTC-USD"`
	Limit     int    `json:"limit,omitempty" jsonschema:"number of recent trades to return (default 10)"`
}

// BookInput selects a product and book depth.
type BookInput struct {
	ProductID string `json:"productId" jsonschema:"product ID, e.g. BTC-USD"`
	Limit     int    `json:"limit,omitempty" jsonschema:"number of price levels per side (optional)"`
}

// TimeInput has no fields; the server time takes no parameters.
type TimeInput struct{}

// BestBidAskInput selects the products to quote.
type BestBidAskInput struct {
	ProductIDs []string `json:"productIds" jsonschema:"product IDs to quote, e.g. [\"BTC-USD\", \"ETH-USD\"]"`
}

// --- Tool handlers ---

func (s *service) list(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, server.ListResult[Product], error) {
	out, err := s.ListProducts(ctx, in.ProductType, in.Limit)
	return nil, server.List(out), err
}

func (s *service) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, *Product, error) {
	out, err := s.GetProduct(ctx, in.ProductID)
	return nil, out, err
}

func (s *service) candles(ctx context.Context, _ *mcp.CallToolRequest, in CandlesInput) (*mcp.CallToolResult, server.ListResult[Candle], error) {
	out, err := s.GetCandles(ctx, in.ProductID, in.Start, in.End, in.Granularity, in.Limit)
	return nil, server.List(out), err
}

func (s *service) ticker(ctx context.Context, _ *mcp.CallToolRequest, in TickerInput) (*mcp.CallToolResult, *Ticker, error) {
	out, err := s.GetTicker(ctx, in.ProductID, in.Limit)
	return nil, out, err
}

func (s *service) book(ctx context.Context, _ *mcp.CallToolRequest, in BookInput) (*mcp.CallToolResult, *Book, error) {
	out, err := s.GetBook(ctx, in.ProductID, in.Limit)
	return nil, out, err
}

func (s *service) time(ctx context.Context, _ *mcp.CallToolRequest, _ TimeInput) (*mcp.CallToolResult, *ServerTime, error) {
	out, err := s.GetTime(ctx)
	return nil, out, err
}

func (s *service) bestBidAsk(ctx context.Context, _ *mcp.CallToolRequest, in BestBidAskInput) (*mcp.CallToolResult, server.ListResult[Book], error) {
	out, err := s.BestBidAsk(ctx, in.ProductIDs)
	return nil, server.List(out), err
}
