# Configuration

All configuration is read from the environment. **Every variable is optional** — with nothing set, the server serves public market data.

| Variable              | Required | Description                                                              |
| --------------------- | :------: | ------------------------------------------------------------------------ |
| `COINBASE_API_KEY`    |    no    | CDP API key name from the [Coinbase Developer Platform](https://portal.cdp.coinbase.com/). Will enable authenticated account/order toolsets; set both key and secret or neither. |
| `COINBASE_API_SECRET` |    no    | CDP API private key.                                                     |
| `COINBASE_BASE_URL`   |    no    | API base URL (default `https://api.coinbase.com`).                       |
| `COINBASE_TOOLSETS`   |    no    | Comma-separated toolset names to enable, or `all` (default).             |
| `COINBASE_TOOLS`      |    no    | Comma-separated individual tool names to allowlist within the enabled toolsets. |
| `COINBASE_READONLY`   |    no    | Truthy (`1`, `true`, `yes`, `on`) to expose only read-only tools.        |

Two validation rules apply at startup:

- Setting only one of `COINBASE_API_KEY`/`COINBASE_API_SECRET` is rejected — a half-set pair is almost certainly a mistake.
- `COINBASE_BASE_URL` must be a valid absolute URL.

## Use with Claude Desktop / Claude Code

Add to your MCP client configuration (e.g. `claude_desktop_config.json`); an `env` block is only needed once authenticated toolsets exist:

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

## Local development

The repo ships a committed [`.mcp.json`](https://github.com/rangertaha/coinbase-mcp/blob/main/.mcp.json) that runs the server straight from source (`go run ./cmd/coinbase mcp`), so changes take effect on the next session without a build step. It reads credentials from your environment (no secrets in the repo). If you need credentials, run `cp .env.example .env` and fill it in — the server loads `.env` from the working directory on startup, and real environment variables take precedence over the file.

## Next: the CLI

With the client wired up, see the [CLI](cli.md) reference for `coinbase test` (verify the connection).
