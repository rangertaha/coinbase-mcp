# Toolsets

Pass any subset to `COINBASE_TOOLSETS` (e.g. `COINBASE_TOOLSETS=products,orders`). Names are case-insensitive; `all` (the default) enables everything.

| Toolset      | Auth    | Covers                                                              |
| ------------ | ------- | ------------------------------------------------------------------- |
| `products`   | public  | market data: products, candles, recent trades, order book, server time |
| `accounts`   | CDP key | trading accounts (wallets) with available and held balances          |
| `orders`     | CDP key | place/preview/edit/cancel orders, order history, fills               |
| `portfolios` | CDP key | portfolios: list/create/rename/delete, breakdowns, moving funds      |
| `convert`    | CDP key | conversions between accounts (e.g. USD ⇄ USDC): quote, commit, status |
| `fees`       | CDP key | trading volume, fees paid, and the current fee tier                  |
| `payments`   | CDP key | linked payment methods (funding sources)                             |
| `futures`    | CDP key | CFM US futures: balances, positions, sweeps, margin settings         |
| `perpetuals` | CDP key | INTX perpetuals: portfolio, positions, balances, collateral          |
| `keys`       | CDP key | permissions of the API key in use                                    |

Authenticated toolsets are skipped automatically (with a stderr note) when no credentials are configured, so an unauthenticated server only ever advertises tools that work.

Authenticated toolsets need a CDP API key from the [Coinbase Developer Platform](https://portal.cdp.coinbase.com/): every request is signed with a short-lived JWT (ES256 for ECDSA keys, EdDSA for Ed25519 keys) built by [`internal/coinbase/jwt.go`](https://github.com/rangertaha/coinbase-mcp/blob/main/internal/coinbase/jwt.go).

See the full [Tools](tools.md) list for the individual tools in each toolset.

## Roadmap

All Advanced Trade REST endpoint groups are implemented. Remaining ideas, in rough order:

- WebSocket market-data streaming (live tickers as MCP resources).
- Richer multi-step prompts built on the order and portfolio tools.
