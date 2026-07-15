// SPDX-License-Identifier: MIT

// Package fees exposes the Coinbase Advanced Trade transaction summary: the
// authenticated user's trailing 30-day volume, fees paid, and current fee tier.
package fees

import (
	"context"
	"net/url"
	"strings"

	"github.com/rangertaha/coinbase-mcp/internal/coinbase"
)

// Name is the toolset name used for enable/disable filtering.
const Name = "fees"

// service wraps the Coinbase clients for fee operations.
type service struct {
	c *coinbase.Clients
}

// FeeTier is the user's current pricing tier. Rates and bounds are decimal
// strings.
type FeeTier struct {
	PricingTier  string `json:"pricing_tier,omitempty"`
	USDFrom      string `json:"usd_from,omitempty"`
	USDTo        string `json:"usd_to,omitempty"`
	TakerFeeRate string `json:"taker_fee_rate,omitempty"`
	MakerFeeRate string `json:"maker_fee_rate,omitempty"`
	AOPFrom      string `json:"aop_from,omitempty"`
	AOPTo        string `json:"aop_to,omitempty"`
}

// MarginRate is a decimal rate wrapped in the API's Decimal envelope.
type MarginRate struct {
	Value string `json:"value"`
}

// Tax is the goods-and-services tax applied to fees, where applicable.
type Tax struct {
	Rate string `json:"rate,omitempty"`
	Type string `json:"type,omitempty"`
}

// Summary is the fee/volume transaction summary, trimmed to the fields useful
// to an LLM. Volume and fee totals are USD-denominated JSON numbers; fee-tier
// rates are decimal strings.
type Summary struct {
	TotalVolume             float64     `json:"total_volume"`
	TotalFees               float64     `json:"total_fees"`
	FeeTier                 FeeTier     `json:"fee_tier"`
	MarginRate              *MarginRate `json:"margin_rate,omitempty"`
	GoodsAndServicesTax     *Tax        `json:"goods_and_services_tax,omitempty"`
	AdvancedTradeOnlyVolume float64     `json:"advanced_trade_only_volume,omitempty"`
	AdvancedTradeOnlyFees   float64     `json:"advanced_trade_only_fees,omitempty"`
	CoinbaseProVolume       float64     `json:"coinbase_pro_volume,omitempty"`
	CoinbaseProFees         float64     `json:"coinbase_pro_fees,omitempty"`
}

// GetSummary returns the transaction summary, optionally filtered by product
// type (e.g. SPOT, FUTURE), venue (e.g. CBE, FCM, INTX), and contract expiry
// type (e.g. EXPIRING, PERPETUAL). All filters are optional.
func (s *service) GetSummary(ctx context.Context, productType, productVenue, contractExpiryType string) (*Summary, error) {
	q := url.Values{}
	if v := strings.TrimSpace(productType); v != "" {
		q.Set("product_type", v)
	}
	if v := strings.TrimSpace(productVenue); v != "" {
		q.Set("product_venue", v)
	}
	if v := strings.TrimSpace(contractExpiryType); v != "" {
		q.Set("contract_expiry_type", v)
	}
	var out Summary
	if err := s.c.API.GetJSON(ctx, "/api/v3/brokerage/transaction_summary", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
