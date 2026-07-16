CREATE TABLE IF NOT EXISTS mystery_shop_boards (
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  shop_id TEXT NOT NULL,
  next_free_refresh_at TIMESTAMPTZ NOT NULL,
  generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  offers JSONB NOT NULL DEFAULT '[]'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (character_id, shop_id)
);

CREATE INDEX IF NOT EXISTS idx_mystery_shop_boards_account
  ON mystery_shop_boards(account_id, character_id);

INSERT INTO schema_migrations (version)
VALUES ('20260715_mystery_shop_v1')
ON CONFLICT (version) DO NOTHING;
