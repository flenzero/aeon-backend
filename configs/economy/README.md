# Aeonblight Economy Config

This directory is the default economy configuration source for `economy-api`.

Runtime path:

```text
ECONOMY_CONFIG_DIR=configs/economy
```

Files:

- `items.json`: canonical item and equipment catalog IDs.
- `economy_rules.json`: shared caps, bag/warehouse sizes, bag expand, trading license, equipment repair, NFT mint fees.
- `marketplace.json`: marketplace fees, slot expands, daily limits.
- `dungeons.json`: chapter/floor entry costs, exp caps and reward pools.
- `loot_pools.json`: reusable reward pools for dungeons, gathering and farming.
- `equipment_affixes.json`: random equipment affix pools.
- `gathering.json`: gather node reward configuration.
- `farming.json`: farming crop and harvest reward configuration.
- `bosses.json`: global boss reward pools and contribution tiers.
- `recipes.json`: inventory synthesis recipes for materials and equipment.

Design notes:

- Game servers submit gameplay facts. `economy-api` uses these JSON files to
  validate and materialize durable rewards.
- Reward pools are shared across activity types so rare material, small token
  and equipment drops use one shape.
- Equipment rewards generate unique backend-owned equipment instances with
  affixes. The client should treat `equipmentUid` as the stable instance ID.
- Dungeon rewards enter the loot tray and require a later claim.
- Gathering and farming rewards are collection-style rewards: items and
  equipment go directly into the bag, while token rewards still become locked
  GAME records.
- Boss rewards enter the loot tray and use participation plus contribution
  tiers. Token rewards still become locked GAME records.
