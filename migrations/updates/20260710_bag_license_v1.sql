-- Bag expand counters + account trading license.

ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS has_trading_license BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS trading_license_at TIMESTAMPTZ;

ALTER TABLE characters
  ADD COLUMN IF NOT EXISTS bag_expand_count INTEGER NOT NULL DEFAULT 0;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_characters_bag_expand_nonnegative'
  ) THEN
    ALTER TABLE characters
      ADD CONSTRAINT chk_characters_bag_expand_nonnegative
      CHECK (bag_expand_count >= 0);
  END IF;
END $$;

INSERT INTO schema_migrations (version)
VALUES ('20260710_bag_license_v1')
ON CONFLICT DO NOTHING;
