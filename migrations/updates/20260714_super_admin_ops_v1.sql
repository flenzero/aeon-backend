BEGIN;

ALTER TABLE game_servers
  DROP CONSTRAINT IF EXISTS chk_game_servers_status;

ALTER TABLE game_servers
  ADD CONSTRAINT chk_game_servers_status
  CHECK (status IN ('STARTING', 'ONLINE', 'DRAINING', 'OFFLINE', 'MAINTENANCE', 'DISABLED'));

CREATE TABLE IF NOT EXISTS admin_operation_logs (
  op_id TEXT PRIMARY KEY,
  admin_id TEXT NOT NULL,
  action TEXT NOT NULL,
  target TEXT NOT NULL,
  response JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_operation_logs_created_at
  ON admin_operation_logs(created_at DESC);

INSERT INTO schema_migrations (version)
VALUES ('20260714_super_admin_ops_v1')
ON CONFLICT (version) DO NOTHING;

COMMIT;
