// SPDX-License-Identifier: MIT

package payments

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the payments toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "payments_list",
		Title:       "List payment methods",
		Description: "List the authenticated user's linked payment methods (funding sources) and what each allows.",
	}, svc.list)

	server.Register(s, server.ToolDef{
		Name:        "payments_get",
		Title:       "Get payment method",
		Description: "Get a single linked payment method by its ID.",
	}, svc.get)
}

// --- Tool input types (schemas are inferred from these structs) ---

// ListInput has no fields; the payment-method list takes no parameters.
type ListInput struct{}

// GetInput identifies a single payment method.
type GetInput struct {
	PaymentMethodID string `json:"paymentMethodId" jsonschema:"payment method ID"`
}

// --- Tool handlers ---

func (s *service) list(ctx context.Context, _ *mcp.CallToolRequest, _ ListInput) (*mcp.CallToolResult, server.ListResult[PaymentMethod], error) {
	out, err := s.ListPaymentMethods(ctx)
	return nil, server.List(out), err
}

func (s *service) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, *PaymentMethod, error) {
	out, err := s.GetPaymentMethod(ctx, in.PaymentMethodID)
	return nil, out, err
}
