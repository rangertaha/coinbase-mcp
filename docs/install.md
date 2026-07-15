# Install

Install with Go:

```sh
go install github.com/rangertaha/coinbase-mcp/cmd/coinbase@latest
```

This puts a `coinbase` binary in your `$GOBIN` (make sure it is on your `PATH`).

## Alternative: build from source

```sh
git clone https://github.com/rangertaha/coinbase-mcp
cd coinbase-mcp
make build        # produces ./bin/coinbase
```

See [Development](development.md) for the full build/test/lint workflow if you're contributing.

## Prebuilt binaries

Releases are tag-triggered ([GoReleaser](https://goreleaser.com) via CI) and publish prebuilt archives for macOS, Linux, and Windows (amd64/arm64), with a `checksums.txt`, on the [releases page](https://github.com/rangertaha/coinbase-mcp/releases) once the first version is tagged.

## Next: configure it

Once `coinbase` is on your `PATH`, head to [Configuration](configuration.md) to wire up your MCP client (and, optionally, credentials).
