# Tools

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

Every tool returns a trimmed, LLM-focused object (or a `{count, items}` list) rather than the raw API response.

## Next: prompts

Multi-step flows are shipped as [Prompts](prompts.md), surfaced by MCP clients as slash commands.
