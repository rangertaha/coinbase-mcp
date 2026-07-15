// SPDX-License-Identifier: MIT

package futures

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the futures toolset to the server.
func Register(s *server.Server, c *coinbase.Clients) {
	s.NoteToolset(Name)
	svc := &service{c: c}

	server.Register(s, server.ToolDef{
		Name:        "futures_balance",
		Title:       "Get futures balance summary",
		Description: "Get the CFM US futures balance summary: buying power, margin, and liquidation levels.",
	}, svc.balance)

	server.Register(s, server.ToolDef{
		Name:        "futures_positions",
		Title:       "List futures positions",
		Description: "List all open CFM US futures positions.",
	}, svc.positions)

	server.Register(s, server.ToolDef{
		Name:        "futures_position",
		Title:       "Get futures position",
		Description: "Get the open CFM US futures position for a single product.",
	}, svc.position)

	server.Register(s, server.ToolDef{
		Name:        "futures_sweeps",
		Title:       "List futures sweeps",
		Description: "List pending and processing USD sweeps between the futures account and the spot wallet.",
	}, svc.sweeps)

	server.Register(s, server.ToolDef{
		Name:        "futures_sweep_schedule",
		Title:       "Schedule futures sweep",
		Description: "Schedule a USD sweep from the CFM futures account to the Coinbase spot wallet. Moves REAL funds.",
		Write:       true,
	}, svc.sweepSchedule)

	server.Register(s, server.ToolDef{
		Name:        "futures_sweep_cancel",
		Title:       "Cancel futures sweep",
		Description: "Cancel the pending USD sweep from the CFM futures account.",
		Write:       true,
		Destructive: true,
	}, svc.sweepCancel)

	server.Register(s, server.ToolDef{
		Name:        "futures_margin_setting",
		Title:       "Get intraday margin setting",
		Description: "Get the current CFM intraday margin setting (standard or intraday).",
	}, svc.marginSetting)

	server.Register(s, server.ToolDef{
		Name:        "futures_margin_setting_set",
		Title:       "Set intraday margin setting",
		Description: "Set the CFM intraday margin setting, e.g. INTRADAY_MARGIN_SETTING_STANDARD or INTRADAY_MARGIN_SETTING_INTRADAY.",
		Write:       true,
	}, svc.marginSettingSet)

	server.Register(s, server.ToolDef{
		Name:        "futures_margin_window",
		Title:       "Get current margin window",
		Description: "Get the current CFM margin window and intraday margin killswitch states.",
	}, svc.marginWindow)
}

// --- Tool input types (schemas are inferred from these structs) ---

// BalanceInput has no fields; the balance summary takes no parameters.
type BalanceInput struct{}

// PositionsInput has no fields; listing positions takes no parameters.
type PositionsInput struct{}

// PositionInput identifies a single futures product.
type PositionInput struct {
	ProductID string `json:"productId" jsonschema:"futures product ID, e.g. BIT-31OCT25-CDE"`
}

// SweepsInput has no fields; listing sweeps takes no parameters.
type SweepsInput struct{}

// SweepScheduleInput sets the USD amount to sweep.
type SweepScheduleInput struct {
	USDAmount string `json:"usdAmount" jsonschema:"USD amount to sweep as a decimal string, e.g. 100.00"`
}

// SweepCancelInput has no fields; cancelling the pending sweep takes no
// parameters.
type SweepCancelInput struct{}

// MarginSettingInput has no fields; reading the margin setting takes no
// parameters.
type MarginSettingInput struct{}

// MarginSettingSetInput selects the intraday margin setting.
type MarginSettingSetInput struct {
	Setting string `json:"setting" jsonschema:"margin setting: INTRADAY_MARGIN_SETTING_STANDARD or INTRADAY_MARGIN_SETTING_INTRADAY"`
}

// MarginWindowInput optionally selects a margin profile to query.
type MarginWindowInput struct {
	MarginProfileType string `json:"marginProfileType,omitempty" jsonschema:"margin profile to query, e.g. MARGIN_PROFILE_TYPE_RETAIL_INTRADAY_MARGIN_1 (optional)"`
}

// --- Tool handlers ---

func (s *service) balance(ctx context.Context, _ *mcp.CallToolRequest, _ BalanceInput) (*mcp.CallToolResult, *BalanceSummary, error) {
	out, err := s.GetBalanceSummary(ctx)
	return nil, out, err
}

func (s *service) positions(ctx context.Context, _ *mcp.CallToolRequest, _ PositionsInput) (*mcp.CallToolResult, server.ListResult[Position], error) {
	out, err := s.ListPositions(ctx)
	return nil, server.List(out), err
}

func (s *service) position(ctx context.Context, _ *mcp.CallToolRequest, in PositionInput) (*mcp.CallToolResult, *Position, error) {
	out, err := s.GetPosition(ctx, in.ProductID)
	return nil, out, err
}

func (s *service) sweeps(ctx context.Context, _ *mcp.CallToolRequest, _ SweepsInput) (*mcp.CallToolResult, server.ListResult[Sweep], error) {
	out, err := s.ListSweeps(ctx)
	return nil, server.List(out), err
}

func (s *service) sweepSchedule(ctx context.Context, _ *mcp.CallToolRequest, in SweepScheduleInput) (*mcp.CallToolResult, *SweepScheduled, error) {
	out, err := s.ScheduleSweep(ctx, in.USDAmount)
	return nil, out, err
}

func (s *service) sweepCancel(ctx context.Context, _ *mcp.CallToolRequest, _ SweepCancelInput) (*mcp.CallToolResult, *SweepCancelled, error) {
	out, err := s.CancelSweep(ctx)
	return nil, out, err
}

func (s *service) marginSetting(ctx context.Context, _ *mcp.CallToolRequest, _ MarginSettingInput) (*mcp.CallToolResult, *MarginSetting, error) {
	out, err := s.GetMarginSetting(ctx)
	return nil, out, err
}

func (s *service) marginSettingSet(ctx context.Context, _ *mcp.CallToolRequest, in MarginSettingSetInput) (*mcp.CallToolResult, *MarginSettingUpdated, error) {
	out, err := s.SetMarginSetting(ctx, in.Setting)
	return nil, out, err
}

func (s *service) marginWindow(ctx context.Context, _ *mcp.CallToolRequest, in MarginWindowInput) (*mcp.CallToolResult, *MarginWindow, error) {
	out, err := s.GetMarginWindow(ctx, in.MarginProfileType)
	return nil, out, err
}
