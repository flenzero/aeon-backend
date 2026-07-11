BEGIN;

ALTER TABLE equipment_items
  DROP CONSTRAINT IF EXISTS chk_equipment_items_location;

ALTER TABLE equipment_items
  ADD CONSTRAINT chk_equipment_items_location
    CHECK (location IN (
      'IN_BAG',
      'EQUIPPED',
      'IN_WAREHOUSE',
      'IN_LOOT_TRAY',
      'LOCKED_FOR_MINT',
      'MINT_PENDING',
      'ON_CHAIN',
      'LISTED',
      'MARKET_CLAIM_PENDING',
      'CONSUMED',
      'DELETED',
      'BURNED'
    ));

COMMIT;
