# Prompts (workflows)

Prompts are user-invoked, parameterized templates that MCP clients surface as **slash commands** (e.g. in Claude Code and Claude Desktop). Each guides the model through a sequence of tool calls. Built-in prompts:

| Prompt            | Arguments          | What it does                                                              |
| ----------------- | ------------------ | ------------------------------------------------------------------------- |
| `market_snapshot` | product (required) | Load a product's price, 24h change, and volume via `products_get`, then summarize the market |

Once the server is connected, each prompt is available as a slash command named after the prompt.

## Next: how it's built

That's the full user-facing surface. See [Architecture](architecture.md) for how the server itself is put together.
