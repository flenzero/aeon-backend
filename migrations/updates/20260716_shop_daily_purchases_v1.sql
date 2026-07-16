CREATE TABLE IF NOT EXISTS shop_daily_purchases (
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  shop_id TEXT NOT NULL,
  slot_index INTEGER NOT NULL CHECK (slot_index > 0),
  business_date DATE NOT NULL,
  quantity BIGINT NOT NULL DEFAULT 0 CHECK (quantity >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (character_id, shop_id, slot_index, business_date)
);

CREATE INDEX IF NOT EXISTS idx_shop_daily_purchases_account
  ON shop_daily_purchases(account_id, character_id, business_date);

INSERT INTO schema_migrations (version)
VALUES ('20260716_shop_daily_purchases_v1')
ON CONFLICT (version) DO NOTHING;
