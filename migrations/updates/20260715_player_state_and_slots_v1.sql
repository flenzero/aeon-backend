-- Character selection slots and first-class player state table.

ALTER TABLE characters
  ADD COLUMN IF NOT EXISTS slot_index INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS highest_cleared_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS character_states (
  character_id BIGINT PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
  map_id TEXT,
  position JSONB NOT NULL DEFAULT '{}'::jsonb,
  play_time_sec BIGINT NOT NULL DEFAULT 0,
  hunger NUMERIC(8,2) NOT NULL DEFAULT 100,
  last_played_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_character_states_play_time_nonnegative
    CHECK (play_time_sec >= 0),
  CONSTRAINT chk_character_states_hunger_nonnegative
    CHECK (hunger >= 0)
);

WITH ranked AS (
  SELECT
    id,
    ROW_NUMBER() OVER (PARTITION BY account_id ORDER BY id) - 1 AS next_slot
  FROM characters
  WHERE is_deleted = FALSE
)
UPDATE characters c
SET slot_index = ranked.next_slot
FROM ranked
WHERE c.id = ranked.id;

INSERT INTO character_states (
  character_id,
  map_id,
  position,
  play_time_sec,
  hunger,
  last_played_at,
  created_at,
  updated_at
)
SELECT
  id,
  map_id,
  position,
  0,
  100,
  NULL,
  NOW(),
  NOW()
FROM characters
ON CONFLICT (character_id) DO NOTHING;

UPDATE characters
SET highest_cleared_at = last_dungeon_cleared_at
WHERE highest_cleared_at IS NULL
  AND highest_cleared_floor > 0
  AND last_dungeon_cleared_at IS NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'chk_characters_slot_index'
  ) THEN
    ALTER TABLE characters
      ADD CONSTRAINT chk_characters_slot_index CHECK (slot_index >= 0);
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_characters_account_slot_active
  ON characters(account_id, slot_index)
  WHERE is_deleted = FALSE;

CREATE INDEX IF NOT EXISTS idx_character_states_last_played
  ON character_states(last_played_at DESC);

ALTER TABLE characters
  DROP COLUMN IF EXISTS map_id,
  DROP COLUMN IF EXISTS position,
  DROP COLUMN IF EXISTS play_time_sec,
  DROP COLUMN IF EXISTS hunger,
  DROP COLUMN IF EXISTS last_played_at;

INSERT INTO schema_migrations(version)
VALUES ('20260715_player_state_and_slots_v1')
ON CONFLICT (version) DO NOTHING;
