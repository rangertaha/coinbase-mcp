# CLI

`coinbase` is a small command tree (built on [`urfave/cli`](https://cli.urfave.org/)) with two commands: run the MCP server, and check connectivity. A bare `coinbase` with no subcommand is equivalent to `coinbase mcp`.

Both commands load a `.env` file from the working directory first (see [Configuration](configuration.md)).

## `coinbase mcp`

Run the MCP server over stdio. This is what MCP clients (Claude Desktop/Code, Cursor) invoke — see [Configuration](configuration.md) for client setup.

```sh
coinbase mcp
```

On startup it logs (to stderr — stdout is reserved for the MCP protocol) the version, tool/prompt counts, enabled toolsets, and read-only state.

## `coinbase test`

Verify connectivity against the Coinbase API: requests one public product and reports how many products the API says are available. Useful for confirming network access (and, later, credentials) before wiring up an MCP client. No credentials are required.

```sh
$ coinbase test
OK  connected to https://api.coinbase.com (792 products available)
    authenticated=false read-only=false
```

`authenticated` reports whether `COINBASE_API_KEY`/`COINBASE_API_SECRET` are set; `read-only` reports the `COINBASE_READONLY` setting.

## Next: browse the toolsets

The MCP server itself is organized into [Toolsets](toolsets.md), each exposing a set of [Tools](tools.md).
