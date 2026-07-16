-- Re-map equipment equip_slot values to the UI order:
-- left1 weapon, right1 helmet, left2 chest, right2 cloak,
-- left3 gloves, right3 accessory, left4 shoes, right4 mount.
--
-- This updates equipment_items.equip_slot only. Do not touch slot: that column
-- is the bag/warehouse grid position, not the equipment slot enum.

UPDATE equipment_items
SET equip_slot = CASE equip_slot
  WHEN 4 THEN 0 -- weapon
  WHEN 2 THEN 1 -- helmet
  WHEN 3 THEN 2 -- chest
  WHEN 5 THEN 3 -- cloak
  WHEN 0 THEN 4 -- gloves
  WHEN 7 THEN 5 -- accessory
  WHEN 1 THEN 6 -- shoes
  WHEN 6 THEN 7 -- mount
  ELSE equip_slot
END,
updated_at = NOW()
WHERE equip_slot IN (0, 1, 2, 3, 4, 5, 6, 7);

INSERT INTO schema_migrations(version)
VALUES ('20260715_equipment_slot_ui_order_v1')
ON CONFLICT (version) DO NOTHING;
