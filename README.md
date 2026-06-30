# coinbase-mcp

[![CI](https://github.com/rangertaha/coinbase-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/rangertaha/coinbase-mcp/actions/workflows/ci.yml)
[![Status: under construction](https://img.shields.io/badge/status-under%20construction-orange)](#-under-construction)

<div align="center">

## 🚧 &nbsp; UNDER CONSTRUCTION &nbsp; 🚧

**This server is an early scaffold — a work in progress.**

It runs over stdio with **one read-only toolset** wired end-to-end.<br>
More toolsets are on the way (see the **TODO** list below).<br>
APIs, configuration, and tool names may still change.

</div>

---

A [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server, written
in Go, exposing the **Coinbase** Advanced Trade API as tools an LLM client
(Claude Desktop/Code, Cursor, and others) can call.

## Features

- **Typed tools with schemas**: every tool has an auto-generated JSON Schema for
  its input and output, inferred from Go structs.
- **Read-only switch**: `COINBASE_READONLY=true` hides every mutating tool.
- **Toolset filtering**: enable only the areas you need with `COINBASE_TOOLSETS`.
- **Public by default**: market-data tools need no credentials.

## Install

```sh
go install github.com/rangertaha/coinbase-mcp/cmd/coinbase@latest
```

Or build from source:

```sh
git clone https://github.com/rangertaha/coinbase-mcp
cd coinbase-mcp
make build        # produces ./bin/coinbase
```

## CLI

```sh
coinbase mcp      # run the MCP server over stdio (default when no subcommand)
coinbase test     # verify connectivity
```

## Configuration

| Variable              | Required | Description                                                |
| --------------------- | :------: | ---------------------------------------------------------- |
| `COINBASE_API_KEY`    |    no    | CDP API key name (enables authenticated tools).            |
| `COINBASE_API_SECRET` |    no    | CDP API private key.                                       |
| `COINBASE_BASE_URL`   |    no    | API base URL (default `https://api.coinbase.com`).         |
| `COINBASE_TOOLSETS`   |    no    | Comma-separated toolset names to enable, or `all`.         |
| `COINBASE_READONLY`   |    no    | `true` to expose only read-only tools.                     |

## Toolsets

| Toolset    | Covers                                                       |
| ---------- | ------------------------------------------------------------ |
| `products` | list tradeable products (`products_list`) and get one (`products_get`) — public market data |

### TODO toolsets

- `accounts` — wallet balances (needs CDP JWT auth).
- `orders` — list/create/cancel orders (write; needs auth).
- `fills` — trade history (needs auth).

> Authenticated toolsets require a CDP JWT `client.Authorizer`; see the TODO in
> `internal/coinbase/coinbase.go`.

## License

MIT — see [LICENSE](LICENSE).
