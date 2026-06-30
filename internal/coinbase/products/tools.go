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

// --- Tool handlers ---

func (s *service) list(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, server.ListResult[Product], error) {
	out, err := s.ListProducts(ctx, in.ProductType, in.Limit)
	return nil, server.List(out), err
}

func (s *service) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, *Product, error) {
	out, err := s.GetProduct(ctx, in.ProductID)
	return nil, out, err
}
