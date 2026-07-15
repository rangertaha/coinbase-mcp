// SPDX-License-Identifier: MIT

package orders

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the orders toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "orders_create",
		Title:       "Create order",
		Description: "Place an order on a Coinbase product. Types: market (BUY takes quoteSize, SELL takes baseSize), limit, limit_fok, sor_limit, stop_limit (stopPrice + stopDirection), and bracket (stopTriggerPrice); non-market types take baseSize and limitPrice, and endTime makes limit/stop_limit/bracket good-til-date. Places a REAL order with REAL funds: always call orders_preview first and confirm with the user.",
		Write:       true,
	}, svc.create)

	server.Register(s, server.ToolDef{
		Name:        "orders_preview",
		Title:       "Preview order",
		Description: "Simulate placing an order and get projected total, fees, slippage, and any errors, without placing it.",
	}, svc.preview)

	server.Register(s, server.ToolDef{
		Name:        "orders_edit",
		Title:       "Edit order",
		Description: "Edit the price and size of an existing open limit order. Changes a LIVE order on a real account.",
		Write:       true,
	}, svc.edit)

	server.Register(s, server.ToolDef{
		Name:        "orders_edit_preview",
		Title:       "Preview order edit",
		Description: "Simulate editing an order's price and size and get the projected outcome, without changing the order.",
	}, svc.editPreview)

	server.Register(s, server.ToolDef{
		Name:        "orders_cancel",
		Title:       "Cancel orders",
		Description: "Cancel one or more open orders by ID, returning a per-order success/failure result. Cancels LIVE orders on a real account.",
		Write:       true,
		Destructive: true,
	}, svc.cancel)

	server.Register(s, server.ToolDef{
		Name:        "orders_close_position",
		Title:       "Close position",
		Description: "Place an order that closes an open position on a product, optionally only a partial size. Places a REAL closing order with REAL funds.",
		Write:       true,
		Destructive: true,
	}, svc.closePosition)

	server.Register(s, server.ToolDef{
		Name:        "orders_list",
		Title:       "List orders",
		Description: "List historical orders, optionally filtered by product, status, and side, with cursor pagination.",
	}, svc.list)

	server.Register(s, server.ToolDef{
		Name:        "orders_get",
		Title:       "Get order",
		Description: "Get a single order by its order ID, including status and fill details.",
	}, svc.get)

	server.Register(s, server.ToolDef{
		Name:        "orders_fills",
		Title:       "List fills",
		Description: "List fills (executions), optionally filtered by order ID and/or product, with cursor pagination.",
	}, svc.fills)
}

// --- Tool input types (schemas are inferred from these structs) ---

// CreateInput describes an order to place.
type CreateInput struct {
	ProductID        string `json:"productId" jsonschema:"product ID, e.g. BTC-USD"`
	Side             string `json:"side" jsonschema:"order side: BUY or SELL"`
	OrderType        string `json:"orderType" jsonschema:"order type: market, limit, limit_fok, sor_limit, stop_limit, or bracket"`
	BaseSize         string `json:"baseSize,omitempty" jsonschema:"quantity in base currency; required for limit orders and market SELL"`
	QuoteSize        string `json:"quoteSize,omitempty" jsonschema:"amount in quote currency; used by market BUY"`
	LimitPrice       string `json:"limitPrice,omitempty" jsonschema:"limit price; required for limit orders"`
	PostOnly         bool   `json:"postOnly,omitempty" jsonschema:"restrict a limit order to maker-only execution (optional)"`
	EndTime          string `json:"endTime,omitempty" jsonschema:"RFC3339 expiry making limit, stop_limit, and bracket orders good-til-date instead of good-til-cancelled (optional)"`
	StopPrice        string `json:"stopPrice,omitempty" jsonschema:"trigger price; required for stop_limit orders"`
	StopDirection    string `json:"stopDirection,omitempty" jsonschema:"STOP_DIRECTION_STOP_UP or STOP_DIRECTION_STOP_DOWN; required for stop_limit orders"`
	StopTriggerPrice string `json:"stopTriggerPrice,omitempty" jsonschema:"attached stop-loss trigger price; required for bracket orders"`
	TakeProfitPrice  string `json:"takeProfitPrice,omitempty" jsonschema:"attach a TP/SL bracket: take-profit limit price (set together with stopLossPrice; exit size is inherited from the parent order)"`
	StopLossPrice    string `json:"stopLossPrice,omitempty" jsonschema:"attach a TP/SL bracket: stop-loss trigger price (set together with takeProfitPrice)"`
	ClientOrderID    string `json:"clientOrderId,omitempty" jsonschema:"idempotency key; auto-generated when omitted (optional)"`
}

// PreviewInput describes an order to preview.
type PreviewInput struct {
	ProductID        string `json:"productId" jsonschema:"product ID, e.g. BTC-USD"`
	Side             string `json:"side" jsonschema:"order side: BUY or SELL"`
	OrderType        string `json:"orderType" jsonschema:"order type: market, limit, limit_fok, sor_limit, stop_limit, or bracket"`
	BaseSize         string `json:"baseSize,omitempty" jsonschema:"quantity in base currency; required for limit orders and market SELL"`
	QuoteSize        string `json:"quoteSize,omitempty" jsonschema:"amount in quote currency; used by market BUY"`
	LimitPrice       string `json:"limitPrice,omitempty" jsonschema:"limit price; required for limit orders"`
	PostOnly         bool   `json:"postOnly,omitempty" jsonschema:"restrict a limit order to maker-only execution (optional)"`
	EndTime          string `json:"endTime,omitempty" jsonschema:"RFC3339 expiry making limit, stop_limit, and bracket orders good-til-date instead of good-til-cancelled (optional)"`
	StopPrice        string `json:"stopPrice,omitempty" jsonschema:"trigger price; required for stop_limit orders"`
	StopDirection    string `json:"stopDirection,omitempty" jsonschema:"STOP_DIRECTION_STOP_UP or STOP_DIRECTION_STOP_DOWN; required for stop_limit orders"`
	StopTriggerPrice string `json:"stopTriggerPrice,omitempty" jsonschema:"attached stop-loss trigger price; required for bracket orders"`
}

// EditInput identifies an order and its new price and size; shared by edit
// and edit-preview.
type EditInput struct {
	OrderID string `json:"orderId" jsonschema:"ID of the order to edit"`
	Price   string `json:"price" jsonschema:"new limit price"`
	Size    string `json:"size" jsonschema:"new order size in base currency"`
}

// CancelInput lists the orders to cancel.
type CancelInput struct {
	OrderIDs []string `json:"orderIds" jsonschema:"IDs of the orders to cancel (at least one)"`
}

// ClosePositionInput identifies the position to close.
type ClosePositionInput struct {
	ProductID     string `json:"productId" jsonschema:"product ID of the position to close, e.g. BTC-USD"`
	Size          string `json:"size,omitempty" jsonschema:"number of contracts to close; omit to close the whole position (optional)"`
	ClientOrderID string `json:"clientOrderId,omitempty" jsonschema:"idempotency key; auto-generated when omitted (optional)"`
}

// ListInput filters the historical orders list.
type ListInput struct {
	ProductID string `json:"productId,omitempty" jsonschema:"filter by product ID, e.g. BTC-USD (optional)"`
	Status    string `json:"status,omitempty" jsonschema:"filter by order status, e.g. OPEN FILLED CANCELLED (optional)"`
	Side      string `json:"side,omitempty" jsonschema:"filter by order side: BUY or SELL (optional)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum number of orders to return (optional)"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"pagination cursor from a previous page (optional)"`
}

// GetInput identifies a single order.
type GetInput struct {
	OrderID string `json:"orderId" jsonschema:"ID of the order to fetch"`
}

// FillsInput filters the fills list.
type FillsInput struct {
	OrderID   string `json:"orderId,omitempty" jsonschema:"filter by order ID (optional)"`
	ProductID string `json:"productId,omitempty" jsonschema:"filter by product ID, e.g. BTC-USD (optional)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum number of fills to return (optional)"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"pagination cursor from a previous page (optional)"`
}

// --- Tool handlers ---

func (s *service) create(ctx context.Context, _ *mcp.CallToolRequest, in CreateInput) (*mcp.CallToolResult, *CreateResult, error) {
	out, err := s.CreateOrder(ctx, CreateParams{
		ConfigParams: ConfigParams{
			OrderType:        in.OrderType,
			BaseSize:         in.BaseSize,
			QuoteSize:        in.QuoteSize,
			LimitPrice:       in.LimitPrice,
			PostOnly:         in.PostOnly,
			EndTime:          in.EndTime,
			StopPrice:        in.StopPrice,
			StopDirection:    in.StopDirection,
			StopTriggerPrice: in.StopTriggerPrice,
		},
		ProductID:       in.ProductID,
		Side:            in.Side,
		ClientOrderID:   in.ClientOrderID,
		TakeProfitPrice: in.TakeProfitPrice,
		StopLossPrice:   in.StopLossPrice,
	})
	return nil, out, err
}

func (s *service) preview(ctx context.Context, _ *mcp.CallToolRequest, in PreviewInput) (*mcp.CallToolResult, *Preview, error) {
	out, err := s.PreviewOrder(ctx, PreviewParams{
		ConfigParams: ConfigParams{
			OrderType:        in.OrderType,
			BaseSize:         in.BaseSize,
			QuoteSize:        in.QuoteSize,
			LimitPrice:       in.LimitPrice,
			PostOnly:         in.PostOnly,
			EndTime:          in.EndTime,
			StopPrice:        in.StopPrice,
			StopDirection:    in.StopDirection,
			StopTriggerPrice: in.StopTriggerPrice,
		},
		ProductID: in.ProductID,
		Side:      in.Side,
	})
	return nil, out, err
}

func (s *service) edit(ctx context.Context, _ *mcp.CallToolRequest, in EditInput) (*mcp.CallToolResult, *EditResult, error) {
	out, err := s.EditOrder(ctx, in.OrderID, in.Price, in.Size)
	return nil, out, err
}

func (s *service) editPreview(ctx context.Context, _ *mcp.CallToolRequest, in EditInput) (*mcp.CallToolResult, *EditPreview, error) {
	out, err := s.PreviewEditOrder(ctx, in.OrderID, in.Price, in.Size)
	return nil, out, err
}

func (s *service) cancel(ctx context.Context, _ *mcp.CallToolRequest, in CancelInput) (*mcp.CallToolResult, server.ListResult[CancelResult], error) {
	out, err := s.CancelOrders(ctx, in.OrderIDs)
	return nil, server.List(out), err
}

func (s *service) closePosition(ctx context.Context, _ *mcp.CallToolRequest, in ClosePositionInput) (*mcp.CallToolResult, *CreateResult, error) {
	out, err := s.ClosePosition(ctx, in.ClientOrderID, in.ProductID, in.Size)
	return nil, out, err
}

func (s *service) list(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, *OrdersPage, error) {
	out, err := s.ListOrders(ctx, in.ProductID, in.Status, in.Side, in.Limit, in.Cursor)
	return nil, out, err
}

func (s *service) get(ctx context.Context, _ *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, *Order, error) {
	out, err := s.GetOrder(ctx, in.OrderID)
	return nil, out, err
}

func (s *service) fills(ctx context.Context, _ *mcp.CallToolRequest, in FillsInput) (*mcp.CallToolResult, *FillsPage, error) {
	out, err := s.ListFills(ctx, in.OrderID, in.ProductID, in.Limit, in.Cursor)
	return nil, out, err
}
