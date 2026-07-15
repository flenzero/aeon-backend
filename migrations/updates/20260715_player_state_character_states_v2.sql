-- Ensure player runtime saves live in character_states even if an earlier
-- player_state_and_slots migration was already applied locally.

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

DO $$
DECLARE
  has_map_id BOOLEAN;
  has_position BOOLEAN;
  has_play_time_sec BOOLEAN;
  has_hunger BOOLEAN;
  has_last_played_at BOOLEAN;
  backfill_sql TEXT;
BEGIN
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'characters' AND column_name = 'map_id'
  ) INTO has_map_id;
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'characters' AND column_name = 'position'
  ) INTO has_position;
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'characters' AND column_name = 'play_time_sec'
  ) INTO has_play_time_sec;
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'characters' AND column_name = 'hunger'
  ) INTO has_hunger;
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'characters' AND column_name = 'last_played_at'
  ) INTO has_last_played_at;

  backfill_sql := 'INSERT INTO character_states (
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
      ' || CASE WHEN has_map_id THEN 'map_id' ELSE 'NULL::text' END || ',
      ' || CASE WHEN has_position THEN 'position' ELSE '''{}''::jsonb' END || ',
      ' || CASE WHEN has_play_time_sec THEN 'play_time_sec' ELSE '0' END || ',
      ' || CASE WHEN has_hunger THEN 'hunger' ELSE '100' END || ',
      ' || CASE WHEN has_last_played_at THEN 'last_played_at' ELSE 'NULL::timestamptz' END || ',
      NOW(),
      NOW()
    FROM characters
    ON CONFLICT (character_id) DO NOTHING';

  EXECUTE backfill_sql;
END $$;

CREATE INDEX IF NOT EXISTS idx_character_states_last_played
  ON character_states(last_played_at DESC);

ALTER TABLE characters
  DROP COLUMN IF EXISTS map_id,
  DROP COLUMN IF EXISTS position,
  DROP COLUMN IF EXISTS play_time_sec,
  DROP COLUMN IF EXISTS hunger,
  DROP COLUMN IF EXISTS last_played_at;

INSERT INTO schema_migrations(version)
VALUES ('20260715_player_state_character_states_v2')
ON CONFLICT (version) DO NOTHING;
