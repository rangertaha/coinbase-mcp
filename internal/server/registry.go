// SPDX-License-Identifier: MIT

package server

import "github.com/rangertaha/coinbase-mcp/internal/coinbase"

// Toolset pairs a toolset name with the function that registers its tools.
// Each service area exposes one of these so the entrypoint can register only
// the toolsets enabled by configuration.
type Toolset struct {
	Name     string
	Register func(s *Server, c *coinbase.Clients)
	// Auth marks toolsets whose every tool needs API credentials. They are
	// skipped when the server runs unauthenticated, so the model never sees
	// tools that can only fail.
	Auth bool
}
