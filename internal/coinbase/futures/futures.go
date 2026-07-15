// SPDX-License-Identifier: MIT

// Package futures exposes the Coinbase Advanced Trade CFM (US-regulated
// futures) endpoints: balance summary, positions, USD sweeps, and intraday
// margin settings.
package futures

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "futures"

// service wraps the Coinbase clients for futures operations.
type service struct {
	c *coinbase.Clients
}

// Amount is a money value paired with its currency.
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// BalanceSummary is the CFM futures balance overview, trimmed to the fields
// useful to an LLM. All monetary values are decimal strings.
type BalanceSummary struct {
	FuturesBuyingPower        Amount `json:"futures_buying_power"`
	TotalUSDBalance           Amount `json:"total_usd_balance"`
	CBIUSDBalance             Amount `json:"cbi_usd_balance"`
	CFMUSDBalance             Amount `json:"cfm_usd_balance"`
	TotalOpenOrdersHoldAmount Amount `json:"total_open_orders_hold_amount"`
	UnrealizedPnL             Amount `json:"unrealized_pnl"`
	DailyRealizedPnL          Amount `json:"daily_realized_pnl"`
	InitialMargin             Amount `json:"initial_margin"`
	AvailableMargin           Amount `json:"available_margin"`
	LiquidationThreshold      Amount `json:"liquidation_threshold"`
	LiquidationBufferAmount   Amount `json:"liquidation_buffer_amount"`
	LiquidationBufferPct      string `json:"liquidation_buffer_percentage,omitempty"`
}

// GetBalanceSummary returns the CFM futures balance summary.
func (s *service) GetBalanceSummary(ctx context.Context) (*BalanceSummary, error) {
	var out struct {
		BalanceSummary BalanceSummary `json:"balance_summary"`
	}
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/cfm/balance_summary", nil, &out); err != nil {
		return nil, err
	}
	return &out.BalanceSummary, nil
}

// Position is one open CFM futures position. All numeric fields are decimal
// strings.
type Position struct {
	ProductID         string `json:"product_id"`
	ExpirationTime    string `json:"expiration_time,omitempty"`
	Side              string `json:"side,omitempty"`
	NumberOfContracts string `json:"number_of_contracts,omitempty"`
	CurrentPrice      string `json:"current_price,omitempty"`
	AvgEntryPrice     string `json:"avg_entry_price,omitempty"`
	UnrealizedPnL     string `json:"unrealized_pnl,omitempty"`
	DailyRealizedPnL  string `json:"daily_realized_pnl,omitempty"`
}

// ListPositions returns all open CFM futures positions.
func (s *service) ListPositions(ctx context.Context) ([]Position, error) {
	var out struct {
		Positions []Position `json:"positions"`
	}
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/cfm/positions", nil, &out); err != nil {
		return nil, err
	}
	return out.Positions, nil
}

// GetPosition returns the open CFM futures position for one product.
func (s *service) GetPosition(ctx context.Context, productID string) (*Position, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, errors.New(`productId is required (e.g. "BIT-31OCT25-CDE")`)
	}
	var out struct {
		Position Position `json:"position"`
	}
	path := fmt.Sprintf("/api/v3/brokerage/cfm/positions/%s", url.PathEscape(productID))
	if err := s.c.API.GetJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out.Position, nil
}

// Sweep is a pending or processing USD sweep between the Coinbase Inc. spot
// wallet and the CFM futures account.
type Sweep struct {
	ID              string `json:"id"`
	RequestedAmount Amount `json:"requested_amount"`
	ShouldSweepAll  bool   `json:"should_sweep_all"`
	Status          string `json:"status,omitempty"`
	ScheduledTime   string `json:"scheduled_time,omitempty"`
}

// ListSweeps returns pending and processing USD sweeps.
func (s *service) ListSweeps(ctx context.Context) ([]Sweep, error) {
	var out struct {
		Sweeps []Sweep `json:"sweeps"`
	}
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/cfm/sweeps", nil, &out); err != nil {
		return nil, err
	}
	return out.Sweeps, nil
}

// SweepScheduled confirms a sweep schedule request.
type SweepScheduled struct {
	Scheduled bool   `json:"scheduled"`
	USDAmount string `json:"usd_amount"`
}

// ScheduleSweep schedules a USD sweep from the CFM futures account to the
// Coinbase Inc. spot wallet.
func (s *service) ScheduleSweep(ctx context.Context, usdAmount string) (*SweepScheduled, error) {
	usdAmount = strings.TrimSpace(usdAmount)
	if usdAmount == "" {
		return nil, errors.New(`usdAmount is required (e.g. "100.00")`)
	}
	body := struct {
		USDAmount string `json:"usd_amount"`
	}{USDAmount: usdAmount}
	var out struct {
		Success bool `json:"success"`
	}
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/cfm/sweeps/schedule", nil, body, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, errors.New("sweep was not scheduled (the API reported success=false)")
	}
	return &SweepScheduled{Scheduled: true, USDAmount: usdAmount}, nil
}

// SweepCancelled confirms a sweep cancellation.
type SweepCancelled struct {
	Cancelled bool `json:"cancelled"`
}

// CancelSweep cancels the pending USD sweep, if any.
func (s *service) CancelSweep(ctx context.Context) (*SweepCancelled, error) {
	var out struct {
		Success bool `json:"success"`
	}
	if err := s.c.API.Delete(ctx, "/api/v3/brokerage/cfm/sweeps", nil, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, errors.New("sweep was not cancelled (the API reported success=false); there may be no pending sweep")
	}
	return &SweepCancelled{Cancelled: true}, nil
}

// MarginSetting is the account's intraday margin setting.
type MarginSetting struct {
	Setting string `json:"setting"`
}

// GetMarginSetting returns the current intraday margin setting.
func (s *service) GetMarginSetting(ctx context.Context) (*MarginSetting, error) {
	var out MarginSetting
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/cfm/intraday/margin_setting", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MarginSettingUpdated confirms an intraday margin setting change.
type MarginSettingUpdated struct {
	Updated bool   `json:"updated"`
	Setting string `json:"setting"`
}

// SetMarginSetting updates the intraday margin setting.
func (s *service) SetMarginSetting(ctx context.Context, setting string) (*MarginSettingUpdated, error) {
	setting = strings.TrimSpace(setting)
	if setting == "" {
		return nil, errors.New(`setting is required (e.g. "INTRADAY_MARGIN_SETTING_STANDARD")`)
	}
	body := struct {
		Setting string `json:"setting"`
	}{Setting: setting}
	var out struct{}
	if err := s.c.API.PostJSON(ctx, "/api/v3/brokerage/cfm/intraday/margin_setting", nil, body, &out); err != nil {
		return nil, err
	}
	return &MarginSettingUpdated{Updated: true, Setting: setting}, nil
}

// MarginWindowDetail describes the margin window currently in effect.
type MarginWindowDetail struct {
	MarginWindowType string `json:"margin_window_type"`
	EndTime          string `json:"end_time,omitempty"`
}

// MarginWindow is the current margin window plus intraday killswitch states.
type MarginWindow struct {
	MarginWindow              MarginWindowDetail `json:"margin_window"`
	IsKillswitchEnabled       bool               `json:"is_intraday_margin_killswitch_enabled"`
	IsEnrollKillswitchEnabled bool               `json:"is_intraday_margin_enrollment_killswitch_enabled"`
}

// GetMarginWindow returns the current margin window and killswitch states.
// marginProfileType optionally selects the profile to query (e.g.
// MARGIN_PROFILE_TYPE_RETAIL_INTRADAY_MARGIN_1).
func (s *service) GetMarginWindow(ctx context.Context, marginProfileType string) (*MarginWindow, error) {
	var q url.Values
	if marginProfileType = strings.TrimSpace(marginProfileType); marginProfileType != "" {
		q = url.Values{}
		q.Set("margin_profile_type", marginProfileType)
	}
	var out MarginWindow
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/cfm/intraday/current_margin_window", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
