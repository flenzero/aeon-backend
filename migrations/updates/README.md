# Aeonblight Database Updates

`../aeonblight_full_schema.sql` is the canonical full database schema. It must
always describe the complete database that a fresh Aeonblight deployment needs.

Put incremental production updates in this directory with stable names, for
example:

```text
20260710_add_equipment_loot_tray_location.sql
20260710_marketplace_v1.sql
20260710_solana_chain_v1.sql
```

Rules:

- update scripts are append-only history
- every accepted update must also be folded back into
  `../aeonblight_full_schema.sql`
- applications and Docker startup never execute schema changes
- technicians use only `scripts/db-migrate.sh bootstrap` for a new database or
  `scripts/db-migrate.sh up` for an existing database
- there is no automatic down migration; use expand/migrate/contract and a
  reviewed forward fix when rollback is required
- update scripts should be safe to run once and record their filename (without
  `.sql`) in `schema_migrations`
