BEGIN;

CREATE TABLE IF NOT EXISTS service_identities (
  service_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  subject_id TEXT,
  public_key TEXT NOT NULL UNIQUE,
  capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  disabled_by TEXT,
  disabled_at TIMESTAMPTZ,
  disable_reason TEXT,
  CONSTRAINT chk_service_identities_kind CHECK (kind IN ('GAME_SERVER', 'WORKER', 'CHAIN_OPERATOR', 'MINT_OPERATOR', 'OPS')),
  CONSTRAINT chk_service_identities_status CHECK (status IN ('ACTIVE', 'DISABLED'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_game_server_subject
  ON service_identities(subject_id)
  WHERE kind = 'GAME_SERVER' AND status = 'ACTIVE';

CREATE TABLE IF NOT EXISTS service_request_nonces (
  service_id TEXT NOT NULL REFERENCES service_identities(service_id) ON DELETE CASCADE,
  nonce TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(service_id, nonce)
);

CREATE INDEX IF NOT EXISTS idx_service_request_nonces_expiry
  ON service_request_nonces(expires_at);

INSERT INTO schema_migrations (version)
VALUES ('20260712_service_identities_v1')
ON CONFLICT DO NOTHING;

COMMIT;
