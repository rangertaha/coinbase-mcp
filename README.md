# Coinbase MCP Server (coinbase-mcp)

[![CI](https://github.com/rangertaha/coinbase-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/rangertaha/coinbase-mcp/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/rangertaha/coinbase-mcp.svg)](https://pkg.go.dev/github.com/rangertaha/coinbase-mcp)
[![Go Version](https://img.shields.io/github/go-mod/go-version/rangertaha/coinbase-mcp)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A **Coinbase** [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server, written in Go, that exposes the Coinbase Advanced Trade API as tools an LLM client (Claude Desktop/Code, Cursor, and others) can call. See the [**Documentation**](https://rangertaha.github.io/coinbase-mcp/) for more details.

**In this README**, roughly shallow to deep: [Install](#install) → [Features](#features) → [Configuration](#configuration) → [CLI](#cli) → [Toolsets](#toolsets) → [Tools](#tools) → [Prompts](#prompts-workflows) → [Architecture](#architecture) → [Development](#development) → [Changelog](#changelog). The [docs site](https://rangertaha.github.io/coinbase-mcp/) mirrors this same path as separate pages, if you'd rather not scroll.

## Install

```sh
go install github.com/rangertaha/coinbase-mcp/cmd/coinbase@latest
```

This puts a `coinbase` binary in your `$GOBIN`.

<details>
<summary><strong>Build from source</strong></summary>

```sh
git clone https://github.com/rangertaha/coinbase-mcp
cd coinbase-mcp
make build        # produces ./bin/coinbase
```

</details>

Releases are tag-triggered ([GoReleaser](https://goreleaser.com) via CI) and publish prebuilt archives for macOS, Linux, and Windows (amd64/arm64) on the [releases page](https://github.com/rangertaha/coinbase-mcp/releases) once the first version is tagged.

Because market data is public, the server works out of the box with **no credentials at all** — install it, wire it into your MCP client, and ask about prices.

## Features

- **Public by default**: the market-data tools need no credentials; point an MCP client at the binary and it works.
- **Typed tools with schemas**: every tool has an auto-generated JSON Schema for its input and output, inferred from Go structs, with per-field descriptions. Inputs are validated before a handler runs.
- **Read-only switch**: `COINBASE_READONLY=true` hides every mutating tool (order placement, cancels, portfolio changes, …); a view-only CDP key (can_trade=false) forces the same automatically at registration time, so the server can be safely pointed at a funded account.
- **Toolset filtering**: enable only the areas you need with `COINBASE_TOOLSETS` to keep the tool list focused.
- **Built on the official SDK**: uses [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) (v1).
- **`.env` support**: for local development, a `.env` file in the working directory is loaded on startup; real environment variables take precedence.

## Configuration

All configuration is read from the environment. **Every variable is optional** — with nothing set, the server serves public market data.

| Variable              | Required | Description                                                              |
| --------------------- | :------: | ------------------------------------------------------------------------ |
| `COINBASE_API_KEY`    |    no    | CDP API key name from the [Coinbase Developer Platform](https://portal.cdp.coinbase.com/). Enables the authenticated toolsets (accounts, orders, portfolios, …); set both key and secret or neither. Every request is signed with a short-lived CDP JWT (ES256 or Ed25519 keys). |
| `COINBASE_API_SECRET` |    no    | CDP API private key.                                                     |
| `COINBASE_BASE_URL`   |    no    | API base URL (default `https://api.coinbase.com`).                       |
| `COINBASE_TOOLSETS`   |    no    | Comma-separated toolset names to enable, or `all` (default).             |
| `COINBASE_TOOLS`      |    no    | Comma-separated individual tool names to allowlist within the enabled toolsets. |
| `COINBASE_READONLY`   |    no    | Truthy (`1`, `true`, `yes`, `on`) to expose only read-only tools.        |

Setting only one of `COINBASE_API_KEY`/`COINBASE_API_SECRET` is rejected at startup — a half-set pair is almost certainly a mistake.

### Use with Claude Desktop / Claude Code

Add to your MCP client configuration (e.g. `claude_desktop_config.json`); the `env` block is only needed once authenticated toolsets exist:

```json
{
  "mcpServers": {
    "coinbase": {
      "command": "coinbase",
      "args": ["mcp"]
    }
  }
}
```

For Claude Code: `claude mcp add coinbase -- coinbase mcp`.

### Local development

The repo ships a committed [`.mcp.json`](.mcp.json) that runs the server straight from source (`go run ./cmd/coinbase mcp`), so changes take effect on the next session without a build step. It reads credentials from your environment (no secrets in the repo). If you need credentials, run `cp .env.example .env` and fill it in — the server loads `.env` from the working directory on startup.

## CLI

`coinbase` is a small command tree (built on urfave/cli). A bare `coinbase` with no subcommand is equivalent to `coinbase mcp`.

- `coinbase mcp`: run the MCP server over stdio. This is what MCP clients (Claude Desktop/Code, Cursor) invoke.
- `coinbase test`: verify connectivity against the Coinbase API (requests one public product and reports how many products are available):

  ```
  $ coinbase test
  OK  connected to https://api.coinbase.com (792 products available)
      authenticated=false read-only=false
  ```

## Tools

Tools follow the naming convention `<toolset>_<action>`. Mutating tools are marked **[write]** (and **[destructive]** where they cancel or delete) and are hidden when `COINBASE_READONLY=true`.

### products (public)

- `products_list`: List tradeable Coinbase products (markets) with current price and 24h stats. Optionally filter by product type (e.g. `SPOT` or `FUTURE`) and cap the number returned.
- `products_get`: Get a single Coinbase product by ID, e.g. `BTC-USD` — price, 24h percentage change, 24h volume, base/quote names, status, and whether trading is enabled.
- `products_candles`: Get OHLCV price history (candles) for a product over a time range, at granularities from one minute to one day.
- `products_ticker`: Get recent market trades and the current best bid/ask for a product.
- `products_book`: Get the order book (bids and asks) for a product, optionally limited to the top N levels.
- `products_time`: Get the Coinbase API server time (ISO and UNIX epoch).
- `products_best_bid_ask`: Get the current best bid/ask (top of book) for several products in one call (needs credentials).

### accounts

- `accounts_list`: List the authenticated user's trading accounts (wallets) with available and held balances; cursor-paginated.
- `accounts_get`: Get a single trading account by its UUID.

### orders

- `orders_create` **[write]**: Place an order. Types: `market` (BUY takes `quoteSize`, SELL takes `baseSize`), `limit`, `limit_fok`, `sor_limit`, `stop_limit` (`stopPrice` + `stopDirection`), and `bracket` (`stopTriggerPrice`); non-market types take `baseSize` and `limitPrice`, with optional `endTime` for good-til-date. `takeProfitPrice`+`stopLossPrice` attach a TP/SL bracket that exits when the parent fills. Real funds move — preview first.
- `orders_preview`: Simulate placing an order — projected total, fees, slippage, and any errors — without placing it.
- `orders_edit` **[write]**: Edit the price and size of an existing open limit order.
- `orders_edit_preview`: Simulate an order edit and get the projected outcome without changing the order.
- `orders_cancel` **[write, destructive]**: Cancel one or more open orders by ID, with per-order results.
- `orders_close_position` **[write, destructive]**: Place an order that closes an open position, optionally partially.
- `orders_list`: List historical orders, filterable by product, status, and side; cursor-paginated.
- `orders_get`: Get a single order by ID, including status and fill details.
- `orders_fills`: List fills (executions), filterable by order and/or product; cursor-paginated.

### portfolios

- `portfolios_list`: List portfolios, optionally filtered by type.
- `portfolios_get`: Get a portfolio's breakdown: aggregate balances and spot positions.
- `portfolios_create` **[write]**: Create a new portfolio.
- `portfolios_edit` **[write]**: Rename a portfolio.
- `portfolios_delete` **[write, destructive]**: Delete a portfolio by UUID.
- `portfolios_move_funds` **[write]**: Move an amount of a currency between portfolios.

### convert

- `convert_quote` **[write]**: Create a quote to convert an amount between two accounts (e.g. USD to USDC).
- `convert_commit` **[write]**: Commit a previously created quote, executing the conversion.
- `convert_get`: Get the status of a convert trade by ID.

### fees

- `fees_summary`: Get trading volume, fees paid, and the current maker/taker fee tier.

### payments

- `payments_list`: List linked payment methods and what each allows (buy/sell/deposit/withdraw).
- `payments_get`: Get a single payment method by ID.

### futures (CFM)

- `futures_balance`: Futures balance summary: buying power, margin, liquidation levels.
- `futures_positions`: List all open US futures positions.
- `futures_position`: Get the open futures position for one product.
- `futures_sweeps`: List pending/processing USD sweeps between futures and spot.
- `futures_sweep_schedule` **[write]**: Schedule a USD sweep to the spot wallet.
- `futures_sweep_cancel` **[write, destructive]**: Cancel the pending sweep.
- `futures_margin_setting`: Get the intraday margin setting.
- `futures_margin_setting_set` **[write]**: Set the intraday margin setting.
- `futures_margin_window`: Get the current margin window and killswitch states.

### perpetuals (INTX)

- `perpetuals_portfolio`: Portfolio summary: collateral, margin, liquidation state.
- `perpetuals_positions`: List all open perpetuals positions in a portfolio.
- `perpetuals_position`: Get the open position for a single symbol.
- `perpetuals_balances`: Asset balances of a perpetuals portfolio.
- `perpetuals_allocate` **[write]**: Allocate collateral to an isolated position.
- `perpetuals_multi_asset_collateral` **[write]**: Enable/disable multi-asset collateral.

### keys

- `keys_permissions`: Get the permissions of the API key in use (view/trade/transfer) and its portfolio scope.

## Prompts (workflows)

Prompts are user-invoked templates that MCP clients surface as **slash commands** (e.g. in Claude Code and Claude Desktop). Each guides the model through a sequence of tool calls. Built-in prompts:

| Prompt            | Arguments | What it does                                                              |
| ----------------- | --------- | ------------------------------------------------------------------------- |
| `market_snapshot` | product   | Load a product's price, 24h change, and volume, then summarize the market |

## Architecture

<details>
<summary>Project layout</summary>

```
cmd/coinbase              entrypoint: a urfave/cli command tree (mcp, test)
internal/config           environment configuration + validation, .env loading
internal/client           generic JSON REST client (pluggable auth, error mapping)
internal/server           MCP server wrapper, typed tool registration, read-only policy
internal/coinbase         Coinbase API connection (shared REST client) + connectivity check
internal/coinbase/<area>  one package per API area: <area>.go (operations) + tools.go (tools)
internal/prompts          built-in workflow prompts
internal/app              assembles the configured server from config
```

The `mcp` subcommand loads config, registers the enabled toolsets, and serves over stdio; `test` checks connectivity.

Each API area follows the same shape: a `service` wrapping the shared REST client exposes typed operations, and a `Register` function registers thin MCP tool handlers for them. Adding a new area is a matter of dropping in a package and listing it in `internal/app`. See the [docs site's Architecture page](https://rangertaha.github.io/coinbase-mcp/architecture/) for the full request-flow walkthrough.

</details>

## Development

<details>
<summary>Build, test, and smoke-test</summary>

```sh
make test        # go test -race ./...
make cover       # run tests and print a coverage summary
make vet         # go vet ./...
make fmt-check   # gofmt verification
make lint        # golangci-lint
make all         # fmt-check + vet + lint + test + build
```

Releases are tag-triggered (GoReleaser via CI); `make next`/`make bump` compute and tag the next version from conventional commits. See the [docs site's Development page](https://rangertaha.github.io/coinbase-mcp/development/) for the full releasing workflow.

### Smoke-testing the protocol

List the tools over stdio without an MCP client (no credentials needed):

```sh
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"s","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
| ./bin/coinbase mcp
```

Or browse interactively with the [MCP Inspector](https://github.com/modelcontextprotocol/inspector):

```sh
npx @modelcontextprotocol/inspector ./bin/coinbase mcp
```

</details>

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## License

MIT. See [LICENSE](LICENSE).
