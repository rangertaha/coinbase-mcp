# Configuration

All configuration is read from the environment.

| Variable              | Required | Description                                          |
| --------------------- | :------: | ---------------------------------------------------- |
| `COINBASE_API_KEY`    |    no    | CDP API key name (enables authenticated tools).      |
| `COINBASE_API_SECRET` |    no    | CDP API private key.                                 |
| `COINBASE_BASE_URL`   |    no    | API base URL (default `https://api.coinbase.com`).   |
| `COINBASE_TOOLSETS`   |    no    | Comma-separated toolset names to enable, or `all`.   |
| `COINBASE_READONLY`   |    no    | `true` to expose only read-only tools.               |

Public market-data tools work without credentials.
