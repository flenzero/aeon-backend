# Data Model

The backend uses one Postgres database for all non-game-server services.

The services stay independently deployable:

- `account-api`
- `economy-api`
- `admin-api`
- `economy-worker`

They coordinate through durable rows, not process memory.

## Domains In The Same Database

`account` data:

- `accounts`
- `wallet_login_nonces`
- `refresh_tokens`
- `account_sessions`
- `characters`

`game server` data:

- `game_servers`
- `game_tickets`
- `online_sessions`
- `game_server_commands`

`economy` data:

- `account_tokens`
- `character_wallets`
- `economy_ledger`
- `locked_game_records`
- `gold_conversion_windows`
- `global_economy_windows`
- `system_consumptions`
- `item_catalog`
- `inventory_items`
- `equipment_items`
- `nft_assets`
- `nft_mint_requests`
- `loot_tray_items`
- `dungeon_runs`
- `gathering_settlements`
- `boss_events`
- `boss_contributions`
- `marketplace_listings`
- `marketplace_orders`
- `economy_payment_orders`

`Solana chain` data:

- `solana_deposits`
- `solana_payouts`
- `chain_cursors`
- `hot_wallet_status`

`risk` data:

- `account_risk_events`
- `account_links`

`admin and operations` data:

- `admin_users`
- `admin_audit_logs`
- `economy_config_versions`
- `revenue_events`
- `revenue_allocations`

## Balance Rules

GAME values are stored as integer base units in `NUMERIC(38, 0)`. This avoids
floating point drift and keeps the schema compatible with SPL token decimals.

Gold and gems are off-chain game values:

- Gold lives in `character_wallets.gold`.
- Gems live in `character_wallets.gems`.
- GAME lives at account level in `account_tokens`.

`locked_game_records` is the source of truth for cooldown GAME. A matured record
is marked `UNLOCKED` by `economy-worker`; the corresponding amount is moved from
`account_tokens.locked_balance` to `account_tokens.withdrawable_balance`.

## Dungeon Run Rules

`dungeon_runs` is the durable record for game-server submitted dungeon facts.
Entering creates a `STARTED` run with a generated `dungeonRunId`. Finishing a
run requires the same account, character, chapter and floor, and only a
`STARTED` run can be finished.

The `result` JSON stores the accepted finish facts, including `result`, `exp`,
optional `kills` and optional `progress`. Defeat and timeout progress is stored
there so the backend can audit and later derive progression without trusting the
client.

Current validation covers ownership, status, chapter/floor match, allowed result
values, non-negative exp and configured exp caps. Victory rewards are generated
from `configs/economy`: item rewards enter `loot_tray_items`, equipment rewards
create unique `equipment_items` rows in `IN_LOOT_TRAY`, and token rewards enter
locked GAME records.

## Inventory And Equipment Rules

Ordinary stackable inventory rows live in `inventory_items`; equipment lives in
`equipment_items` as unique instances.

Equipment must never be identified only by `item_id`, rarity or enhance level.
Every equipment row has a globally unique `equipment_uid`, and mintable
equipment can also carry a unique `equipment_hash` for chain/NFT workflows.

Storage uniqueness is enforced in SQL:

- one ordinary item stack per character/location/slot in active storage
- one equipment instance per `equipment_uid`
- one optional chain `equipment_hash`
- one equipment item per character bag/warehouse slot
- one equipped item per character/equip slot

Dungeon equipment reward instances use `IN_LOOT_TRAY` until loot claim moves
them into a concrete bag slot. This keeps generated weapons and their random
affixes unique before the player claims them.

Gathering and farming are treated as collection-style activities. Their
material and equipment rewards skip `loot_tray_items` and are inserted directly
into `inventory_items` or `equipment_items` with `location = 'IN_BAG'`.

This follows the internal economy model and keeps later mint, marketplace and
equipment transfer flows from accidentally duplicating a game asset.

## Config Rules

Economy parameters should not be hard-coded in service binaries. Runtime-tuned
values live in `economy_config_versions`:

- withdrawal limits
- Gold to GAME conversion limits
- contribution tier cooldowns
- chain defaults
- later: marketplace fee split, burn/recycle/reward split, NFT mint price

Only one row per `config_key` should be `ACTIVE` in production. The initial SQL
does not enforce that with a partial unique index yet, because the admin config
workflow will decide whether draft/activation should be strict or history-based.

## Worker Ownership

The worker owns time-based state transitions:

- settle mature `locked_game_records`
- advance `withdrawals`
- submit and confirm `solana_payouts`
- scan `solana_deposits`
- update `chain_cursors`
- monitor `hot_wallet_status`

The worker should use row locks and small batches when the Postgres adapter is
implemented.

The first Postgres adapter now uses row locks for the two implemented worker
state transitions: locked GAME unlock settlement and withdrawal queue
processing.
