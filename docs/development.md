# Development

See [Architecture](architecture.md) first for how the code is laid out; this page covers building, testing, and exercising the server directly.

## Build, test, and lint

```sh
make build       # compile ./bin/coinbase
make test        # go test -race ./...
make cover       # run tests and print a coverage summary
make vet         # go vet ./...
make fmt-check   # gofmt verification (make fmt to apply)
make lint        # golangci-lint (config in .golangci.yml)
make all         # fmt-check + vet + lint + test + build, in one pass
```

`make run` builds and runs the binary directly. `make tidy` runs `go mod tidy`, and `make clean` removes `bin/`, `dist/`, and `coverage.out`.

## Smoke-testing the protocol

List the tools over stdio without an MCP client (no credentials needed — the `products` toolset is public):

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

## Releasing

Releases are tag-triggered: pushing a `vX.Y.Z` tag runs [GoReleaser](https://goreleaser.com) in CI to publish archives and a GitHub Release (see [Install](install.md)).

```sh
make next        # print the next version svu would compute, from conventional commits
make bump        # tag that version locally (override with BUMP=major|minor|patch or TAG=vX.Y.Z)
```

Then `git push origin` the tag `make bump` created to trigger the release workflow. `make snapshot` builds release artifacts locally with GoReleaser, without publishing anything.
