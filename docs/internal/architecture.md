# Backend Service Split

This repository uses one Go module and multiple deployable binaries:

```text
cmd/account-api
cmd/economy-api
cmd/admin-api
cmd/economy-worker
```

## Runtime Boundaries

`account-api` owns identity and game admission:

- wallet login
- access token issuing
- character list/create
- short-lived game launch tickets
- ticket consumption by game servers

`economy-api` owns all durable economy state:

- character economy snapshot, including account token, Gold/Gems, inventory,
  warehouse, loot tray and equipment instances
- inventory and equipment persistence after game-server validation, including
  warehouse moves, equipment equip/unequip, opId idempotency and slot uniqueness
- dungeon enter/finish fact persistence, including run ownership, status,
  chapter/floor matching, result enum validation, exp non-negative validation
  and failure progress storage
- config-driven dungeon rewards, including exp caps, item loot, equipment
  instances with random affixes, and small locked token rewards
- loot claim/discard flows for pending dungeon rewards
- gathering and farming settlement using the same loot pool and affix model,
  with item/equipment rewards placed directly into bag slots
- boss contribution and settlement using participation floor plus contribution
  tiers, with item/equipment rewards placed into the loot tray
- internal boss event open/close/settle lifecycle for global boss operations
- inventory and warehouse organize, bag discard and recipe synthesize
- locked AEB records
- withdrawable AEB
- withdrawal requests
- economy ledger
- internal worker endpoints
- Solana-first chain accounting boundary for deposits, payouts and confirmations

`admin-api` owns privileged operations:

- account lookup, ban, risk level, trading license, session revoke
- marketplace BUY/SELL/ALL restrictions
- risk event write/list and audit log list
- withdrawal review, payment/NFT queue visibility
- hot-wallet payout pause
- service identity approval/list/revocation
- NFT mint confirm (development/test stub only; production fails closed until Core is wired)

`economy-worker` owns scheduled economy progress:

- marks mature locked AEB as unlocked
- advances queued withdrawals
- later: chain confirmation, retry and hot-wallet monitoring
- later: SPL Token deposit scanning and payout confirmation

## Client And Game Server Flow

The WebGL client should keep a minimal trust surface:

```text
WebGL client -> account-api -> public server list / launch ticket
WebGL client -> game-server -> gameplay
game-server -> account-api -> consume ticket / validate account / online enter after character selection
game-server -> economy-api -> settlement / snapshot / withdrawal requests
```

The game server should submit intent and settlement facts. It must not directly
write economy storage.

Every production machine caller has an independent Ed25519 Service Identity.
Capabilities isolate gameplay, worker, payment, mint, boss-ops and reward routes.
A Game Server identity is additionally bound to its registered `serverId`.
`INTERNAL_KEY` is a development/test compatibility path and is rejected by
staging/production startup validation.

For inventory and equipment actions, the game server remains the gameplay
authority. It decides whether the action is allowed in the current scene, level,
class, combat state or social context. `economy-api` persists the accepted result
transactionally and enforces durable asset rules such as ownership, movable
status, unique slots and `opId` idempotency.

For dungeon settlement, the game server submits facts only: enter intent,
finish result, submitted exp, optional kills and optional failure/progress
payload. `economy-api` owns the durable `dungeon_runs` record, rejects mismatched
character/run/chapter/floor state, and stores accepted progress in the run
result JSON. It loads `configs/economy` through `ECONOMY_CONFIG_DIR` to enforce
configured exp caps and generate durable rewards. Items and equipment enter the
loot tray; token rewards enter locked AEB records.

Each Dungeon Run records its origin Game Server. Home-page launch only creates
an account-level ticket for the selected server. After the game client selects a
Character, it checks recovery; a started run must reconnect to its origin server
or be explicitly abandoned. Reconnect issues a short-lived origin-bound Launch
Ticket. Abandon changes the run to `CANCELLED` without invoking reward
settlement.
Creating the run also requires the Account/Character Online Presence to match
the signed calling Game Server. Both recovery decisions require an active,
durably confirmed Session; Redis cache entries never override revocation.

Gathering and farming reuse the same shared loot pool and affix model, but they
do not use the loot tray. Their item and equipment rewards go directly into bag
slots after backend ownership/idempotency checks. Token rewards still enter
locked AEB records because token balances are account-level economy state, not
inventory rows.

For global boss events, the game server submits contribution deltas during an
open event and later submits per-player settlement after the event ends. Boss
rewards reuse the shared loot pool model: every participant receives the
participation pool, contributors receive the main pool, and high contribution
tiers unlock bonus pools. Item and equipment rewards enter the loot tray; token
rewards enter locked AEB records.

## Persistence Note

The current Go code has memory and PostgreSQL implementations behind
`internal/platform/store`. Development/integration/production profiles use the
PostgreSQL adapter and the single shared schema in
`migrations/aeonblight_full_schema.sql`; memory is limited to unit/contract tests.
Durable account/economy/admin/game-server coordination therefore survives
independent process restarts.

The worker is already shaped correctly: it calls `economy-api` internal HTTP
endpoints instead of importing economy service state. Once `economy-api` uses
Postgres, the worker will continue to work across independent restarts.
