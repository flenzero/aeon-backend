-- Readiness baseline for explicit runtime profiles and independent migrations.
-- This migration intentionally changes only the required schema marker.

INSERT INTO schema_migrations (version)
VALUES ('20260712_runtime_profiles_v1')
ON CONFLICT DO NOTHING;
