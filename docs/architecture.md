# Architecture

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

The CLI ([full reference](cli.md)) exposes two commands: `mcp` (load config, register the enabled toolsets, and serve over stdio) and `test` (verify connectivity against the Coinbase API).

Each API area follows the same shape: a `service` wrapping the shared REST client exposes typed operations, and a `Register` function registers thin MCP tool handlers for them. Adding a new area is a matter of dropping in a package under `internal/coinbase/` and listing it in `internal/app`.

## Request flow

1. `cmd/coinbase`'s `mcp` command loads configuration (`internal/config`) and assembles the server (`internal/app`), which registers every enabled toolset's tools against `internal/server`.
2. Each toolset's `tools.go` derives a JSON Schema from its Go input/output structs; `internal/server` validates incoming arguments against it and dispatches to the area's `service` methods. Mutating tools are skipped entirely at registration time when the server runs read-only.
3. `service` methods call out through `internal/client`, a small dependency-free JSON REST client that applies authentication via a pluggable `Authorizer` and maps non-2xx responses to a structured `APIError`, so the model gets an actionable message rather than an opaque status code.
4. `internal/coinbase` owns the `client.Client` bound to the Coinbase API host. Public market-data endpoints run unauthenticated; when CDP credentials are configured, an `Authorizer` will sign requests for the authenticated toolsets (see the [roadmap](toolsets.md#roadmap)).

List-returning tools wrap their results in a `{count, items}` object, because the MCP specification requires a tool's structured output to be a JSON object rather than a bare array.

## Next: contribute

See [Development](development.md) for the build, test, lint, and protocol smoke-testing workflow.
