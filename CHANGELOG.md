# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial scaffold: MCP server over stdio with the `products` toolset
  (`products_list`, `products_get`), a `market_snapshot` prompt, and a
  `coinbase test` connectivity check.
- Full Coinbase Advanced Trade API coverage: 10 toolsets (46 tools) —
  public `products` market data plus authenticated `accounts`, `orders`,
  `portfolios`, `convert`, `fees`, `payments`, `futures`, `perpetuals`,
  and `keys`. Authenticated requests are signed with short-lived JWTs
  (ES256/EdDSA); authenticated toolsets are skipped automatically when no
  credentials are configured.
- Unit, fuzz, and stress tests across all packages.
- Documentation site pages: install, configuration, CLI, toolsets, tools,
  prompts, architecture, and development.

### Removed
- "Under construction" notices from the README and docs landing page.
