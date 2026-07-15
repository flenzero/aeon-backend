BEGIN;

CREATE TABLE IF NOT EXISTS announcement_templates (
  code TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  title_template TEXT NOT NULL,
  body_template TEXT NOT NULL,
  display_mode TEXT NOT NULL DEFAULT 'BANNER',
  priority INTEGER NOT NULL DEFAULT 50,
  duration_seconds INTEGER NOT NULL DEFAULT 0,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_by TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_announcement_templates_kind
    CHECK (kind IN ('RARE_REWARD', 'OPS_NOTICE')),
  CONSTRAINT chk_announcement_templates_display
    CHECK (display_mode IN ('POPUP', 'BANNER')),
  CONSTRAINT chk_announcement_templates_duration
    CHECK (duration_seconds >= 0)
);

CREATE TABLE IF NOT EXISTS announcements (
  id BIGSERIAL PRIMARY KEY,
  kind TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'ACTIVE',
  template_code TEXT REFERENCES announcement_templates(code) ON DELETE SET NULL,
  display_mode TEXT NOT NULL DEFAULT 'BANNER',
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  priority INTEGER NOT NULL DEFAULT 50,
  scope TEXT NOT NULL DEFAULT 'GLOBAL',
  starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ends_at TIMESTAMPTZ,
  account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
  character_id BIGINT REFERENCES characters(id) ON DELETE SET NULL,
  character_name TEXT,
  event_type TEXT,
  source TEXT,
  ref_type TEXT,
  ref_id TEXT,
  item_id TEXT,
  item_name TEXT,
  equipment_uid TEXT,
  rarity INTEGER,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  dedupe_key TEXT,
  created_by TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_by TEXT,
  revoked_at TIMESTAMPTZ,
  revoke_reason TEXT,
  CONSTRAINT chk_announcements_kind
    CHECK (kind IN ('RARE_REWARD', 'OPS_NOTICE')),
  CONSTRAINT chk_announcements_status
    CHECK (status IN ('ACTIVE', 'REVOKED')),
  CONSTRAINT chk_announcements_display
    CHECK (display_mode IN ('POPUP', 'BANNER')),
  CONSTRAINT chk_announcements_scope
    CHECK (scope IN ('GLOBAL')),
  CONSTRAINT chk_announcements_window
    CHECK (ends_at IS NULL OR ends_at > starts_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_announcements_dedupe
  ON announcements(dedupe_key)
  WHERE dedupe_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_announcements_active
  ON announcements(status, starts_at, ends_at, priority DESC, id);

CREATE INDEX IF NOT EXISTS idx_announcements_kind_created
  ON announcements(kind, created_at DESC);

INSERT INTO announcement_templates (
  code, kind, title_template, body_template, display_mode, priority, duration_seconds, enabled
)
VALUES
  ('rare_equipment', 'RARE_REWARD', '超稀有装备出现', '恭喜 {characterName} 通过{source}获得 {rarity}星装备 {itemName}', 'POPUP', 900, 12, TRUE),
  ('rare_mount', 'RARE_REWARD', '稀有坐骑出现', '恭喜 {characterName} 通过{source}获得稀有坐骑 {itemName}', 'POPUP', 950, 12, TRUE),
  ('ops_notice', 'OPS_NOTICE', '{title}', '{body}', 'BANNER', 500, 0, TRUE)
ON CONFLICT (code) DO NOTHING;

INSERT INTO schema_migrations (version)
VALUES ('20260715_announcements_v1')
ON CONFLICT (version) DO NOTHING;

COMMIT;
