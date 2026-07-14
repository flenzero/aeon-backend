BEGIN;

ALTER TABLE dungeon_runs
  ADD COLUMN IF NOT EXISTS origin_server_id TEXT REFERENCES game_servers(server_id) ON DELETE SET NULL;

WITH ranked AS (
  SELECT id, ROW_NUMBER() OVER (PARTITION BY character_id ORDER BY started_at DESC, id DESC) AS row_num
  FROM dungeon_runs
  WHERE status = 'STARTED'
)
UPDATE dungeon_runs d
SET status = 'CANCELLED',
    result = d.result || '{"result":"abandoned","reason":"duplicate active run closed by dungeon recovery migration"}'::jsonb,
    finished_at = COALESCE(d.finished_at, NOW())
FROM ranked r
WHERE d.id = r.id AND r.row_num > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_dungeon_runs_active_character
  ON dungeon_runs(character_id)
  WHERE status = 'STARTED';

CREATE INDEX IF NOT EXISTS idx_dungeon_runs_origin_status
  ON dungeon_runs(origin_server_id, status, started_at DESC);

INSERT INTO schema_migrations (version)
VALUES ('20260712_dungeon_recovery_v1')
ON CONFLICT DO NOTHING;

COMMIT;
