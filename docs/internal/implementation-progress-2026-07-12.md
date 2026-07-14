# Aeon Backend Implementation Progress — 2026-07-12

## Outcome

The first deployment-safety and NFT-economy tranche is implemented and passes
the full local suite. The four runtime modules remain independently runnable.
The canonical chain token symbol is **AEB**, short for Aeonblight.

## Completed in this tranche

1. Runtime controls:
   - `APP_PROFILE=test|development|staging|production`
   - `TEST_SCOPE=unit|contract|integration|full`
   - `STUB_MODE=enabled|disabled`
   - `ALLOW_REDIS_FALLBACK=true|false` (development only)
2. Startup validation aggregates all missing/invalid configuration into one
   refusal message. Test unit/contract may use memory persistence; development,
   staging, production, and test integration/full require their real declared
   dependencies.
3. `/health` is liveness only. `/ready` reports PostgreSQL, schema version,
   Redis, economy rule loading, and Solana RPC when real-chain mode is enabled.
   Required failures return HTTP 503 with per-check reasons.
4. Schema mutation is isolated to `scripts/db-migrate.sh`. It supports only
   explicit `bootstrap`, `up`, and `status`. Docker and application startup do
   not run migrations, and there is no automatic down path.
5. AEB replaces the former placeholder token name in public documentation,
   economy configuration, loot configuration, and ledger writes.
6. NFT Mint fees are configuration-driven and snapshotted when requested:
   - rarity 3 / Rare: 500 AEB
   - rarity 4 / Epic: 2,000 AEB
   - rarity 5+ / Legendary: 10,000 AEB
7. NFT cancellation restores AEB to its original locked, withdrawable, and
   external balance categories. Locked refunds retain their original unlock
   timestamps. Old requests without a source snapshot retain a compatibility
   fallback to withdrawable balance.
8. Every machine caller can use an independent Ed25519 Service Identity. The
   super administrator approves/creates and soft-disables identities; ordinary
   administrators can list them. Capabilities isolate gameplay, worker,
   payments, mint, boss-ops and reward routes, while Game Server `subjectId`
   binding prevents cross-server ticket/presence operations.
9. Request boundaries now reject unknown/trailing JSON, empty command bodies,
   bodies over 1 MiB, malformed/overflow pagination and identifiers, invalid
   Unicode character limits, stale/future/tampered signatures, nonce replay,
   cross-account/cross-request idempotency reuse, and non-positive withdrawal.
10. Chain/economy invariants now prevent duplicate payout rows, payment receipt
    double-credit and single-page deposit loss. Production NFT confirmation
    fails closed until the Metaplex Core adapter exists.
11. `test/run.sh` now implements the four completeness levels. Full mode performs
    an explicit `bootstrap`/`up` migration choice, 19 PostgreSQL integration groups,
    Redis connectivity, 97 route contracts, readiness checks, four builds/starts,
    and a worker tick.
12. Dungeon recovery now records the origin Game Server, limits each Character
    to one active run, prompts the JWT-authenticated home screen, restricts
    resume tickets to the origin server, blocks cross-server launch/finish, and
    cancels declined recovery without rewards. Redis is a hot hint only;
    PostgreSQL prevents stale-cache resurrection.

## Confirmed product and chain decisions

- Equipment NFTs use the official Metaplex Core Program. Version one does not
  introduce a custom Solana program.
- The player wallet owns the Core Asset. Stable identity/rarity/affix-hash data
  belongs in Core Attributes; high-frequency gameplay state remains in Postgres.
- Activating an NFT for gameplay will verify ownership and use the Core Freeze
  Delegate. Deactivation requires unequipped/no-listing/no-combat state before
  unfreezing.
- A dedicated project Mint Payer pays SOL. The player pays the configured AEB
  fee once. Retry never charges twice; terminal failure/cancellation refunds the
  original AEB categories.
- Collection/Update Authority must be separate from the low-balance Mint Payer.
  Low SOL and daily caps must pause new mint work with an explicit readiness or
  admin reason.

## Remaining roadmap

Priority 1:

- implement the Metaplex Core adapter: create Asset, verify collection and
  Attributes, Freeze/Thaw, ownership synchronization, reconciliation, retry,
  SOL balance/cap checks, and validator-backed tests;
- extend the new ordinary-admin Ed25519 signed-login model to super-admin
  identity, fine-grained role enforcement, re-sign for high-risk actions, and
  ordinary grant limits;
- make admin mutations and their audit/risk records one database transaction.

Priority 2:

- split the 97-method persistence interface into caller-focused account,
  economy, admin, and worker interfaces;
- extract the worker scheduler from `main.go` and add partial-failure/overlap
  tests;
- centralize HTTP graceful shutdown and runtime composition;
- move Solana RPC calls outside long database transactions and deepen the chain
  settlement module.

## Verification

Passed locally on 2026-07-12:

```bash
./test/run.sh --full
go test ./...
go vet ./...
go test -race ./...
DATABASE_URL='postgres://…' go test ./internal/platform/store -run '^TestPostgres' -count=1 -v
```

Result: all unit/contract tests, 97 routes, capability and body/query boundary
matrices, race detection, all 19 PostgreSQL integration groups, explicit
migration, Docker startup, Redis, readiness and the worker tick passed.
