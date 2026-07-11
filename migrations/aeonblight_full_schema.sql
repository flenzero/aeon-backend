-- Aeonblight Game Backend full schema.
--
-- One production database is shared by account-api, economy-api, admin-api and
-- economy-worker. Services remain separately deployable; durability and
-- cross-service coordination live here.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- Shared helpers
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
  op_id TEXT PRIMARY KEY,
  scope TEXT NOT NULL,
  account_id BIGINT,
  character_id BIGINT,
  request_hash TEXT,
  response JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_idempotency_account_scope
  ON idempotency_keys(account_id, scope, created_at DESC);

-- ---------------------------------------------------------------------------
-- Account service data
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS accounts (
  id BIGSERIAL PRIMARY KEY,
  username TEXT NOT NULL,
  solana_wallet_address TEXT UNIQUE,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  risk_level INTEGER NOT NULL DEFAULT 0,
  ban_reason TEXT,
  has_trading_license BOOLEAN NOT NULL DEFAULT FALSE,
  trading_license_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_login_at TIMESTAMPTZ,
  CONSTRAINT chk_accounts_status
    CHECK (status IN ('ACTIVE', 'BANNED', 'FROZEN', 'DELETED'))
);

CREATE INDEX IF NOT EXISTS idx_accounts_status
  ON accounts(status);

CREATE TABLE IF NOT EXISTS wallet_login_nonces (
  nonce TEXT PRIMARY KEY,
  wallet_address TEXT NOT NULL,
  message TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'PENDING',
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_wallet_login_nonces_status
    CHECK (status IN ('PENDING', 'CONSUMED', 'EXPIRED'))
);

CREATE INDEX IF NOT EXISTS idx_wallet_login_nonces_wallet
  ON wallet_login_nonces(wallet_address, created_at DESC);

CREATE TABLE IF NOT EXISTS refresh_tokens (
  token TEXT PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  session_id TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_account
  ON refresh_tokens(account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS account_sessions (
  session_id TEXT PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  wallet_plugin TEXT,
  device_id TEXT,
  ip_address INET,
  user_agent TEXT,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  CONSTRAINT chk_account_sessions_status
    CHECK (status IN ('ACTIVE', 'REVOKED', 'EXPIRED'))
);

CREATE INDEX IF NOT EXISTS idx_account_sessions_account
  ON account_sessions(account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS characters (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  class_key TEXT,
  level INTEGER NOT NULL DEFAULT 1,
  exp BIGINT NOT NULL DEFAULT 0,
  map_id TEXT,
  position JSONB NOT NULL DEFAULT '{}'::jsonb,
  appearance JSONB NOT NULL DEFAULT '{}'::jsonb,
  bag_expand_count INTEGER NOT NULL DEFAULT 0,
  is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_characters_bag_expand_nonnegative
    CHECK (bag_expand_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_characters_account
  ON characters(account_id, is_deleted, id);

-- ---------------------------------------------------------------------------
-- Game server discovery, launch tickets and live sessions
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS game_servers (
  server_id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  region TEXT,
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  public_endpoint TEXT,
  max_players INTEGER NOT NULL DEFAULT 50,
  online_players INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'STARTING',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_heartbeat_at TIMESTAMPTZ,
  CONSTRAINT chk_game_servers_status
    CHECK (status IN ('STARTING', 'ONLINE', 'DRAINING', 'OFFLINE'))
);

CREATE INDEX IF NOT EXISTS idx_game_servers_status
  ON game_servers(status, last_heartbeat_at DESC);

CREATE TABLE IF NOT EXISTS game_tickets (
  ticket TEXT PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  session_id TEXT NOT NULL,
  server_id TEXT,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_game_tickets_status
    CHECK (status IN ('ACTIVE', 'CONSUMED', 'EXPIRED', 'CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_game_tickets_account
  ON game_tickets(account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS online_sessions (
  account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  session_id TEXT NOT NULL,
  server_id TEXT NOT NULL REFERENCES game_servers(server_id) ON DELETE CASCADE,
  connection_id TEXT NOT NULL,
  entered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_online_sessions_server
  ON online_sessions(server_id, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS game_server_commands (
  id BIGSERIAL PRIMARY KEY,
  server_id TEXT REFERENCES game_servers(server_id) ON DELETE CASCADE,
  account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE,
  command TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL DEFAULT 'PENDING',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  delivered_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  CONSTRAINT chk_game_server_commands_status
    CHECK (status IN ('PENDING', 'DELIVERED', 'COMPLETED', 'CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_game_server_commands_pending
  ON game_server_commands(server_id, status, created_at);

-- ---------------------------------------------------------------------------
-- Economy balances and ledgers
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS account_tokens (
  account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  token_balance NUMERIC(38, 0) NOT NULL DEFAULT 0,
  withdrawable_balance NUMERIC(38, 0) NOT NULL DEFAULT 0,
  locked_balance NUMERIC(38, 0) NOT NULL DEFAULT 0,
  external_balance NUMERIC(38, 0) NOT NULL DEFAULT 0,
  unlock_credit NUMERIC(38, 0) NOT NULL DEFAULT 0,
  cumulative_effective_spend NUMERIC(38, 0) NOT NULL DEFAULT 0,
  contribution_tier INTEGER NOT NULL DEFAULT 0,
  tier_benefit_tier INTEGER NOT NULL DEFAULT 0,
  last_effective_activity_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_account_tokens_nonnegative
    CHECK (
      token_balance >= 0
      AND withdrawable_balance >= 0
      AND locked_balance >= 0
      AND external_balance >= 0
      AND unlock_credit >= 0
    )
);

CREATE TABLE IF NOT EXISTS character_wallets (
  character_id BIGINT PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
  gold BIGINT NOT NULL DEFAULT 0,
  gems BIGINT NOT NULL DEFAULT 0,
  stamina INTEGER NOT NULL DEFAULT 0,
  gold_convert_capacity BIGINT NOT NULL DEFAULT 0,
  gold_convert_efficiency_bps INTEGER NOT NULL DEFAULT 10000,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_character_wallets_nonnegative
    CHECK (gold >= 0 AND gems >= 0 AND stamina >= 0)
);

CREATE TABLE IF NOT EXISTS economy_ledger (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
  character_id BIGINT REFERENCES characters(id) ON DELETE SET NULL,
  kind TEXT NOT NULL,
  currency TEXT,
  amount NUMERIC(38, 0),
  before_value NUMERIC(38, 0),
  after_value NUMERIC(38, 0),
  ref_type TEXT,
  ref_id TEXT,
  reason TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_economy_ledger_account
  ON economy_ledger(account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_economy_ledger_character
  ON economy_ledger(character_id, created_at DESC);

CREATE TABLE IF NOT EXISTS locked_token_records (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  amount NUMERIC(38, 0) NOT NULL,
  remaining_amount NUMERIC(38, 0) NOT NULL,
  source TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'LOCKED',
  ref_type TEXT,
  ref_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  unlock_at TIMESTAMPTZ NOT NULL,
  settled_at TIMESTAMPTZ,
  CONSTRAINT chk_locked_token_records_amount
    CHECK (amount > 0 AND remaining_amount >= 0 AND remaining_amount <= amount),
  CONSTRAINT chk_locked_token_records_status
    CHECK (status IN ('LOCKED', 'UNLOCKED', 'CONSUMED', 'WITHDRAWN', 'CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_locked_token_unlock
  ON locked_token_records(status, unlock_at);

CREATE INDEX IF NOT EXISTS idx_locked_token_account
  ON locked_token_records(account_id, status, unlock_at);

CREATE TABLE IF NOT EXISTS system_consumptions (
  id BIGSERIAL PRIMARY KEY,
  op_id TEXT NOT NULL UNIQUE,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT REFERENCES characters(id) ON DELETE SET NULL,
  spend_source TEXT NOT NULL,
  purpose TEXT NOT NULL,
  amount_token NUMERIC(38, 0) NOT NULL,
  burn_amount NUMERIC(38, 0) NOT NULL DEFAULT 0,
  recycle_amount NUMERIC(38, 0) NOT NULL DEFAULT 0,
  reward_pool_amount NUMERIC(38, 0) NOT NULL DEFAULT 0,
  effective_contribution_amount NUMERIC(38, 0) NOT NULL DEFAULT 0,
  unlock_credit_created NUMERIC(38, 0) NOT NULL DEFAULT 0,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_system_consumptions_spend_source
    CHECK (spend_source IN ('LOCKED_TOKEN', 'WITHDRAWABLE_TOKEN', 'EXTERNAL_TOKEN', 'MIXED')),
  CONSTRAINT chk_system_consumptions_amounts
    CHECK (
      amount_token > 0
      AND burn_amount >= 0
      AND recycle_amount >= 0
      AND reward_pool_amount >= 0
      AND effective_contribution_amount >= 0
      AND unlock_credit_created >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_system_consumptions_account
  ON system_consumptions(account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS gold_conversion_windows (
  id BIGSERIAL PRIMARY KEY,
  window_date DATE NOT NULL,
  account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE,
  converted_gold BIGINT NOT NULL DEFAULT 0,
  minted_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(window_date, account_id)
);

CREATE INDEX IF NOT EXISTS idx_gold_conversion_windows_date
  ON gold_conversion_windows(window_date);

CREATE TABLE IF NOT EXISTS global_economy_windows (
  id BIGSERIAL PRIMARY KEY,
  window_key TEXT NOT NULL,
  window_start TIMESTAMPTZ NOT NULL,
  window_end TIMESTAMPTZ NOT NULL,
  metric TEXT NOT NULL,
  used_amount NUMERIC(38, 0) NOT NULL DEFAULT 0,
  limit_amount NUMERIC(38, 0),
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(window_key, metric)
);

CREATE INDEX IF NOT EXISTS idx_global_economy_windows_metric
  ON global_economy_windows(metric, window_start DESC);

-- ---------------------------------------------------------------------------
-- Inventory, equipment, loot and warehouse
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS item_catalog (
  item_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  category TEXT NOT NULL,
  rarity INTEGER NOT NULL DEFAULT 0,
  stackable BOOLEAN NOT NULL DEFAULT TRUE,
  tradable BOOLEAN NOT NULL DEFAULT FALSE,
  nft_mintable BOOLEAN NOT NULL DEFAULT FALSE,
  default_bind_type TEXT NOT NULL DEFAULT 'BOUND',
  base_sell_gold BIGINT NOT NULL DEFAULT 0,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_item_catalog_default_bind_type
    CHECK (default_bind_type IN ('BOUND', 'ACCOUNT_BOUND', 'UNBOUND'))
);

CREATE TABLE IF NOT EXISTS inventory_items (
  id BIGSERIAL PRIMARY KEY,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  item_id TEXT NOT NULL REFERENCES item_catalog(item_id),
  quantity BIGINT NOT NULL,
  location TEXT NOT NULL DEFAULT 'BAG',
  slot INTEGER,
  bind_type TEXT NOT NULL DEFAULT 'BOUND',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_inventory_items_quantity CHECK (quantity > 0),
  CONSTRAINT chk_inventory_items_location
    CHECK (location IN ('BAG', 'WAREHOUSE', 'LOOT_TRAY', 'LISTED', 'CONSUMED', 'DELETED')),
  CONSTRAINT chk_inventory_items_bind_type
    CHECK (bind_type IN ('BOUND', 'ACCOUNT_BOUND', 'UNBOUND'))
);

CREATE INDEX IF NOT EXISTS idx_inventory_items_character_location
  ON inventory_items(character_id, location, slot);

CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_items_character_location_slot
  ON inventory_items(character_id, location, slot)
  WHERE slot IS NOT NULL AND location IN ('BAG', 'WAREHOUSE', 'LOOT_TRAY');

CREATE TABLE IF NOT EXISTS equipment_items (
  id BIGSERIAL PRIMARY KEY,
  equipment_uid TEXT NOT NULL,
  equipment_hash TEXT,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT REFERENCES characters(id) ON DELETE SET NULL,
  item_id TEXT NOT NULL REFERENCES item_catalog(item_id),
  location TEXT NOT NULL DEFAULT 'IN_BAG',
  slot INTEGER,
  equip_slot INTEGER,
  rarity INTEGER NOT NULL DEFAULT 0,
  level INTEGER NOT NULL DEFAULT 1,
  enhance_level INTEGER NOT NULL DEFAULT 0,
  durability INTEGER,
  max_durability INTEGER,
  affixes JSONB NOT NULL DEFAULT '[]'::jsonb,
  bind_type TEXT NOT NULL DEFAULT 'BOUND',
  minted_nft_id BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_equipment_items_uid UNIQUE (equipment_uid),
  CONSTRAINT uq_equipment_items_hash UNIQUE (equipment_hash),
  CONSTRAINT chk_equipment_items_location
    CHECK (location IN (
      'IN_BAG',
      'EQUIPPED',
      'IN_WAREHOUSE',
      'IN_LOOT_TRAY',
      'LOCKED_FOR_MINT',
      'MINT_PENDING',
      'ON_CHAIN',
      'LISTED',
      'MARKET_CLAIM_PENDING',
      'CONSUMED',
      'DELETED',
      'BURNED'
    )),
  CONSTRAINT chk_equipment_items_bind_type
    CHECK (bind_type IN ('BOUND', 'ACCOUNT_BOUND', 'UNBOUND'))
);

CREATE INDEX IF NOT EXISTS idx_equipment_items_account_location
  ON equipment_items(account_id, location);

CREATE INDEX IF NOT EXISTS idx_equipment_items_character_location
  ON equipment_items(character_id, location);

CREATE UNIQUE INDEX IF NOT EXISTS uq_equipment_items_character_bag_slot
  ON equipment_items(character_id, location, slot)
  WHERE character_id IS NOT NULL
    AND slot IS NOT NULL
    AND location IN ('IN_BAG', 'IN_WAREHOUSE');

CREATE UNIQUE INDEX IF NOT EXISTS uq_equipment_items_character_equip_slot
  ON equipment_items(character_id, equip_slot)
  WHERE character_id IS NOT NULL
    AND equip_slot IS NOT NULL
    AND location = 'EQUIPPED';

CREATE TABLE IF NOT EXISTS nft_assets (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  source_asset_type TEXT NOT NULL,
  source_asset_id BIGINT NOT NULL,
  mint_address TEXT,
  metadata_uri TEXT,
  status TEXT NOT NULL DEFAULT 'OFFCHAIN',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  minted_at TIMESTAMPTZ,
  CONSTRAINT chk_nft_assets_source_type
    CHECK (source_asset_type IN ('EQUIPMENT', 'RARE_ITEM')),
  CONSTRAINT chk_nft_assets_status
    CHECK (status IN ('OFFCHAIN', 'MINT_REQUESTED', 'MINTED', 'BURNED', 'CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_nft_assets_account
  ON nft_assets(account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS nft_mint_requests (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  nft_asset_id BIGINT REFERENCES nft_assets(id) ON DELETE SET NULL,
  source_asset_type TEXT NOT NULL,
  source_asset_id BIGINT NOT NULL,
  mint_fee_token NUMERIC(38, 0) NOT NULL,
  status TEXT NOT NULL DEFAULT 'REQUESTED',
  tx_signature TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  submitted_at TIMESTAMPTZ,
  confirmed_at TIMESTAMPTZ,
  CONSTRAINT chk_nft_mint_requests_status
    CHECK (status IN ('REQUESTED', 'PAID', 'SUBMITTED', 'CONFIRMED', 'FAILED', 'CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_nft_mint_requests_status
  ON nft_mint_requests(status, created_at);

CREATE TABLE IF NOT EXISTS loot_tray_items (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  item_type TEXT NOT NULL,
  item_id TEXT,
  equipment_id BIGINT REFERENCES equipment_items(id) ON DELETE SET NULL,
  quantity BIGINT NOT NULL DEFAULT 1,
  source TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'PENDING',
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  claimed_at TIMESTAMPTZ,
  CONSTRAINT chk_loot_tray_items_status
    CHECK (status IN ('PENDING', 'CLAIMED', 'DISCARDED', 'EXPIRED'))
);

CREATE INDEX IF NOT EXISTS idx_loot_tray_items_character_status
  ON loot_tray_items(character_id, status, created_at DESC);

-- ---------------------------------------------------------------------------
-- Gameplay settlement records
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS dungeon_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  dungeon_key TEXT NOT NULL,
  difficulty TEXT,
  status TEXT NOT NULL DEFAULT 'STARTED',
  enter_cost JSONB NOT NULL DEFAULT '{}'::jsonb,
  result JSONB NOT NULL DEFAULT '{}'::jsonb,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  CONSTRAINT chk_dungeon_runs_status
    CHECK (status IN ('STARTED', 'FINISHED', 'FAILED', 'CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_dungeon_runs_character
  ON dungeon_runs(character_id, started_at DESC);

CREATE TABLE IF NOT EXISTS gathering_settlements (
  id BIGSERIAL PRIMARY KEY,
  op_id TEXT NOT NULL UNIQUE,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  node_key TEXT NOT NULL,
  rewards JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gathering_settlements_character
  ON gathering_settlements(character_id, created_at DESC);

CREATE TABLE IF NOT EXISTS boss_events (
  id BIGSERIAL PRIMARY KEY,
  boss_key TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'OPEN',
  starts_at TIMESTAMPTZ NOT NULL,
  ends_at TIMESTAMPTZ NOT NULL,
  reward_pool JSONB NOT NULL DEFAULT '{}'::jsonb,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_boss_events_status
    CHECK (status IN ('OPEN', 'SETTLING', 'SETTLED', 'CANCELLED'))
);

CREATE TABLE IF NOT EXISTS boss_contributions (
  id BIGSERIAL PRIMARY KEY,
  boss_event_id BIGINT NOT NULL REFERENCES boss_events(id) ON DELETE CASCADE,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  contribution BIGINT NOT NULL DEFAULT 0,
  reward JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(boss_event_id, account_id)
);

-- ---------------------------------------------------------------------------
-- Marketplace
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS marketplace_listings (
  id BIGSERIAL PRIMARY KEY,
  seller_account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  seller_character_id BIGINT REFERENCES characters(id) ON DELETE SET NULL,
  asset_type TEXT NOT NULL,
  asset_id BIGINT NOT NULL,
  item_id TEXT,
  quantity BIGINT NOT NULL DEFAULT 1,
  price_token NUMERIC(38, 0) NOT NULL,
  listing_deposit_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  fee_bps INTEGER NOT NULL DEFAULT 500,
  status TEXT NOT NULL DEFAULT 'LISTED',
  op_id TEXT,
  cancelled_at TIMESTAMPTZ,
  sold_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_marketplace_listings_asset_type
    CHECK (asset_type IN ('ITEM', 'EQUIPMENT', 'RARE_ASSET')),
  CONSTRAINT chk_marketplace_listings_status
    CHECK (status IN ('LISTED', 'LOCKED', 'SOLD', 'CANCELLED', 'CLAIM_PENDING')),
  CONSTRAINT chk_marketplace_listings_quantity CHECK (quantity > 0),
  CONSTRAINT chk_marketplace_listings_price CHECK (price_token > 0)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_listings_status
  ON marketplace_listings(status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_marketplace_listings_seller
  ON marketplace_listings(seller_account_id, status, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_marketplace_listings_op_id
  ON marketplace_listings(op_id)
  WHERE op_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_marketplace_listing_equipment
  ON marketplace_listings(asset_id)
  WHERE asset_type = 'EQUIPMENT' AND status IN ('LISTED', 'LOCKED');

CREATE TABLE IF NOT EXISTS marketplace_orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  listing_id BIGINT NOT NULL REFERENCES marketplace_listings(id) ON DELETE CASCADE,
  buyer_account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  buyer_character_id BIGINT REFERENCES characters(id) ON DELETE SET NULL,
  amount_token NUMERIC(38, 0) NOT NULL,
  fee_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  burn_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  treasury_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  rewards_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  seller_proceeds_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  deposit_returned_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  spend_source TEXT,
  status TEXT NOT NULL DEFAULT 'PENDING_PAYMENT',
  payment_ref TEXT,
  op_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  CONSTRAINT chk_marketplace_orders_status
    CHECK (status IN ('PENDING_PAYMENT', 'SUBMITTED', 'CONFIRMED', 'COMPLETED', 'EXPIRED', 'CANCELLED', 'ANOMALY'))
);

CREATE INDEX IF NOT EXISTS idx_marketplace_orders_buyer
  ON marketplace_orders(buyer_account_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_marketplace_orders_op_id
  ON marketplace_orders(op_id)
  WHERE op_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS account_market_slots (
  account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  material_expand_count INTEGER NOT NULL DEFAULT 0,
  wallet_expand_count INTEGER NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_account_market_slots_nonnegative
    CHECK (material_expand_count >= 0 AND wallet_expand_count >= 0)
);

CREATE TABLE IF NOT EXISTS account_market_restrictions (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  restriction_type TEXT NOT NULL,
  reason TEXT,
  created_by TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  CONSTRAINT chk_account_market_restrictions_type
    CHECK (restriction_type IN ('BUY', 'SELL', 'ALL'))
);

CREATE INDEX IF NOT EXISTS idx_account_market_restrictions_active
  ON account_market_restrictions(account_id, revoked_at, expires_at);

CREATE TABLE IF NOT EXISTS economy_payment_orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT REFERENCES characters(id) ON DELETE SET NULL,
  purpose TEXT NOT NULL,
  pay_asset TEXT NOT NULL DEFAULT 'AEB',
  amount NUMERIC(38, 0) NOT NULL,
  receiver_wallet TEXT,
  status TEXT NOT NULL DEFAULT 'PENDING_PAYMENT',
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  tx_signature TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  submitted_at TIMESTAMPTZ,
  confirmed_at TIMESTAMPTZ,
  fulfilled_at TIMESTAMPTZ,
  CONSTRAINT chk_economy_payment_orders_status
    CHECK (status IN ('PENDING_PAYMENT', 'SUBMITTED', 'CONFIRMED', 'FULFILLED', 'EXPIRED', 'CANCELLED', 'ANOMALY'))
);

CREATE INDEX IF NOT EXISTS idx_economy_payment_orders_account
  ON economy_payment_orders(account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_economy_payment_orders_status
  ON economy_payment_orders(status, created_at);

CREATE UNIQUE INDEX IF NOT EXISTS uq_economy_payment_orders_tx_signature
  ON economy_payment_orders(tx_signature)
  WHERE tx_signature IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Withdrawals, Solana deposits and payouts
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS withdrawals (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  wallet TEXT NOT NULL,
  amount NUMERIC(38, 0) NOT NULL,
  status TEXT NOT NULL DEFAULT 'QUEUED',
  reason TEXT,
  tx_signature TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  reviewed_at TIMESTAMPTZ,
  processed_at TIMESTAMPTZ,
  confirmed_at TIMESTAMPTZ,
  CONSTRAINT chk_withdrawals_status
    CHECK (status IN ('QUEUED', 'MANUAL_REVIEW', 'REJECTED', 'SUBMITTED', 'CONFIRMED', 'FAILED', 'CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_withdrawals_status_created
  ON withdrawals(status, created_at);

CREATE INDEX IF NOT EXISTS idx_withdrawals_account
  ON withdrawals(account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS solana_deposits (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
  wallet TEXT NOT NULL,
  token_mint TEXT NOT NULL,
  amount NUMERIC(38, 0) NOT NULL,
  signature TEXT NOT NULL UNIQUE,
  slot BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'PENDING',
  observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  credited_at TIMESTAMPTZ,
  CONSTRAINT chk_solana_deposits_status
    CHECK (status IN ('PENDING', 'CREDITED', 'IGNORED', 'REORGED', 'PAYMENT_MATCHED'))
);

CREATE INDEX IF NOT EXISTS idx_solana_deposits_wallet
  ON solana_deposits(wallet, observed_at DESC);

CREATE TABLE IF NOT EXISTS solana_payouts (
  id BIGSERIAL PRIMARY KEY,
  withdrawal_id BIGINT REFERENCES withdrawals(id) ON DELETE SET NULL,
  wallet TEXT NOT NULL,
  token_mint TEXT NOT NULL,
  amount NUMERIC(38, 0) NOT NULL,
  signature TEXT UNIQUE,
  status TEXT NOT NULL DEFAULT 'CREATED',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  submitted_at TIMESTAMPTZ,
  confirmed_at TIMESTAMPTZ,
  CONSTRAINT chk_solana_payouts_status
    CHECK (status IN ('CREATED', 'SUBMITTED', 'CONFIRMED', 'FAILED', 'CANCELLED'))
);

CREATE INDEX IF NOT EXISTS idx_solana_payouts_status
  ON solana_payouts(status, created_at);

CREATE TABLE IF NOT EXISTS chain_cursors (
  name TEXT PRIMARY KEY,
  network TEXT NOT NULL,
  cursor_slot BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'OK',
  lag_slots BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS hot_wallet_status (
  wallet TEXT PRIMARY KEY,
  network TEXT NOT NULL,
  token_mint TEXT,
  balance NUMERIC(38, 0) NOT NULL DEFAULT 0,
  low_balance_threshold NUMERIC(38, 0) NOT NULL DEFAULT 0,
  payouts_paused BOOLEAN NOT NULL DEFAULT FALSE,
  last_checked_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO chain_cursors (name, network, cursor_slot, status)
VALUES ('solana_deposits', 'solana-devnet', 0, 'OK')
ON CONFLICT (name) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Risk control
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS account_risk_events (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL,
  severity INTEGER NOT NULL DEFAULT 0,
  device_id TEXT,
  ip_address INET,
  wallet TEXT,
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_risk_events_account
  ON account_risk_events(account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_risk_events_device
  ON account_risk_events(device_id, created_at DESC);

CREATE TABLE IF NOT EXISTS account_links (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  linked_account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  link_type TEXT NOT NULL,
  strength INTEGER NOT NULL DEFAULT 0,
  evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(account_id, linked_account_id, link_type)
);

-- ---------------------------------------------------------------------------
-- Revenue, treasury and operations accounting
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS revenue_events (
  id BIGSERIAL PRIMARY KEY,
  source TEXT NOT NULL,
  asset TEXT NOT NULL,
  amount NUMERIC(38, 0) NOT NULL,
  tx_signature TEXT,
  status TEXT NOT NULL DEFAULT 'RECORDED',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_revenue_events_status
    CHECK (status IN ('RECORDED', 'ALLOCATED', 'CANCELLED'))
);

CREATE TABLE IF NOT EXISTS revenue_allocations (
  id BIGSERIAL PRIMARY KEY,
  revenue_event_id BIGINT REFERENCES revenue_events(id) ON DELETE SET NULL,
  bucket TEXT NOT NULL,
  amount NUMERIC(38, 0) NOT NULL,
  status TEXT NOT NULL DEFAULT 'PLANNED',
  operator_id TEXT,
  reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  executed_at TIMESTAMPTZ,
  CONSTRAINT chk_revenue_allocations_status
    CHECK (status IN ('PLANNED', 'EXECUTED', 'CANCELLED'))
);

-- ---------------------------------------------------------------------------
-- Admin, config and audit
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS admin_users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  role TEXT NOT NULL DEFAULT 'OPERATOR',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_login_at TIMESTAMPTZ,
  CONSTRAINT chk_admin_users_status
    CHECK (status IN ('ACTIVE', 'DISABLED')),
  CONSTRAINT chk_admin_users_role
    CHECK (role IN ('SUPER_ADMIN', 'OPERATOR', 'FINANCE', 'SUPPORT', 'VIEWER'))
);

CREATE TABLE IF NOT EXISTS admin_audit_logs (
  id BIGSERIAL PRIMARY KEY,
  admin_id TEXT,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT,
  before_value JSONB,
  after_value JSONB,
  reason TEXT,
  ip_address INET,
  user_agent TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_audit_logs_target
  ON admin_audit_logs(target_type, target_id, created_at DESC);

CREATE TABLE IF NOT EXISTS economy_config_versions (
  id BIGSERIAL PRIMARY KEY,
  config_key TEXT NOT NULL,
  config_value JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'DRAFT',
  created_by TEXT,
  reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  activated_at TIMESTAMPTZ,
  CONSTRAINT chk_economy_config_versions_status
    CHECK (status IN ('DRAFT', 'ACTIVE', 'ARCHIVED'))
);

CREATE INDEX IF NOT EXISTS idx_economy_config_versions_key_status
  ON economy_config_versions(config_key, status);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_economy_config_active
  ON economy_config_versions(config_key)
  WHERE status = 'ACTIVE';

-- ---------------------------------------------------------------------------
-- Initial operational defaults
-- ---------------------------------------------------------------------------

INSERT INTO economy_config_versions (config_key, config_value, status, reason, activated_at)
VALUES
  (
    'chain.default',
    '{"network":"solana-devnet","tokenSymbol":"AEB","tokenDecimals":9}'::jsonb,
    'ACTIVE',
    'Initial Solana-first chain default',
    NOW()
  ),
  (
    'withdrawal.limits',
    '{"singleAutoMax":"5000","userDailyAutoMax":"20000","globalHourlyAutoMax":"30000","globalDailyAutoMax":"150000"}'::jsonb,
    'ACTIVE',
    'Conservative launch withdrawal limits',
    NOW()
  ),
  (
    'game.cooldownTiers',
    '{"tiers":[{"tier":0,"spend":"0","cooldownHours":74},{"tier":1,"spend":"10000","cooldownHours":68},{"tier":2,"spend":"50000","cooldownHours":60},{"tier":3,"spend":"200000","cooldownHours":48},{"tier":4,"spend":"500000","cooldownHours":36},{"tier":5,"spend":"1000000","cooldownHours":24}]}'::jsonb,
    'ACTIVE',
    'Initial contribution tier cooldown table',
    NOW()
  ),
  (
    'gold.convert',
    '{"goldPerGame":"10","userDailyGameMin":"100","userDailyGameMax":"300","globalDailyGameMin":"10000","globalDailyGameMax":"20000","storageDays":5}'::jsonb,
    'ACTIVE',
    'Initial Gold to GAME conversion guardrails',
    NOW()
  )
ON CONFLICT DO NOTHING;

INSERT INTO schema_migrations (version)
VALUES ('aeonblight_full_schema')
ON CONFLICT DO NOTHING;
