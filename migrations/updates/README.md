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
- Docker fresh initialization mounts only the full schema file, so these update
  scripts are not replayed on top of a complete new database
- update scripts should be safe to run once against an existing database and
  recorded by the operator or migration runner
