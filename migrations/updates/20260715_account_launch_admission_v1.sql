-- Account-level launch admission: homepage tickets bind account/session/server,
-- while character selection happens inside the game client.

ALTER TABLE game_tickets
  ALTER COLUMN character_id DROP NOT NULL;

INSERT INTO schema_migrations (version)
VALUES ('20260715_account_launch_admission_v1')
ON CONFLICT DO NOTHING;
