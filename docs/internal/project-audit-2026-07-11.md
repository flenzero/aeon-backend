# Project Audit — 2026-07-11

## Outcome

The repository is a broad, runnable backend implementation, not only a scaffold.
The four modules build and start independently, 89 registered HTTP routes have
contract coverage, and the existing PostgreSQL integration suite passes. It is
not yet production-complete because the on-chain NFT mint implementation is
absent and the no-database runtime branch advertises healthy processes while
several features are unavailable.

## Progress by runtime module

### `account-api`

Implemented: Solana nonce/signature login, access/refresh sessions, logout,
characters, launch ticket issue/consume, game-server registry, and online
presence. PostgreSQL plus Redis is the intended durable runtime.

### `economy-api`

Implemented: snapshots, inventory/warehouse/equipment actions, dungeon and
activity settlement, boss lifecycle, marketplace, growth payments, locked
rewards, withdrawals, Solana deposit/payment/payout logic, NFT request/cancel/
confirm backend state, and asset listing.

Incomplete: a real Solana NFT mint adapter/program and validator-backed mint
test. Current confirmation can synthesize a `nft_stub_*` mint address.

### `admin-api`

Implemented: account/ban/risk/license/session controls, market restrictions,
risk events, audits, ledger and queue views, withdrawal review, NFT confirmation,
and hot-wallet pause.

### `economy-worker`

Implemented: unlock settlement, withdrawal processing, deposit scan, payout
submission, and payout confirmation calls on each tick. The worker is runnable,
but its scheduler still lives in `main.go` and has no direct unit-test surface.

## Git branch audit

- The current directory is not a Git working tree; `.git` is absent.
- `git ls-remote --heads --tags https://github.com/flenzero/aeon-backend.git`
  returned no refs on 2026-07-11.
- Therefore there are no local or remote branches to classify as usable or
  unusable. The immediate issue is missing repository initialization/history,
  not a broken branch.

## Verified test baseline

`./test/run.sh --full` passed on 2026-07-11:

- all Go unit tests;
- all 89 HTTP route registration/authentication contracts;
- wallet-login → character → snapshot → admin lookup flow;
- local Solana JSON-RPC contract transport;
- backend NFT request → confirmation → asset-list lifecycle;
- builds and real local startup for all four runtime modules;
- 12 PostgreSQL integration groups, including Solana deposit/payment and NFT
  request/confirmation state.

## Deepening opportunities

The terms Module, Interface, Seam, Adapter, Depth, Leverage, and Locality below
use the repository architecture-review vocabulary.

1. **Split the persistence Interface by runtime/domain usage.**
   - Files: `internal/platform/store/repository.go`, account/economy/admin callers.
   - Problem: one roughly 97-method Interface makes every caller learn nearly
     the whole system and makes fakes expensive.
   - Solution: keep the real Postgres/in-memory Seam, but expose smaller caller-
     focused persistence Interfaces and propagate request context consistently.
   - Benefit: higher Depth and Locality; focused Adapter contracts and failure/
     cancellation tests become practical.

2. **Remove the misleading implicit in-memory runtime branch.**
   - Files: `internal/platform/store/factory.go`, `internal/platform/store/store.go`.
   - Problem: independent processes do not share memory; NFT, marketplace,
     equipment repair, payments, and chain jobs are unavailable while `/health`
     still reports success.
   - Solution: fail fast when a complete runtime lacks PostgreSQL, and allow the
     limited in-memory Adapter only through an explicit development/test profile.
   - Benefit: readiness becomes truthful and feature-availability tests can
     distinguish a usable module from a merely listening process.

3. **Deepen Economy modules instead of pass-through methods.**
   - Files: `internal/economy/http.go`, `internal/economy/service.go`,
     `internal/platform/store/postgres.go`, `marketplace.go`, `chain.go`.
   - Problem: a 1,300-line HTTP Adapter and many shallow Service pass-throughs
     leave rules, transactions, and SQL spread across several large files.
   - Solution: concentrate Dungeon, Inventory, Marketplace, NFT, and Token state
     transitions behind domain-named Modules; keep HTTP and Postgres as Adapters.
   - Benefit: rule/state-machine tests use the same Interface as callers, while
     SQL tests focus on atomic persistence.

4. **Separate Solana settlement orchestration from database transactions.**
   - Files: `internal/chain`, `internal/economy/service.go`,
     `internal/platform/store/chain.go`.
   - Problem: RPC calls, long database transactions, signing, and status changes
     are mixed. `chain.RPC` is a useful real Seam with HTTP and memory Adapters.
   - Solution: make chain settlement a deep Module and avoid external RPC calls
     while holding database transactions.
   - Benefit: RPC errors, duplicate signatures, missing ATA, and confirmation
     states can be tested without PostgreSQL; DB tests verify claim/commit rules.

5. **Make each Admin Action atomic with its audit/risk effects.**
   - Files: `internal/admin/http.go`, `internal/platform/store/admin.go`.
   - Problem: handlers perform multiple independent writes and sometimes ignore
     risk-event errors, allowing state change without the required audit trail.
   - Solution: concentrate each Admin Action, risk event, and Audit Record in one
     transactional Module.
   - Benefit: table-driven tests can assert audit-always and rollback-on-failure.

6. **Create an application composition Module for all four runtimes.**
   - Files: `cmd/*/main.go`, `internal/account/http.go`, store/config setup.
   - Problem: dependency validation, Redis lifecycle, shutdown, and readiness are
     hidden or repeated; constructors can terminate the process.
   - Solution: centralize construction, capability validation, startup, graceful
     shutdown, and readiness while preserving four independent binaries.
   - Benefit: each runtime becomes an in-process test surface with deterministic
     dependency failure and shutdown coverage.

7. **Deepen the worker scheduler.**
   - Files: `cmd/economy-worker/main.go`.
   - Problem: scheduling, five jobs, HTTP calls, and string-form results live in
     `main.go`, with no direct tests.
   - Solution: move the worker tick into a reusable Module with structured job
     results; retain HTTP as its Adapter.
   - Benefit: tests can cover ordering, timeouts, partial failures, cancellation,
     and overlap rules without starting a process.

The strongest existing deep Modules to preserve are `chain.RPC`,
`redisx.Client`, and the idempotent database action implementation.
