BEGIN;

ALTER TABLE characters
  ADD COLUMN IF NOT EXISTS highest_cleared_chapter INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS highest_cleared_floor INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS dungeon_clear_count BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_dungeon_cleared_at TIMESTAMPTZ;

WITH progress AS (
  SELECT character_id,
    COUNT(*) FILTER (WHERE status = 'FINISHED' AND result->>'result' = 'victory') AS clear_count,
    MAX(finished_at) FILTER (WHERE status = 'FINISHED' AND result->>'result' = 'victory') AS last_cleared_at,
    MAX((regexp_match(dungeon_key, '^chapter:([0-9]+):floor:([0-9]+)$'))[1]::int * 1000000 +
        (regexp_match(dungeon_key, '^chapter:([0-9]+):floor:([0-9]+)$'))[2]::int)
      FILTER (WHERE status = 'FINISHED' AND result->>'result' = 'victory') AS max_progress
  FROM dungeon_runs
  GROUP BY character_id
)
UPDATE characters c
SET highest_cleared_chapter = COALESCE(p.max_progress / 1000000, 0),
    highest_cleared_floor = COALESCE(p.max_progress % 1000000, 0),
    dungeon_clear_count = COALESCE(p.clear_count, 0),
    last_dungeon_cleared_at = p.last_cleared_at
FROM progress p
WHERE c.id = p.character_id;

CREATE INDEX IF NOT EXISTS idx_characters_compensation
  ON characters(is_deleted, level, highest_cleared_chapter, highest_cleared_floor);

CREATE TABLE IF NOT EXISTS admin_operation_previews (
  preview_id TEXT PRIMARY KEY,
  admin_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  character_id BIGINT REFERENCES characters(id) ON DELETE CASCADE,
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'PENDING',
  expires_at TIMESTAMPTZ NOT NULL,
  committed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_admin_operation_previews_kind CHECK (kind IN ('COMPENSATION', 'LOTTERY')),
  CONSTRAINT chk_admin_operation_previews_status CHECK (status IN ('PENDING', 'COMMITTED', 'EXPIRED'))
);

CREATE TABLE IF NOT EXISTS admin_operation_preview_targets (
  preview_id TEXT NOT NULL REFERENCES admin_operation_previews(preview_id) ON DELETE CASCADE,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  PRIMARY KEY (preview_id, character_id)
);

CREATE INDEX IF NOT EXISTS idx_admin_operation_previews_active
  ON admin_operation_previews(admin_id, kind, status, expires_at DESC);

INSERT INTO schema_migrations (version)
VALUES ('20260714_admin_compensation_v1')
ON CONFLICT (version) DO NOTHING;

COMMIT;
