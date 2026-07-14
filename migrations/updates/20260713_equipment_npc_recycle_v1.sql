BEGIN;

ALTER TABLE equipment_items
  ADD COLUMN IF NOT EXISTS npc_recycled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS npc_recycle_expires_at TIMESTAMPTZ;

ALTER TABLE equipment_items
  DROP CONSTRAINT IF EXISTS chk_equipment_items_location;

ALTER TABLE equipment_items
  ADD CONSTRAINT chk_equipment_items_location
    CHECK (location IN (
      'IN_BAG', 'EQUIPPED', 'IN_WAREHOUSE', 'IN_LOOT_TRAY',
      'LOCKED_FOR_MINT', 'MINT_PENDING', 'ON_CHAIN', 'LISTED',
      'MARKET_CLAIM_PENDING', 'CONSUMED', 'DELETED', 'BURNED', 'NPC_RECYCLED'
    ));

CREATE INDEX IF NOT EXISTS idx_equipment_items_npc_recycle_expiry
  ON equipment_items(npc_recycle_expires_at)
  WHERE location = 'NPC_RECYCLED';

INSERT INTO schema_migrations (version)
VALUES ('20260713_equipment_npc_recycle_v1')
ON CONFLICT DO NOTHING;

COMMIT;
