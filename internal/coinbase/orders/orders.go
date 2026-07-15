// SPDX-License-Identifier: MIT

// Package orders exposes Coinbase Advanced Trade order management: creating,
// previewing, editing, and cancelling orders, plus order history and fills.
package orders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "orders"

// service wraps the Coinbase clients for order operations.
type service struct {
	c *coinbase.Clients
}

// --- Order configuration ---

// marketIOC is a market order executed immediately at the best available
// price. Exactly one of QuoteSize (BUY) or BaseSize (SELL) is set.
type marketIOC struct {
	QuoteSize string `json:"quote_size,omitempty"`
	BaseSize  string `json:"base_size,omitempty"`
}

// limitGTC is a limit order that rests on the book until cancelled.
type limitGTC struct {
	BaseSize   string `json:"base_size"`
	LimitPrice string `json:"limit_price"`
	PostOnly   bool   `json:"post_only"`
}

// limitGTD is a limit order that expires at EndTime.
type limitGTD struct {
	BaseSize   string `json:"base_size"`
	LimitPrice string `json:"limit_price"`
	EndTime    string `json:"end_time"`
	PostOnly   bool   `json:"post_only"`
}

// limitFOK is a limit order that fills completely and immediately or not at
// all.
type limitFOK struct {
	BaseSize   string `json:"base_size"`
	LimitPrice string `json:"limit_price"`
}

// sorLimitIOC is a smart-order-routed limit order, immediate-or-cancel.
type sorLimitIOC struct {
	BaseSize   string `json:"base_size"`
	LimitPrice string `json:"limit_price"`
}

// stopLimitGTC is a stop-limit order that rests until cancelled.
type stopLimitGTC struct {
	BaseSize      string `json:"base_size"`
	LimitPrice    string `json:"limit_price"`
	StopPrice     string `json:"stop_price"`
	StopDirection string `json:"stop_direction"`
}

// stopLimitGTD is a stop-limit order that expires at EndTime.
type stopLimitGTD struct {
	BaseSize      string `json:"base_size"`
	LimitPrice    string `json:"limit_price"`
	StopPrice     string `json:"stop_price"`
	EndTime       string `json:"end_time"`
	StopDirection string `json:"stop_direction"`
}

// triggerBracketGTC is a limit order with an attached stop-loss trigger.
type triggerBracketGTC struct {
	BaseSize         string `json:"base_size"`
	LimitPrice       string `json:"limit_price"`
	StopTriggerPrice string `json:"stop_trigger_price"`
}

// triggerBracketGTD is a bracket order that expires at EndTime.
type triggerBracketGTD struct {
	BaseSize         string `json:"base_size"`
	LimitPrice       string `json:"limit_price"`
	StopTriggerPrice string `json:"stop_trigger_price"`
	EndTime          string `json:"end_time"`
}

// orderConfiguration is the API's polymorphic order shape; exactly one field
// is populated. The variants mirror the official SDK's OrderConfiguration.
type orderConfiguration struct {
	MarketIOC    *marketIOC         `json:"market_market_ioc,omitempty"`
	SorLimitIOC  *sorLimitIOC       `json:"sor_limit_ioc,omitempty"`
	LimitGTC     *limitGTC          `json:"limit_limit_gtc,omitempty"`
	LimitGTD     *limitGTD          `json:"limit_limit_gtd,omitempty"`
	LimitFOK     *limitFOK          `json:"limit_limit_fok,omitempty"`
	StopLimitGTC *stopLimitGTC      `json:"stop_limit_stop_limit_gtc,omitempty"`
	StopLimitGTD *stopLimitGTD      `json:"stop_limit_stop_limit_gtd,omitempty"`
	TriggerGTC   *triggerBracketGTC `json:"trigger_bracket_gtc,omitempty"`
	TriggerGTD   *triggerBracketGTD `json:"trigger_bracket_gtd,omitempty"`
}

// ConfigParams are the flattened order-shape parameters shared by order
// creation and preview.
type ConfigParams struct {
	// OrderType is one of "market", "limit", "limit_fok", "sor_limit",
	// "stop_limit", or "bracket".
	OrderType string
	// BaseSize is the order quantity in base currency (e.g. BTC).
	BaseSize string
	// QuoteSize is the order quantity in quote currency (e.g. USD); market
	// orders only.
	QuoteSize string
	// LimitPrice is the limit order price; required for every type but market.
	LimitPrice string
	// PostOnly restricts a limit order to maker-only execution.
	PostOnly bool
	// EndTime, when set (RFC3339), makes limit, stop_limit, and bracket
	// orders good-til-date instead of good-til-cancelled.
	EndTime string
	// StopPrice is the trigger price; required for stop_limit orders.
	StopPrice string
	// StopDirection is STOP_DIRECTION_STOP_UP or STOP_DIRECTION_STOP_DOWN;
	// required for stop_limit orders.
	StopDirection string
	// StopTriggerPrice is the attached stop-loss trigger; required for
	// bracket orders.
	StopTriggerPrice string
}

// orderTypes documents the accepted OrderType values for error messages.
const orderTypes = `"market", "limit", "limit_fok", "sor_limit", "stop_limit", or "bracket"`

// attachedBracket is the take-profit/stop-loss pair attached to a parent
// order; the exit order's size is inherited from the parent.
type attachedBracket struct {
	LimitPrice       string `json:"limit_price"`
	StopTriggerPrice string `json:"stop_trigger_price"`
}

// attachedOrderConfiguration wraps the attached TP/SL bracket.
type attachedOrderConfiguration struct {
	TriggerGTC *attachedBracket `json:"trigger_bracket_gtc,omitempty"`
}

// buildAttached validates and assembles the optional attached TP/SL bracket.
// Both prices must be given together, or neither.
func buildAttached(takeProfit, stopLoss string) (*attachedOrderConfiguration, error) {
	takeProfit = strings.TrimSpace(takeProfit)
	stopLoss = strings.TrimSpace(stopLoss)
	switch {
	case takeProfit == "" && stopLoss == "":
		return nil, nil
	case takeProfit == "" || stopLoss == "":
		return nil, errors.New("takeProfitPrice and stopLossPrice must be set together (attached TP/SL bracket)")
	}
	return &attachedOrderConfiguration{TriggerGTC: &attachedBracket{
		LimitPrice:       takeProfit,
		StopTriggerPrice: stopLoss,
	}}, nil
}

// requireBaseAndPrice validates the fields every non-market type needs.
func requireBaseAndPrice(base, price, orderType string) error {
	if price == "" {
		return fmt.Errorf("limitPrice is required for %s orders", orderType)
	}
	if base == "" {
		return fmt.Errorf("baseSize is required for %s orders", orderType)
	}
	return nil
}

// buildConfig validates the flattened parameters and assembles the API's
// order_configuration object. The variants and their required fields mirror
// the official SDK's OrderConfiguration.
func buildConfig(p ConfigParams) (*orderConfiguration, error) {
	base := strings.TrimSpace(p.BaseSize)
	quote := strings.TrimSpace(p.QuoteSize)
	price := strings.TrimSpace(p.LimitPrice)
	end := strings.TrimSpace(p.EndTime)
	switch strings.ToLower(strings.TrimSpace(p.OrderType)) {
	case "":
		return nil, errors.New("orderType is required (" + orderTypes + ")")
	case "market":
		if (base == "") == (quote == "") {
			return nil, errors.New("market orders require exactly one of baseSize or quoteSize")
		}
		return &orderConfiguration{MarketIOC: &marketIOC{QuoteSize: quote, BaseSize: base}}, nil
	case "limit":
		if err := requireBaseAndPrice(base, price, "limit"); err != nil {
			return nil, err
		}
		if end != "" {
			return &orderConfiguration{LimitGTD: &limitGTD{BaseSize: base, LimitPrice: price, EndTime: end, PostOnly: p.PostOnly}}, nil
		}
		return &orderConfiguration{LimitGTC: &limitGTC{BaseSize: base, LimitPrice: price, PostOnly: p.PostOnly}}, nil
	case "limit_fok":
		if err := requireBaseAndPrice(base, price, "limit_fok"); err != nil {
			return nil, err
		}
		return &orderConfiguration{LimitFOK: &limitFOK{BaseSize: base, LimitPrice: price}}, nil
	case "sor_limit":
		if err := requireBaseAndPrice(base, price, "sor_limit"); err != nil {
			return nil, err
		}
		return &orderConfiguration{SorLimitIOC: &sorLimitIOC{BaseSize: base, LimitPrice: price}}, nil
	case "stop_limit":
		if err := requireBaseAndPrice(base, price, "stop_limit"); err != nil {
			return nil, err
		}
		stop := strings.TrimSpace(p.StopPrice)
		if stop == "" {
			return nil, errors.New("stopPrice is required for stop_limit orders")
		}
		dir := strings.ToUpper(strings.TrimSpace(p.StopDirection))
		if dir != "STOP_DIRECTION_STOP_UP" && dir != "STOP_DIRECTION_STOP_DOWN" {
			return nil, fmt.Errorf("stopDirection must be STOP_DIRECTION_STOP_UP or STOP_DIRECTION_STOP_DOWN, got %q", p.StopDirection)
		}
		if end != "" {
			return &orderConfiguration{StopLimitGTD: &stopLimitGTD{BaseSize: base, LimitPrice: price, StopPrice: stop, EndTime: end, StopDirection: dir}}, nil
		}
		return &orderConfiguration{StopLimitGTC: &stopLimitGTC{BaseSize: base, LimitPrice: price, StopPrice: stop, StopDirection: dir}}, nil
	case "bracket":
		if err := requireBaseAndPrice(base, price, "bracket"); err != nil {
			return nil, err
		}
		trigger := strings.TrimSpace(p.StopTriggerPrice)
		if trigger == "" {
			return nil, errors.New("stopTriggerPrice is required for bracket orders")
		}
		if end != "" {
			return &orderConfiguration{TriggerGTD: &triggerBracketGTD{BaseSize: base, LimitPrice: price, StopTriggerPrice: trigger, EndTime: end}}, nil
		}
		return &orderConfiguration{TriggerGTC: &triggerBracketGTC{BaseSize: base, LimitPrice: price, StopTriggerPrice: trigger}}, nil
	default:
		return nil, fmt.Errorf("orderType must be one of "+orderTypes+", got %q", p.OrderType)
	}
}

// normalizeSide validates and canonicalizes an order side to BUY or SELL.
func normalizeSide(side string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(side))
	switch s {
	case "BUY", "SELL":
		return s, nil
	case "":
		return "", errors.New(`side is required ("BUY" or "SELL")`)
	default:
		return "", fmt.Errorf(`side must be "BUY" or "SELL", got %q`, side)
	}
}

// newClientOrderID generates a random hex idempotency key for an order.
func newClientOrderID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read never fails (Go >= 1.24)
	return hex.EncodeToString(b)
}

// --- Create / close position ---

// CreateResult identifies a successfully placed order.
type CreateResult struct {
	OrderID       string `json:"order_id"`
	ProductID     string `json:"product_id"`
	Side          string `json:"side"`
	ClientOrderID string `json:"client_order_id"`
}

// createError is the API's rejection detail when an order is not accepted.
type createError struct {
	Error                string `json:"error"`
	Message              string `json:"message"`
	ErrorDetails         string `json:"error_details"`
	PreviewFailureReason string `json:"preview_failure_reason"`
}

// asError converts a rejection into a Go error carrying every detail the API
// provided, so the model can see why the order failed.
func (e *createError) asError() error {
	var parts []string
	if e != nil {
		for _, p := range []string{e.Error, e.Message, e.ErrorDetails, e.PreviewFailureReason} {
			if p != "" {
				parts = append(parts, p)
			}
		}
	}
	if len(parts) == 0 {
		return errors.New("order rejected (no error details returned)")
	}
	return fmt.Errorf("order rejected: %s", strings.Join(parts, ": "))
}

// createResponse is the envelope shared by the create and close-position
// endpoints.
type createResponse struct {
	Success         bool         `json:"success"`
	SuccessResponse CreateResult `json:"success_response"`
	ErrorResponse   *createError `json:"error_response"`
}

// createRequest is the POST body for order creation.
type createRequest struct {
	ClientOrderID              string                      `json:"client_order_id"`
	ProductID                  string                      `json:"product_id"`
	Side                       string                      `json:"side"`
	OrderConfiguration         orderConfiguration          `json:"order_configuration"`
	AttachedOrderConfiguration *attachedOrderConfiguration `json:"attached_order_configuration,omitempty"`
}

// CreateParams describes an order to place.
type CreateParams struct {
	ConfigParams
	// ProductID is the trading pair, e.g. "BTC-USD".
	ProductID string
	// Side is BUY or SELL.
	Side string
	// ClientOrderID is an optional idempotency key; generated when empty.
	ClientOrderID string
	// TakeProfitPrice / StopLossPrice, when both set, attach a TP/SL bracket
	// that creates the exit order when the parent fills (BUY orders).
	TakeProfitPrice string
	StopLossPrice   string
}

// CreateOrder places an order. When the API accepts the request but rejects
// the order (success=false), the rejection details are returned as an error.
func (s *service) CreateOrder(ctx context.Context, p CreateParams) (*CreateResult, error) {
	productID := strings.TrimSpace(p.ProductID)
	if productID == "" {
		return nil, errors.New(`productId is required (e.g. "BTC-USD")`)
	}
	side, err := normalizeSide(p.Side)
	if err != nil {
		return nil, err
	}
	cfg, err := buildConfig(p.ConfigParams)
	if err != nil {
		return nil, err
	}
	attached, err := buildAttached(p.TakeProfitPrice, p.StopLossPrice)
	if err != nil {
		return nil, err
	}
	clientOrderID := strings.TrimSpace(p.ClientOrderID)
	if clientOrderID == "" {
		clientOrderID = newClientOrderID()
	}
	body := createRequest{
		ClientOrderID:              clientOrderID,
		ProductID:                  productID,
		Side:                       side,
		OrderConfiguration:         *cfg,
		AttachedOrderConfiguration: attached,
	}
	var out createResponse
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/orders", nil, body, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, out.ErrorResponse.asError()
	}
	return &out.SuccessResponse, nil
}

// closePositionRequest is the POST body for closing a position.
type closePositionRequest struct {
	ClientOrderID string `json:"client_order_id"`
	ProductID     string `json:"product_id"`
	Size          string `json:"size,omitempty"`
}

// ClosePosition places an order that closes an open position on the product.
// size optionally closes only part of the position. Rejections (success=false)
// are returned as an error.
func (s *service) ClosePosition(ctx context.Context, clientOrderID, productID, size string) (*CreateResult, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, errors.New(`productId is required (e.g. "BTC-USD")`)
	}
	clientOrderID = strings.TrimSpace(clientOrderID)
	if clientOrderID == "" {
		clientOrderID = newClientOrderID()
	}
	body := closePositionRequest{
		ClientOrderID: clientOrderID,
		ProductID:     productID,
		Size:          strings.TrimSpace(size),
	}
	var out createResponse
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/orders/close_position", nil, body, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, out.ErrorResponse.asError()
	}
	return &out.SuccessResponse, nil
}

// --- Preview ---

// previewRequest is the POST body for an order preview.
type previewRequest struct {
	ProductID          string             `json:"product_id"`
	Side               string             `json:"side"`
	OrderConfiguration orderConfiguration `json:"order_configuration"`
}

// Preview is the projected outcome of an order before placing it. Monetary
// values are decimal strings.
type Preview struct {
	OrderTotal      string   `json:"order_total,omitempty"`
	CommissionTotal string   `json:"commission_total,omitempty"`
	Errs            []string `json:"errs,omitempty"`
	Warning         []string `json:"warning,omitempty"`
	QuoteSize       string   `json:"quote_size,omitempty"`
	BaseSize        string   `json:"base_size,omitempty"`
	BestBid         string   `json:"best_bid,omitempty"`
	BestAsk         string   `json:"best_ask,omitempty"`
	IsMax           bool     `json:"is_max,omitempty"`
	Slippage        string   `json:"slippage,omitempty"`
}

// PreviewParams describes an order to preview.
type PreviewParams struct {
	ConfigParams
	// ProductID is the trading pair, e.g. "BTC-USD".
	ProductID string
	// Side is BUY or SELL.
	Side string
}

// PreviewOrder simulates placing an order and returns the projected totals,
// fees, and any errors or warnings, without placing it.
func (s *service) PreviewOrder(ctx context.Context, p PreviewParams) (*Preview, error) {
	productID := strings.TrimSpace(p.ProductID)
	if productID == "" {
		return nil, errors.New(`productId is required (e.g. "BTC-USD")`)
	}
	side, err := normalizeSide(p.Side)
	if err != nil {
		return nil, err
	}
	cfg, err := buildConfig(p.ConfigParams)
	if err != nil {
		return nil, err
	}
	body := previewRequest{ProductID: productID, Side: side, OrderConfiguration: *cfg}
	var out Preview
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/orders/preview", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Edit ---

// editRequest is the POST body shared by edit and edit-preview.
type editRequest struct {
	OrderID string `json:"order_id"`
	Price   string `json:"price"`
	Size    string `json:"size"`
}

// validateEdit checks the shared edit inputs and builds the request body.
func validateEdit(orderID, price, size string) (*editRequest, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, errors.New("orderId is required")
	}
	price = strings.TrimSpace(price)
	if price == "" {
		return nil, errors.New("price is required")
	}
	size = strings.TrimSpace(size)
	if size == "" {
		return nil, errors.New("size is required")
	}
	return &editRequest{OrderID: orderID, Price: price, Size: size}, nil
}

// EditFailure describes why an edit was (or would be) rejected.
type EditFailure struct {
	EditFailureReason    string `json:"edit_failure_reason,omitempty"`
	PreviewFailureReason string `json:"preview_failure_reason,omitempty"`
}

// editFailuresAsError converts edit rejections into a Go error carrying every
// reason the API provided.
func editFailuresAsError(failures []EditFailure) error {
	var parts []string
	for _, f := range failures {
		if f.EditFailureReason != "" {
			parts = append(parts, f.EditFailureReason)
		}
		if f.PreviewFailureReason != "" {
			parts = append(parts, f.PreviewFailureReason)
		}
	}
	if len(parts) == 0 {
		return errors.New("edit rejected (no error details returned)")
	}
	return fmt.Errorf("edit rejected: %s", strings.Join(parts, "; "))
}

// EditResult reports a successful order edit.
type EditResult struct {
	Success bool `json:"success"`
}

// editResponse is the envelope returned by the edit endpoint.
type editResponse struct {
	Success bool          `json:"success"`
	Errors  []EditFailure `json:"errors"`
}

// EditOrder changes the price and size of a resting limit order. Rejections
// (success=false) are returned as an error.
func (s *service) EditOrder(ctx context.Context, orderID, price, size string) (*EditResult, error) {
	body, err := validateEdit(orderID, price, size)
	if err != nil {
		return nil, err
	}
	var out editResponse
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/orders/edit", nil, body, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, editFailuresAsError(out.Errors)
	}
	return &EditResult{Success: true}, nil
}

// EditPreview is the projected outcome of editing an order. Monetary values
// are decimal strings.
type EditPreview struct {
	Errors             []EditFailure `json:"errors,omitempty"`
	Slippage           string        `json:"slippage,omitempty"`
	OrderTotal         string        `json:"order_total,omitempty"`
	CommissionTotal    string        `json:"commission_total,omitempty"`
	QuoteSize          string        `json:"quote_size,omitempty"`
	BaseSize           string        `json:"base_size,omitempty"`
	AverageFilledPrice string        `json:"average_filled_price,omitempty"`
}

// PreviewEditOrder simulates editing an order and returns the projected
// totals, fees, and any blocking errors, without changing the order.
func (s *service) PreviewEditOrder(ctx context.Context, orderID, price, size string) (*EditPreview, error) {
	body, err := validateEdit(orderID, price, size)
	if err != nil {
		return nil, err
	}
	var out EditPreview
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/orders/edit_preview", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Cancel ---

// CancelResult is the per-order outcome of a cancel request.
type CancelResult struct {
	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason,omitempty"`
	OrderID       string `json:"order_id"`
}

// CancelOrders cancels the given orders in one batch, returning a per-order
// success/failure result.
func (s *service) CancelOrders(ctx context.Context, orderIDs []string) ([]CancelResult, error) {
	ids := make([]string, 0, len(orderIDs))
	for _, id := range orderIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("orderIds is required (at least one order ID)")
	}
	body := struct {
		OrderIDs []string `json:"order_ids"`
	}{OrderIDs: ids}
	var out struct {
		Results []CancelResult `json:"results"`
	}
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/orders/batch_cancel", nil, body, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// --- History ---

// Order is a historical order, trimmed to the fields useful to an LLM. All
// monetary and quantity values are decimal strings.
type Order struct {
	OrderID              string `json:"order_id"`
	ClientOrderID        string `json:"client_order_id,omitempty"`
	ProductID            string `json:"product_id,omitempty"`
	Side                 string `json:"side,omitempty"`
	Status               string `json:"status,omitempty"`
	OrderType            string `json:"order_type,omitempty"`
	TimeInForce          string `json:"time_in_force,omitempty"`
	CreatedTime          string `json:"created_time,omitempty"`
	CompletionPercentage string `json:"completion_percentage,omitempty"`
	FilledSize           string `json:"filled_size,omitempty"`
	AverageFilledPrice   string `json:"average_filled_price,omitempty"`
	FilledValue          string `json:"filled_value,omitempty"`
	NumberOfFills        string `json:"number_of_fills,omitempty"`
	TotalFees            string `json:"total_fees,omitempty"`
	TotalValueAfterFees  string `json:"total_value_after_fees,omitempty"`
	Settled              bool   `json:"settled,omitempty"`
	RejectReason         string `json:"reject_reason,omitempty"`
	CancelMessage        string `json:"cancel_message,omitempty"`
}

// OrdersPage is one page of historical orders.
type OrdersPage struct {
	Orders  []Order `json:"orders"`
	HasNext bool    `json:"has_next"`
	Cursor  string  `json:"cursor,omitempty"`
}

// ListOrders returns historical orders, optionally filtered by product,
// status (e.g. OPEN, FILLED, CANCELLED), and side, with cursor pagination.
func (s *service) ListOrders(ctx context.Context, productID, status, side string, limit int, cursor string) (*OrdersPage, error) {
	q := url.Values{}
	if productID = strings.TrimSpace(productID); productID != "" {
		q.Set("product_ids", productID)
	}
	if status = strings.TrimSpace(status); status != "" {
		q.Set("order_status", status)
	}
	if side = strings.TrimSpace(side); side != "" {
		q.Set("order_side", side)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if cursor = strings.TrimSpace(cursor); cursor != "" {
		q.Set("cursor", cursor)
	}
	var out OrdersPage
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/orders/historical/batch", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOrder returns a single historical order by ID.
func (s *service) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, errors.New("orderId is required")
	}
	var out struct {
		Order Order `json:"order"`
	}
	path := fmt.Sprintf("/api/v3/brokerage/orders/historical/%s", url.PathEscape(orderID))
	if err := s.c.API.GetJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out.Order, nil
}

// Fill is a single execution (partial or full) of an order. Monetary values
// are decimal strings.
type Fill struct {
	EntryID            string `json:"entry_id"`
	TradeID            string `json:"trade_id,omitempty"`
	OrderID            string `json:"order_id,omitempty"`
	TradeTime          string `json:"trade_time,omitempty"`
	TradeType          string `json:"trade_type,omitempty"`
	Price              string `json:"price,omitempty"`
	Size               string `json:"size,omitempty"`
	Commission         string `json:"commission,omitempty"`
	ProductID          string `json:"product_id,omitempty"`
	Side               string `json:"side,omitempty"`
	LiquidityIndicator string `json:"liquidity_indicator,omitempty"`
}

// FillsPage is one page of fills.
type FillsPage struct {
	Fills  []Fill `json:"fills"`
	Cursor string `json:"cursor,omitempty"`
}

// ListFills returns fills (executions), optionally filtered by order and/or
// product, with cursor pagination.
func (s *service) ListFills(ctx context.Context, orderID, productID string, limit int, cursor string) (*FillsPage, error) {
	q := url.Values{}
	if orderID = strings.TrimSpace(orderID); orderID != "" {
		q.Set("order_ids", orderID)
	}
	if productID = strings.TrimSpace(productID); productID != "" {
		q.Set("product_ids", productID)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if cursor = strings.TrimSpace(cursor); cursor != "" {
		q.Set("cursor", cursor)
	}
	var out FillsPage
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/orders/historical/fills", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
