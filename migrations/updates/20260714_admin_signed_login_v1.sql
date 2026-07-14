ALTER TABLE admin_users
  ADD COLUMN IF NOT EXISTS public_key TEXT,
  ADD COLUMN IF NOT EXISTS created_by TEXT,
  ADD COLUMN IF NOT EXISTS disabled_by TEXT,
  ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS disable_reason TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS uq_admin_users_public_key
  ON admin_users(public_key)
  WHERE public_key IS NOT NULL AND public_key <> '';

CREATE TABLE IF NOT EXISTS admin_login_nonces (
  nonce TEXT PRIMARY KEY,
  admin_id TEXT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  message TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'PENDING',
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  consumed_at TIMESTAMPTZ,
  CONSTRAINT chk_admin_login_nonces_status
    CHECK (status IN ('PENDING', 'CONSUMED', 'EXPIRED'))
);

CREATE INDEX IF NOT EXISTS idx_admin_login_nonces_admin
  ON admin_login_nonces(admin_id, created_at DESC);

INSERT INTO schema_migrations(version)
VALUES ('20260714_admin_signed_login_v1')
ON CONFLICT DO NOTHING;
