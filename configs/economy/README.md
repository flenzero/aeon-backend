# Aeonblight Economy Config

This directory is the default economy configuration source for `economy-api`.

Runtime path:

```text
ECONOMY_CONFIG_DIR=configs/economy
```

Files:

- `items.json`: canonical item and equipment catalog IDs.
- `shops.json`: shop definitions and item availability lists.
- `economy_rules.json`: shared caps, bag/warehouse sizes, bag expand, trading license, equipment repair, NFT mint fees.
- `marketplace.json`: marketplace fees, slot expands, daily limits.
- `dungeons.json`: chapter/floor entry costs, exp caps, reward pools, and combat scale passthrough.
- `loot_pools.json`: reusable reward pools for dungeons, gathering and farming.
- `equipment_affixes.json`: random equipment affix pools.
- `equipment_templates.json`: equipment series/stages, rarity multipliers, sell/recycle prices and enhancement.
- `gathering.json`: gather node reward configuration.
- `farming.json`: farming crop and harvest reward configuration.
- `bosses.json`: global boss reward pools and contribution tiers.
- `recipes.json`: inventory synthesis recipes for materials and equipment.
- `lottery.json` / `bounties.json`: paid lottery and bounty-board rules.

## Shops

`shops.json` defines merchant inventories. A normal shop becomes usable only
when it contains a `shopId` and either `sellAllItems=true` or the requested
`itemId` is present in `sellItems`.

Normal merchant `sellItems` should use object rows with `slotIndex` and
`dailyLimit`; purchases are limited per character, shop, slot, and business
date. `dailyLimitTimezone` defaults to `Asia/Shanghai`. `buyPrice`, `rarity`,
`minLevel`, and `maxLevel` can be overridden per shop row.

Prices live on `items.json` rows:

- `buyCurrency`: `0` = gold, `1` = AEB chain payment order.
- `buyPrice`: unit purchase price; runtime purchase rejects zero-priced rows.
- `sellPrice`: unit gold credit when selling an inventory item to a shop;
  runtime sale rejects zero-priced rows.
- `grantGold`: optional gold granted after purchase instead of creating an
  inventory/equipment item.

Generated equipment sell prices live in `equipment_templates.json` as
`sellPriceGoldByStage`: the first key is the equipment stage, the nested key is
rarity. Shop selling and NPC recycle both use this stage/rarity table.

## Equipment enums

`equipment_templates.json` owns current equipment slots and weapon subtypes.
`equipSlot` is the character equipment slot; `slot` in runtime responses is the
bag/warehouse grid position.

Equipment slot order follows the character UI from top to bottom, alternating
left/right:

| equipSlot | Key | UI position |
| ---: | --- | --- |
| -1 | none | not equipped |
| 0 | weapon | left 1 |
| 1 | helmet | right 1 |
| 2 | chest | left 2 |
| 3 | cloak | right 2 |
| 4 | gloves | left 3 |
| 5 | accessory | right 3 |
| 6 | shoes | left 4 |
| 7 | mount | right 4 |

Weapon subtype values:

| weaponType | weaponTypeKey |
| ---: | --- |
| 0 | none |
| 1 | sword |
| 2 | axe |
| 3 | bow |
| 4 | staff |

## Dungeons

`dungeons.json` defines three chapters with global floors `1..30`:

| Chapter | Floors | Normal gear stage | Boss floor | Boss ticket |
| --- | --- | --- | --- | --- |
| 0 Ashen Threshold | 1-9 / 10 | t1 | 10 → t5 | `boss_ticket_ashen_threshold` |
| 1 Gloomwood | 11-19 / 20 | t10 | 20 → t15 | `boss_ticket_gloomwood` |
| 2 Voidscar | 21-29 / 30 | t20 | 30 → t25 + t30 | `boss_ticket_voidscar` |

Floor fields owned by economy:

- `enterCost`: gold plus optional boss ticket items (authoritative; request body costs are ignored).
- `maxExp`: submitted exp cap (`20 + floorId * 10`, boss floors ×1.5).
- `lootPoolId`: shared pool id in `loot_pools.json` (banded by ticket drop chance within each chapter).
- `enemyHpScale` / `enemyAtkScale`: combat passthrough for game servers; economy reward logic does not consume them.

Equipment drops reference `equipment_templates.json` item ids. Fixed entry `rarity` is used today (normal floors `1`, boss floors `2`, floor-30 t30 pieces `3`). Weighted rarity rolling from legacy `normalLootTierWeights` is not implemented; extending the loot engine would be required to restore that behavior.

Design notes:

- Game servers submit gameplay facts. `economy-api` uses these JSON files to
  validate and materialize durable rewards.
- Reward pools are shared across activity types so rare material, small token
  and equipment drops use one shape.
- Equipment rewards generate unique backend-owned equipment instances with
  affixes. The client should treat `equipmentUid` as the stable instance ID.
- Generated equipment sell prices are based on the template stage and instance
  rarity via `sellPriceGoldByStage`; legacy item-row weapon prices are not used.
- Dungeon rewards enter the loot tray and require a later claim.
- Gathering and farming rewards are collection-style rewards: items and
  equipment go directly into the bag, while token rewards still become locked
  AEB records.
- Gathering nodes enforce `respawnSeconds` as the minimum settlement interval
  for the same character and the same `nodeId`.
- Fishing is modeled as a gathering node with `nodeType=fishing`. Its equipment
  drops can use `equipmentStageMode=character_level_floor`, which picks the
  nearest configured equipment stage at or below the character level before
  rolling a template from `equipmentSeries`.
- Boss rewards enter the loot tray and use participation plus contribution
  tiers. Token rewards still become locked AEB records.
- NFT Mint uses AEB and is versioned by rarity in `economy_rules.json`: rarity
  3 = 500, rarity 4 = 2,000, rarity 5+ = 10,000 AEB by default.
