// SPDX-License-Identifier: MIT

package coinbase

import (
	"context"
)

// Check verifies connectivity by listing public products. It returns the total
// number of tradeable products. The list must be fetched unfiltered: the API's
// num_products field counts the products in the response, so requesting with a
// limit would report the limit, not the catalog size.
func Check(ctx context.Context, c *Clients) (int, error) {
	var out struct {
		NumProducts int `json:"num_products"`
	}
	if err := c.API.GetJSON(ctx, "/api/v3/brokerage/market/products", nil, &out); err != nil {
		return 0, err
	}
	return out.NumProducts, nil
}
