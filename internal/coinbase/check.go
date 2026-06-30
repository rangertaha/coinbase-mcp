// SPDX-License-Identifier: MIT

package coinbase

import (
	"context"
	"net/url"
)

// Check verifies connectivity by requesting a single public product. It returns
// the number of tradeable products reported by the API.
func Check(ctx context.Context, c *Clients) (int, error) {
	q := url.Values{}
	q.Set("limit", "1")
	var out struct {
		NumProducts int `json:"num_products"`
	}
	if err := c.API.GetJSON(ctx, "/api/v3/brokerage/market/products", q, &out); err != nil {
		return 0, err
	}
	return out.NumProducts, nil
}
