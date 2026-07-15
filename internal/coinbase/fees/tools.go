// SPDX-License-Identifier: MIT

package fees

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the fees toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "fees_summary",
		Title:       "Get fee summary",
		Description: "Get the authenticated user's trading volume, fees paid, and current fee tier (maker/taker rates).",
	}, svc.summary)
}

// --- Tool input types (schemas are inferred from these structs) ---

// SummaryInput filters the transaction summary; all fields are optional.
type SummaryInput struct {
	ProductType        string `json:"productType,omitempty" jsonschema:"filter by product type: SPOT or FUTURE (optional)"`
	ProductVenue       string `json:"productVenue,omitempty" jsonschema:"filter by venue: CBE FCM or INTX (optional)"`
	ContractExpiryType string `json:"contractExpiryType,omitempty" jsonschema:"filter futures by expiry type: EXPIRING or PERPETUAL (optional)"`
}

// --- Tool handlers ---

func (s *service) summary(ctx context.Context, _ *mcp.CallToolRequest, in SummaryInput) (*mcp.CallToolResult, *Summary, error) {
	out, err := s.GetSummary(ctx, in.ProductType, in.ProductVenue, in.ContractExpiryType)
	return nil, out, err
}
