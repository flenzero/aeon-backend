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
state machines such as locked AEB unlocks and withdrawals.

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
docker compose -f deploy/docker-compose.yml --env-file local.env up -d postgres redis
DATABASE_URL='postgres://aeonblight:aeonblight_dev_password@127.0.0.1:55432/aeonblight_game?sslmode=disable' \
  ./scripts/db-migrate.sh bootstrap
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

See [`test/README.md`](test/README.md) for `unit`, `contract`, `integration`,
and `full` coverage.

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
- Economy balance, locked AEB records, unlock settlement, withdrawal request
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
  locked AEB.
- Config-driven boss contribution and settlement with participation floor,
  contribution-weighted loot pools and loot-tray materialization.
- Internal boss event lifecycle APIs to open, close and mark events settled.
- Admin review actions and audit ledger.
- Ed25519 ordinary-admin public-key provisioning, one-time signed login
  challenge, short-lived admin JWTs and immediate disablement enforcement.
- Economy worker loop as an independent process.
- Docker build and compose files for independent service restart.
- Canonical full Postgres schema target in `migrations/aeonblight_full_schema.sql`.
- Explicit-only database migration command in `scripts/db-migrate.sh`; application
  and Docker startup never mutate schema.
- Runtime profiles, aggregated startup validation, and separate `/health` and
  dependency-aware `/ready` endpoints.
- First-pass Postgres store adapter selected by `DATABASE_URL`.
- Complete first-pass single-database model for account, economy, admin,
  game-server coordination and Solana chain accounting.
- Unique equipment instance storage constraints for future inventory, NFT mint
  and marketplace flows.

```text
WebGL / launcher
  -> account-api (wallet login, JWT, public server list, launch ticket, sessions)
Game servers
  -> account-api (consume ticket, character-bound online enter/heartbeat)
  -> economy-api (snapshot, inventory, marketplace, chain payments)
economy-worker
  -> economy-api internal jobs
Ops
  -> admin-api
Shared
  -> postgres
  -> redis (account sessions / online presence)
```

## Production service identities

Production and staging do not accept a shared `INTERNAL_KEY`. Every game server,
worker or operator process uses its own Ed25519 key pair and registered service
identity. The private key stays on that process; the super administrator stores
only the public key and approved capability set.

- `POST /api/admin/service-identities`: super administrator creates/approves an identity.
- `GET /api/admin/service-identities`: administrators may inspect active/disabled identities.
- `DELETE /api/admin/service-identities/{serviceId}`: super administrator soft-disables it; audit history is retained and subsequent requests are rejected immediately.
- Game-server identities are also bound to one `subjectId`/`serverId`, so one server cannot heartbeat, consume tickets for, or inspect/mutate online presence owned by another server.

Signed requests carry `X-Service-Id`, `X-Service-Timestamp`, `X-Service-Nonce`
and `X-Service-Signature`. The signature covers method, escaped path/query and
the SHA-256 body hash. Nonces are one-time and timestamps default to a two-minute
window. `INTERNAL_KEY` remains only as an explicit development/test compatibility
path and must be empty in staging/production.

## Dungeon reconnect recovery

`GET /api/game/dungeon/recovery?characterId=...` is called with the player's JWT
after the game client has selected a character. A `STARTED` dungeon returns `required=true` plus
its `dungeonRunId` and original `serverId`. Redis keeps the hot recovery hint,
while PostgreSQL confirms the run is still active so stale Redis data cannot
resurrect a finished run.

`POST /api/game/dungeon/recovery` requires the still-active `sessionId` and
accepts `action=resume|abandon`:

- `resume` requires the active session and returns a 90-second Launch Ticket
  restricted to the original server;
- `abandon` atomically changes the run to `CANCELLED`, invalidates outstanding
  resume tickets, clears the Redis hint and grants no experience, loot or AEB.

Home-page launch tickets are account-level and server-bound; they do not carry a
character. A character can have at most one `STARTED` dungeon, and recovery
decisions after character selection still force reconnect to the origin server
or explicit abandon.

Still intentionally stubbed / next:

- Metaplex Core on-chain NFT mint/freeze/transfer reconciliation (development/test
  may use the stub; staging/production fail closed until the adapter is wired)
- Ed25519 super-admin authentication beyond the current operations key
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
  ./scripts/db-migrate.sh bootstrap
DATABASE_URL='postgres://aeonblight:aeonblight_dev_password@127.0.0.1:55432/aeonblight_game?sslmode=disable' \
  go test ./internal/platform/store -run TestPostgresStoreIntegration -count=1 -v
```

## Database SQL Strategy

- `migrations/aeonblight_full_schema.sql` is the canonical full database schema.
- Docker and all four application processes never execute migrations.
- A technician explicitly runs `scripts/db-migrate.sh bootstrap` for a new
  database or `scripts/db-migrate.sh up` for an existing database.
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
See `docs/internal/implementation-progress-2026-07-12.md` for the current
implementation checkpoint and remaining roadmap.

## API Documentation

- [`docs/api/interface-list.md`](docs/api/interface-list.md): non-admin API quick list
- [`docs/api/interface-reference.md`](docs/api/interface-reference.md): non-admin API integration reference
- [`docs/api/admin-interface-reference.md`](docs/api/admin-interface-reference.md): independent administrator API reference
