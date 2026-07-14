BEGIN;

CREATE TABLE IF NOT EXISTS bounty_combat_submissions (
  dungeon_run_id UUID PRIMARY KEY REFERENCES dungeon_runs(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  op_id TEXT NOT NULL UNIQUE,
  kill_count BIGINT NOT NULL CHECK (kill_count > 0),
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO schema_migrations (version)
VALUES ('20260713_bounty_combat_proofs_v1')
ON CONFLICT DO NOTHING;

COMMIT;
