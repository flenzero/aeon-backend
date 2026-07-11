# Aeonblight Game Backend

Go backend scaffold for the non-game-server parts of the Aeonblight GameFi stack.

Runtime shape:

```text
account-api
economy-api
admin-api
economy-worker
postgres
redis
```

The first version keeps the service boundaries strict while using one repository
and shared internal packages. The HTTP services can be restarted independently in
Docker. The worker is deployed as its own process because it advances economy
state machines such as locked GAME unlocks and withdrawals.

## Configuration

Runtime / secrets (env):

```bash
cp local.env.example local.env
# edit local.env, then:
docker compose -f deploy/docker-compose.yml --env-file local.env up -d
# or just go run — config.Load() auto-reads ./local.env in non-production
```

- Template: [`local.env.example`](local.env.example)
- Local overrides: `local.env` (gitignored)
- Play values (fees, slots, loot): `configs/economy/*.json`

## Run Locally

```bash
cp local.env.example local.env
docker compose -f deploy/docker-compose.yml --env-file local.env up -d postgres
go test ./...
go run ./cmd/account-api
go run ./cmd/economy-api
go run ./cmd/admin-api
go run ./cmd/economy-worker
```

Run the complete local verification suite (all routes, Solana/NFT backend
contracts, four binaries, and PostgreSQL/Redis integration):

```bash
./test/run.sh --full
```

See [`test/README.md`](test/README.md) for automatic mode and exact coverage.

Default ports:

- `account-api`: `8081`
- `economy-api`: `8082`
- `admin-api`: `8083`
- `postgres`: container `5432`, host `${POSTGRES_PORT:-55432}`
- `economy-worker`: no HTTP listener

Chain default:

- Solana is the default chain target for wallet login, token accounting,
  deposit scan and payout confirmation.
- Wallet addresses are treated as case-sensitive base58 public keys.

## Current Scope

Implemented first:

- Account wallet nonce, Solana signature login, durable sessions (Postgres +
  Redis), refresh/logout, game-server registry and online presence.
- Economy balance, locked GAME records, unlock settlement, withdrawal request
  and automatic withdrawal queue logic.
- Character economy snapshot with account token, Gold/Gems, inventory,
  warehouse, loot tray and equipment instance reads.
- Game-server mediated warehouse moves and equipment equip/unequip persistence
  with `opId` idempotency and durable slot uniqueness.
- Game-server submitted dungeon enter/finish facts, with Postgres-backed
  `opId` idempotency, run ownership/status validation and failure progress
  persistence.
- Versioned JSON economy configuration under `configs/economy`, including
  dungeon exp caps, shared loot pools, gathering/farming reward pools and random
  equipment affix pools.
- Config-driven dungeon rewards that materialize item loot, equipment instances
  with unique `equipmentUid` and affixes, and small locked token rewards.
- Loot claim and discard flows that move pending dungeon rewards from loot tray
  into bag slots or delete unwanted pending rewards.
- Inventory organize, discard and config-driven synthesize for bag management.
- Warehouse organize for warehouse slot compaction and stack merging.
- Config-driven gathering and farming harvest settlements that place item and
  equipment rewards directly into the bag, while token rewards still enter
  locked GAME.
- Config-driven boss contribution and settlement with participation floor,
  contribution-weighted loot pools and loot-tray materialization.
- Internal boss event lifecycle APIs to open, close and mark events settled.
- Admin review actions and audit ledger.
- Economy worker loop as an independent process.
- Docker build and compose files for independent service restart.
- Canonical full Postgres schema target in `migrations/aeonblight_full_schema.sql`.
- First-pass Postgres store adapter selected by `DATABASE_URL`.
- Complete first-pass single-database model for account, economy, admin,
  game-server coordination and Solana chain accounting.
- Unique equipment instance storage constraints for future inventory, NFT mint
  and marketplace flows.

```text
WebGL / launcher
  -> account-api (wallet login, JWT, launch ticket, sessions)
Game servers
  -> account-api (consume ticket, online enter/heartbeat)
  -> economy-api (snapshot, inventory, marketplace, chain payments)
economy-worker
  -> economy-api internal jobs
Ops
  -> admin-api
Shared
  -> postgres
  -> redis (account sessions / online presence)
```

Still intentionally stubbed / next:

- On-chain Solana NFT mint (current confirm uses stub mint address until chain mint is wired)
- Account-link graph tooling and automated risk scoring

Recently completed:

- Marketplace list/buy/cancel + slot expands
- Solana deposit scan, SPL payout signer, payment tx verify
- Bag expand + trading license (on-chain payment orders)
- Redis-backed sessions, refresh/logout, game-server registry, online presence
- Equipment durability wear + AEB repair
- NFT mint request / cancel / confirm (stub mint address) + asset list
- Admin ops: account detail/ban/risk/license, market restrictions, risk events,
  audits, payments/NFT queue, hot-wallet pause
## Economy Configuration

`economy-api` reads JSON economy rules from `ECONOMY_CONFIG_DIR` (default
`configs/economy`): items, marketplace, recipes, loot, dungeons, gathering,
farming, bosses, affixes.

## Postgres Verification

Local Docker defaults:

- database: `aeonblight_game`
- user: `aeonblight`
- password: `aeonblight_dev_password`
- host port: `55432`

```bash
docker compose -f deploy/docker-compose.yml up -d postgres
DATABASE_URL='postgres://aeonblight:aeonblight_dev_password@127.0.0.1:55432/aeonblight_game?sslmode=disable' \
  go test ./internal/platform/store -run TestPostgresStoreIntegration -count=1 -v
```

## Database SQL Strategy

- `migrations/aeonblight_full_schema.sql` is the canonical full database schema.
- Fresh Docker initialization mounts only that full schema file.
- Future production deltas go under `migrations/updates/` with explicit names
  such as `0002_add_inventory_actions.sql`.
- Every accepted update must also be folded back into
  `migrations/aeonblight_full_schema.sql`, so the full schema always remains a
  complete from-zero database definition.

See `docs/internal/architecture.md` for the service split and persistence boundary.
See `docs/internal/chain-solana.md` for the Solana-first chain boundary.
See `docs/internal/data-model.md` for the shared database model.
See `docs/internal/gamefi-economy-system-v0.2.md` for the current economy design.
See `docs/internal/migration-handoff.md` for the current progress, Grill-Me
constraints and migration instructions for `/Users/TJ/code/Project/SOLP/game-backend`.
