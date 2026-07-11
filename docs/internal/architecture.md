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
- locked GAME records
- withdrawable GAME
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
- NFT mint confirm (stub mint address until chain mint is wired)

`economy-worker` owns scheduled economy progress:

- marks mature locked GAME as unlocked
- advances queued withdrawals
- later: chain confirmation, retry and hot-wallet monitoring
- later: SPL Token deposit scanning and payout confirmation

## Client And Game Server Flow

The WebGL client should keep a minimal trust surface:

```text
WebGL client -> account-api -> launch ticket
WebGL client -> game-server -> gameplay
game-server -> account-api -> consume ticket / validate player
game-server -> economy-api -> settlement / snapshot / withdrawal requests
```

The game server should submit intent and settlement facts. It must not directly
write economy storage.

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
loot tray; token rewards enter locked GAME records.

Gathering and farming reuse the same shared loot pool and affix model, but they
do not use the loot tray. Their item and equipment rewards go directly into bag
slots after backend ownership/idempotency checks. Token rewards still enter
locked GAME records because token balances are account-level economy state, not
inventory rows.

For global boss events, the game server submits contribution deltas during an
open event and later submits per-player settlement after the event ends. Boss
rewards reuse the shared loot pool model: every participant receives the
participation pool, contributors receive the main pool, and high contribution
tiers unlock bonus pools. Item and equipment rewards enter the loot tray; token
rewards enter locked GAME records.

## Persistence Note

The current Go code uses `internal/platform/store` as a development adapter so
the services compile and domain behavior can be tested immediately.

For Docker production, replace it with a Postgres-backed adapter using the
single shared database model in `migrations/aeonblight_full_schema.sql`. This matters because
each Docker container has its own process memory; durable
account/economy/admin/game-server coordination state must live outside the
process to support independent restart.

The worker is already shaped correctly: it calls `economy-api` internal HTTP
endpoints instead of importing economy service state. Once `economy-api` uses
Postgres, the worker will continue to work across independent restarts.
