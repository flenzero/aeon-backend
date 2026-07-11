# Aeonblight Backend Context Map

The repository contains four independently runnable modules in one Go module.

| Runtime module | Domain context | Context document | Main implementation |
| --- | --- | --- | --- |
| `account-api` | Account and game admission | `docs/contexts/account/CONTEXT.md` | `internal/account` |
| `economy-api` | Durable game economy and chain settlement | `docs/contexts/economy/CONTEXT.md` | `internal/economy`, `internal/chain` |
| `admin-api` | Privileged operations and risk controls | `docs/contexts/admin/CONTEXT.md` | `internal/admin` |
| `economy-worker` | Scheduled economy progression | `docs/contexts/economy-worker/CONTEXT.md` | `cmd/economy-worker` |

All four modules share the platform implementation under `internal/platform` and
the canonical PostgreSQL schema under `migrations/`. The production economy is
global across game servers and uses one PostgreSQL database.
