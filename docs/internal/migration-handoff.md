# Migration Handoff

This document captures the current backend progress and the known Grill-Me
constraints before moving the project into:

```text
/Users/TJ/code/Project/SOLP/game-backend
```

The Codex sandbox for this thread can read that directory but cannot write to
it, so the implementation currently lives in:

```text
/Users/TJ/Documents/Codex/2026-07-10/wo-x
```

## Current Implementation State

Implemented runtime shape:

```text
cmd/account-api
cmd/economy-api
cmd/admin-api
cmd/economy-worker
```

Implemented support packages:

```text
internal/account
internal/admin
internal/chain
internal/economy
internal/platform/config
internal/platform/httpx
internal/platform/security
internal/platform/store
```

Implemented deployment and data files:

```text
deploy/Dockerfile
deploy/docker-compose.yml
migrations/aeonblight_full_schema.sql
docs/internal/architecture.md
docs/internal/chain-solana.md
docs/internal/data-model.md
docs/internal/gamefi-economy-system-v0.2.md
configs/economy/
```

## Key Decisions Already Made

Use one repository with multiple Go binaries.

Reason: services can restart independently in Docker, while the team avoids the
cost of maintaining multiple repositories too early.

Use one production Postgres database.

Reason: account, economy, admin, game-server coordination and worker state need
one durable source of truth. Backups do not count as separate production
databases.

Keep `economy-worker` as a separate process.

Reason: it owns scheduled state transitions and should restart independently
from `economy-api`.

Keep chain logic inside the economy domain for now.

Reason: early team size is small. Code is separated under `internal/chain`, but
runtime remains inside `economy-api` and `economy-worker`.

Default to Solana, not EVM.

Reason: the new game should use Solana wallet addresses, SPL token semantics,
Solana transaction signatures and Solana deposit/payout confirmation rules.

The WebGL client should mainly talk to game servers after login.

Reason: the client has only a session/ticket trust surface. Game servers call
`account-api` and `economy-api` through internal endpoints.

## Grill-Me Constraints Known So Far

1. The backend must support independent Docker restart for `account-api`,
   `economy-api`, `admin-api` and `economy-worker`.

2. The game has multiple game servers running the same code. Game servers are
   gameplay and social/performance partitions, not separate economies.

3. The economic ledger is global across all servers.

4. The client should not directly mutate durable economy state.

5. The game server may submit settlement facts, but the economy service is the
   authority for durable assets.

6. `worker` belongs to the economy domain but should be deployed separately.

7. Chain implementation is Solana-first.

8. Early implementation should avoid too many runtime services.

9. Admin writes must produce audit logs.

10. High-risk economic exits must support limits, cooldowns, queues and manual
    review.

## Current API Surface

`account-api` currently has:

- `GET /health`
- `GET /api/auth/wallet/nonce`
- `POST /api/auth/wallet`
- `GET /api/auth/verify`
- `GET /api/character/list`
- `POST /api/character/create`
- `POST /api/game/launch`
- `POST /api/game/launch/consume`

`economy-api` currently has:

- `GET /health`
- `GET /api/economy/snapshot` with `accountId` and `characterId`; returns
  account token, character wallet, inventory, warehouse, loot tray and equipment
  instances
- `POST /api/economy/warehouse/deposit`
- `POST /api/economy/warehouse/withdraw`
- `POST /api/economy/equipment/equip`
- `POST /api/economy/equipment/unequip`
- `POST /api/economy/dungeon/enter`
- `POST /api/economy/dungeon/finish`
- `POST /api/economy/loot/claim-player`
- `POST /api/economy/loot/claim-all`
- `POST /api/economy/loot/discard`
- `POST /api/economy/gathering/settle`
- `POST /api/economy/farming/harvest`
- `POST /api/economy/rewards/grant-locked`
- `POST /api/chain/token/claim`
- `GET /api/chain/token/ledger`
- `POST /api/economy/internal/unlocks/settle`
- `POST /api/economy/internal/withdrawals/process`

`admin-api` currently has:

- `GET /health`
- `GET /api/admin/accounts`
- `POST /api/admin/accounts/ban`
- `POST /api/admin/accounts/risk-level`
- `POST /api/admin/accounts/license`
- `POST /api/admin/accounts/sessions/revoke`
- `GET|POST /api/admin/market/restrictions` (+ revoke)
- `GET|POST /api/admin/risk/events`
- `GET /api/admin/audits`
- `GET /api/admin/ledger`
- `GET /api/admin/withdrawals`
- `POST /api/admin/withdrawals/review`
- `GET /api/admin/payments`
- `GET /api/admin/nft/requests`
- `POST /api/admin/nft/mint/confirm`
- `GET /api/admin/hot-wallet`
- `POST /api/admin/hot-wallet/pause`

See `docs/internal/admin-api.md`.

`economy-worker` currently calls:

- `POST /api/economy/internal/unlocks/settle`
- `POST /api/economy/internal/withdrawals/process`

## Persistence State

`internal/platform/store` now has two persistence shapes:

- `Store`: in-memory development adapter.
- `PostgresStore`: Postgres adapter using `pgxpool`.

Startup selection:

- If `DATABASE_URL` is empty, the service uses in-memory store.
- If `DATABASE_URL` is set, the service opens Postgres and fails fast if it
  cannot connect.

Important: `economy-worker` still drives state by calling `economy-api` internal
HTTP endpoints. That is intentional. It keeps worker deployment independent
from direct DB ownership and preserves one service boundary for economy writes.

## Database Model

The first complete schema is in:

```text
migrations/aeonblight_full_schema.sql
```

Future SQL changes should be added as named incremental scripts under:

```text
migrations/updates/
```

The full schema file remains canonical and must be updated with every accepted
incremental change. Docker fresh initialization mounts only the full schema
file, so update scripts are not replayed on top of a complete new database.

It includes:

- account identity, wallet login nonce, refresh token, sessions
- characters
- game server registry, launch tickets, online sessions, server commands
- account GAME balances, character Gold/Gems/Stamina
- economy ledger
- locked GAME cooldown records
- system consumption split records
- Gold conversion windows and global economy windows
- item catalog, inventory, equipment, loot tray
- dungeon, gathering and boss settlement records
- marketplace listings and orders
- generic economy payment orders
- withdrawals
- Solana deposits, Solana payouts, chain cursors, hot wallet status
- risk events and account links
- revenue events and allocations
- admin users and audit logs
- versioned economy config

## Verification Done

These commands pass in the current workspace:

```bash
go test ./...
go build -o work/bin/account-api ./cmd/account-api
go build -o work/bin/economy-api ./cmd/economy-api
go build -o work/bin/admin-api ./cmd/admin-api
go build -o work/bin/economy-worker ./cmd/economy-worker
```

Postgres migration and adapter verification now pass with the local Docker
database:

```bash
docker compose -f deploy/docker-compose.yml up -d postgres
DATABASE_URL='postgres://aeonblight:aeonblight_dev_password@127.0.0.1:55432/aeonblight_game?sslmode=disable' \
  go test ./internal/platform/store -run TestPostgresStoreIntegration -count=1 -v
```

The migration created 43 public tables in `aeonblight_game`. The full schema was
also applied cleanly to an independent validation database,
`aeonblight_schema_check_codex`, with the schema migration marker
`aeonblight_full_schema`. The integration test covers wallet nonce persistence,
account/character creation, locked GAME unlock settlement, withdrawal
processing, economy snapshot reads, warehouse moves, equipment equip/unequip and
equipment uniqueness constraints. It also covers economy JSON config loading,
dungeon enter/finish idempotency, run ownership/status validation, submitted exp
persistence, config-driven item/equipment/token rewards, loot claim/discard,
gathering/farming direct-to-bag rewards and failure progress recording.

## Known Limitations

Wallet login signature verification is implemented for Solana-style Ed25519
message signatures. The login flow now requires a stored one-time nonce and a
valid signature over the exact returned message.

Solana deposit scanning and payout submission are still stubs.

The database has `solana_deposits`, `solana_payouts`, `chain_cursors` and
`hot_wallet_status`, but the worker has not implemented real RPC scanning or
transaction submission yet.

Inventory/equipment APIs are partially implemented: snapshot, warehouse
deposit/withdraw and equipment equip/unequip are available for game-server
mediated persistence. Dungeon enter/finish is available for game-server
submitted facts with config-driven backend validation and rewards. Loot
claim/discard is available for pending dungeon rewards. Gathering settlement and
farming harvest are available and place configured item/equipment rewards
directly into bag slots. Organize, craft, synthesize, repair, boss, marketplace
and NFT APIs are not implemented yet.

Dungeon settlement records accepted run facts and applies submitted exp to the
character. It validates `opId`, character ownership, run status, chapter/floor
match, allowed result values, non-negative exp and configured exp caps. On
victory it generates item loot, `IN_LOOT_TRAY` equipment instances with unique
`equipment_uid` plus affixes, and small locked token rewards from
`configs/economy`.

Gathering and farming use the same configured loot pools and equipment affix
model, but they skip `loot_tray_items`; item and equipment rewards are inserted
directly into the bag. If no bag slot is available, the settlement transaction
fails instead of silently storing the reward elsewhere.

Their tables exist so future service logic has a stable target. Equipment
storage now includes uniqueness constraints for `equipment_uid`, optional
`equipment_hash`, bag/warehouse slots and equipped slots. The schema now also
allows `IN_LOOT_TRAY` equipment so equipment rewards can wait for explicit loot
claim instead of entering the bag immediately.

Runtime SQL errors currently panic inside the repository and are converted to
HTTP 500 by `httpx.Recover`.

This is acceptable for the first adapter pass. Later, repository methods should
return explicit errors so each handler can map domain errors more precisely.

## Migration Steps To Target Directory

This workspace now lives in `/Users/TJ/code/Project/SOLP/game-backend`. If
another implementation copy needs to be moved here, use:

```bash
cd /Users/TJ/Documents/Codex/2026-07-10/wo-x
rsync -av --exclude work --exclude .git ./ /Users/TJ/code/Project/SOLP/game-backend/
```

Then verify from the target directory:

```bash
cd /Users/TJ/code/Project/SOLP/game-backend
go test ./...
go build -o work/bin/account-api ./cmd/account-api
go build -o work/bin/economy-api ./cmd/economy-api
go build -o work/bin/admin-api ./cmd/admin-api
go build -o work/bin/economy-worker ./cmd/economy-worker
DATABASE_URL='postgres://aeonblight:aeonblight_dev_password@127.0.0.1:55432/aeonblight_game?sslmode=disable' \
  go test ./internal/platform/store -run TestPostgresStoreIntegration -count=1 -v
```

To start with Docker:

```bash
cd /Users/TJ/code/Project/SOLP/game-backend/deploy
docker compose up --build
```

## Recommended Next Grill-Me Question

Should we implement Boss settlement next, or inventory organize/craft/synthesize
first?

Recommended answer: implement Boss settlement next.

Reason: dungeon, loot claim and collection-style rewards now exercise the shared
reward materialization paths. Boss settlement is the next high-value economic
source before marketplace and NFT mint flows depend on rare equipment and token
outputs.
