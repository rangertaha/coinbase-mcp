# coinbase-mcp

[![CI](https://github.com/rangertaha/coinbase-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/rangertaha/coinbase-mcp/actions/workflows/ci.yml)

A **Coinbase** [Model Context Protocol](https://modelcontextprotocol.io) (MCP) server, written in Go, that exposes the Coinbase Advanced Trade API as tools an LLM client (Claude Desktop/Code, Cursor, and others) can call.

!!! warning "Under construction"
    This server is a work in progress. It runs over stdio with **10 toolsets (46 tools)** covering the Advanced Trade API — public market data plus authenticated accounts, orders, portfolios, convert, fees, payments, futures, and perpetuals. The authenticated toolsets are new and not yet battle-tested against live accounts; tool names may still change.

Because market data is public, the server works out of the box with **no credentials at all** — install it, wire it into your MCP client, and ask about prices.

## Features

- **Public by default**: the market-data tools need no credentials.
- **Typed tools with schemas**: every tool has an auto-generated JSON Schema for its input and output, inferred from Go structs, with per-field descriptions. Inputs are validated before a handler runs.
- **Read-only switch**: `COINBASE_READONLY=true` hides every mutating tool (order placement, cancels, portfolio changes, …); a view-only CDP key (can_trade=false) forces the same automatically, so the server can be safely pointed at a funded account.
- **Toolset filtering**: enable only the areas you need with `COINBASE_TOOLSETS` to keep the tool list focused.
- **Built on the official SDK**: uses [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) (v1).
- **`.env` support**: for local development, a `.env` file in the working directory is loaded on startup; real environment variables take precedence.

## Documentation map

Each page goes one level deeper than the last:

1. [Install](install.md) — get the `coinbase` binary onto your machine.
2. [Configuration](configuration.md) — environment variables and MCP client setup.
3. [CLI](cli.md) — the `coinbase mcp` / `coinbase test` command tree.
4. [Toolsets](toolsets.md) — the ten toolsets and what each covers.
5. [Tools](tools.md) — every individual tool, with its full description.
6. [Prompts](prompts.md) — the built-in slash-command workflows.
7. [Architecture](architecture.md) — how the server is put together internally.
8. [Development](development.md) — building, testing, linting, and smoke-testing the protocol.

New to the project? Start at [Install](install.md) and follow the numbers down. Looking for one specific tool? Jump straight to [Tools](tools.md) — every page stands on its own.
