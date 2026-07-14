BEGIN;

ALTER TABLE chain_cursors
  ADD COLUMN IF NOT EXISTS cursor_signature TEXT;

INSERT INTO schema_migrations (version)
VALUES ('20260712_deposit_cursor_v1')
ON CONFLICT DO NOTHING;

COMMIT;
