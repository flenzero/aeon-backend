-- Marketplace v1 + token column rename + LISTED inventory + market slots/restrictions.
-- Safe to run once against an existing Aeonblight database.

-- ---------------------------------------------------------------------------
-- Token naming: *_game* → *_token* (keep game_servers / game_tickets untouched)
-- ---------------------------------------------------------------------------

ALTER TABLE account_tokens RENAME COLUMN game_balance TO token_balance;

ALTER TABLE locked_game_records RENAME TO locked_token_records;
ALTER INDEX IF EXISTS idx_locked_game_unlock RENAME TO idx_locked_token_unlock;
ALTER INDEX IF EXISTS idx_locked_game_account RENAME TO idx_locked_token_account;
ALTER TABLE locked_token_records RENAME CONSTRAINT chk_locked_game_records_amount TO chk_locked_token_records_amount;
ALTER TABLE locked_token_records RENAME CONSTRAINT chk_locked_game_records_status TO chk_locked_token_records_status;

ALTER TABLE system_consumptions RENAME COLUMN amount_game TO amount_token;
ALTER TABLE system_consumptions DROP CONSTRAINT IF EXISTS chk_system_consumptions_spend_source;
ALTER TABLE system_consumptions DROP CONSTRAINT IF EXISTS chk_system_consumptions_amounts;
UPDATE system_consumptions
SET spend_source = CASE spend_source
  WHEN 'LOCKED_GAME' THEN 'LOCKED_TOKEN'
  WHEN 'WITHDRAWABLE_GAME' THEN 'WITHDRAWABLE_TOKEN'
  WHEN 'EXTERNAL_GAME' THEN 'EXTERNAL_TOKEN'
  ELSE spend_source
END;
ALTER TABLE system_consumptions
  ADD CONSTRAINT chk_system_consumptions_spend_source
    CHECK (spend_source IN ('LOCKED_TOKEN', 'WITHDRAWABLE_TOKEN', 'EXTERNAL_TOKEN', 'MIXED'));
ALTER TABLE system_consumptions
  ADD CONSTRAINT chk_system_consumptions_amounts
    CHECK (
      amount_token > 0
      AND burn_amount >= 0
      AND recycle_amount >= 0
      AND reward_pool_amount >= 0
      AND effective_contribution_amount >= 0
      AND unlock_credit_created >= 0
    );

ALTER TABLE gold_conversion_windows RENAME COLUMN minted_game TO minted_token;
ALTER TABLE nft_mint_requests RENAME COLUMN mint_fee_game TO mint_fee_token;

ALTER TABLE marketplace_listings RENAME COLUMN price_game TO price_token;
ALTER TABLE marketplace_orders RENAME COLUMN amount_game TO amount_token;
ALTER TABLE marketplace_orders RENAME COLUMN fee_game TO fee_token;

ALTER TABLE economy_payment_orders ALTER COLUMN pay_asset SET DEFAULT 'AEB';

ALTER TABLE account_tokens DROP CONSTRAINT IF EXISTS chk_account_tokens_nonnegative;
ALTER TABLE account_tokens
  ADD CONSTRAINT chk_account_tokens_nonnegative
    CHECK (
      token_balance >= 0
      AND withdrawable_balance >= 0
      AND locked_balance >= 0
      AND external_balance >= 0
      AND unlock_credit >= 0
    );

-- ---------------------------------------------------------------------------
-- Catalog / inventory LISTED
-- ---------------------------------------------------------------------------

ALTER TABLE item_catalog
  ADD COLUMN IF NOT EXISTS default_bind_type TEXT NOT NULL DEFAULT 'BOUND';

ALTER TABLE item_catalog DROP CONSTRAINT IF EXISTS chk_item_catalog_default_bind_type;
ALTER TABLE item_catalog
  ADD CONSTRAINT chk_item_catalog_default_bind_type
    CHECK (default_bind_type IN ('BOUND', 'ACCOUNT_BOUND', 'UNBOUND'));

ALTER TABLE inventory_items DROP CONSTRAINT IF EXISTS chk_inventory_items_location;
ALTER TABLE inventory_items
  ADD CONSTRAINT chk_inventory_items_location
    CHECK (location IN ('BAG', 'WAREHOUSE', 'LOOT_TRAY', 'LISTED', 'CONSUMED', 'DELETED'));

DROP INDEX IF EXISTS uq_inventory_items_character_location_slot;
CREATE UNIQUE INDEX IF NOT EXISTS uq_inventory_items_character_location_slot
  ON inventory_items(character_id, location, slot)
  WHERE slot IS NOT NULL AND location IN ('BAG', 'WAREHOUSE', 'LOOT_TRAY');

-- ---------------------------------------------------------------------------
-- Marketplace listing settlement fields
-- ---------------------------------------------------------------------------

ALTER TABLE marketplace_listings
  ADD COLUMN IF NOT EXISTS item_id TEXT,
  ADD COLUMN IF NOT EXISTS quantity BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS listing_deposit_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS fee_bps INTEGER NOT NULL DEFAULT 500,
  ADD COLUMN IF NOT EXISTS op_id TEXT,
  ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS sold_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS uq_marketplace_listings_op_id
  ON marketplace_listings(op_id)
  WHERE op_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_marketplace_listing_equipment
  ON marketplace_listings(asset_id)
  WHERE asset_type = 'EQUIPMENT' AND status IN ('LISTED', 'LOCKED');

CREATE INDEX IF NOT EXISTS idx_marketplace_listings_seller
  ON marketplace_listings(seller_account_id, status, created_at DESC);

ALTER TABLE marketplace_orders
  ADD COLUMN IF NOT EXISTS burn_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS treasury_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS rewards_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS seller_proceeds_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS deposit_returned_token NUMERIC(38, 0) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS op_id TEXT,
  ADD COLUMN IF NOT EXISTS spend_source TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS uq_marketplace_orders_op_id
  ON marketplace_orders(op_id)
  WHERE op_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Account market slots + restrictions (BNBLAND-style gates)
-- ---------------------------------------------------------------------------

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

INSERT INTO schema_migrations (version)
VALUES ('20260710_marketplace_v1')
ON CONFLICT DO NOTHING;
