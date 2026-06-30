// SPDX-License-Identifier: MIT

// Package prompts registers MCP prompts: user-invoked, parameterized templates
// that clients surface as slash commands. Each prompt encodes a multi-step
// workflow by guiding the model to call the right tools in order.
package prompts

import (
	"fmt"

	"github.com/rangertaha/coinbase-mcp/internal/server"
)

// Register adds the built-in workflow prompts to the server.
func Register(s *server.Server) {
	s.AddPrompt(
		"market_snapshot",
		"Summarize the current market for a Coinbase product: price, 24h change, and volume.",
		[]server.PromptArg{
			{Name: "product", Description: "product ID, e.g. BTC-USD", Required: true},
		},
		func(a map[string]string) string {
			return fmt.Sprintf(`Give a market snapshot for "%s".

Steps:
1. Call products_get (productId="%s") to load price, 24h percentage change, and 24h volume.
2. State the current price and whether it is up or down over 24h, with the percentage.
3. Note the 24h volume and whether trading is currently enabled.`,
				a["product"], a["product"])
		},
	)
}
