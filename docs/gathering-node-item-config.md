# 采集节点物品配置表

适用接口：`POST /api/economy/gathering/settle`

请求体使用 `nodeId` 指定采集点：

```json
{"opId":"gather-01","characterId":10,"nodeId":"shadow_woods_iron_vein"}
```

服务端会把 `nodeId` 作为内部 `ActivityID` 使用；响应中的 `activityId` 等于本次请求的 `nodeId`。采集结算产出直接进入背包。`respawnSeconds` 是同一角色同一 `nodeId` 的最快结算间隔；未到间隔时，后端会拒绝本次结算。

数据源：

- `configs/economy/gathering.json`：采集点、体力消耗、刷新时间、掉落池 ID。
- `configs/economy/loot_pools.json`：掉落条目、数量范围、掉落概率、动态装备阶段模式。
- `configs/economy/items.json`：物品中文名、分类、稀有度、堆叠、绑定与售卖价格配置。

## 采集点总表

| nodeId | nodeType | 行为 | staminaCost | respawnSeconds | lootPoolId | 产出摘要 |
| --- | --- | --- | ---: | ---: | --- | --- |
| `shadow_woods_ashwood_grove` | `logging` | 伐木 | 1 | 6 | `gather_shadow_woods_ashwood` | 灰木四品质；AEB兑换券 |
| `shadow_woods_iron_vein` | `mining` | 采矿 | 1 | 8 | `gather_shadow_woods_iron` | 影铁四品质；AEB兑换券 |
| `shadow_woods_gloomcap_patch` | `foraging` | 采集 | 1 | 5 | `gather_shadow_woods_gloomcap` | 幽蕈孢子四品质；AEB兑换券 |
| `shadow_woods_gloomstone_outcrop` | `mining` | 采矿 | 1 | 10 | `gather_shadow_woods_gloomstone` | 幽辉石四品质；AEB兑换券 |
| `shadow_woods_blackwater_fishing_spot` | `fishing` | 钓鱼 | 1 | 5 | `gather_shadow_woods_blackwater_fishing` | 幽鳍鱼四类；低品质动态装备；AEB兑换券 |

## 详细产出表

`dropChance` 是每条掉落项的独立判定概率，不是互斥权重。也就是说，一次采集可以同时命中多条产出；普通采集的白色材料 `dropChance=1` 表示必得，钓鱼的基础鱼获不是必得。

普通物品的 `sellPrice` 来自 `configs/economy/items.json`，表示单个物品卖给商店时获得的金币。动态装备的卖出价来自 `configs/economy/equipment_templates.json` 的 `sellPriceGoldByStage`，按装备阶段和品质决定。

| nodeId | lootPoolId | itemId | 中文名 | category | rarity | maxStack | sellPrice | 绑定/交易 | quantity | dropChance | 概率 |
| --- | --- | --- | --- | --- | ---: | ---: | ---: | --- | --- | ---: | ---: |
| `shadow_woods_ashwood_grove` | `gather_shadow_woods_ashwood` | `ashwood_white` | 粗粝灰木 | `material` | 1 | 999 | 3 | `UNBOUND` / 可交易 | 1-3 | 1 | 100% |
| `shadow_woods_ashwood_grove` | `gather_shadow_woods_ashwood` | `ashwood_green` | 原生灰木 | `material` | 2 | 999 | 12 | `UNBOUND` / 可交易 | 1-2 | 0.25 | 25% |
| `shadow_woods_ashwood_grove` | `gather_shadow_woods_ashwood` | `ashwood_blue` | 精炼灰木 | `rare_material` | 3 | 999 | 90 | `UNBOUND` / 可交易 | 1 | 0.05 | 5% |
| `shadow_woods_ashwood_grove` | `gather_shadow_woods_ashwood` | `ashwood_purple` | 纯净灰木 | `rare_material` | 4 | 999 | 130 | `UNBOUND` / 可交易 | 1 | 0.005 | 0.5% |
| `shadow_woods_ashwood_grove` | `gather_shadow_woods_ashwood` | `aeb_exchange_voucher` | AEB兑换券 | `aeb_voucher` | 5 | 99 | 300 | `BOUND` / 不可交易 | 1 | 0.0002 | 0.02% |
| `shadow_woods_iron_vein` | `gather_shadow_woods_iron` | `shadow_iron_white` | 粗粝影铁 | `material` | 1 | 999 | 4 | `UNBOUND` / 可交易 | 1-3 | 1 | 100% |
| `shadow_woods_iron_vein` | `gather_shadow_woods_iron` | `shadow_iron_green` | 原生影铁 | `material` | 2 | 999 | 15 | `UNBOUND` / 可交易 | 1-2 | 0.25 | 25% |
| `shadow_woods_iron_vein` | `gather_shadow_woods_iron` | `shadow_iron_blue` | 精炼影铁 | `rare_material` | 3 | 999 | 100 | `UNBOUND` / 可交易 | 1 | 0.05 | 5% |
| `shadow_woods_iron_vein` | `gather_shadow_woods_iron` | `shadow_iron_purple` | 纯净影铁 | `rare_material` | 4 | 999 | 145 | `UNBOUND` / 可交易 | 1 | 0.005 | 0.5% |
| `shadow_woods_iron_vein` | `gather_shadow_woods_iron` | `aeb_exchange_voucher` | AEB兑换券 | `aeb_voucher` | 5 | 99 | 300 | `BOUND` / 不可交易 | 1 | 0.0002 | 0.02% |
| `shadow_woods_gloomcap_patch` | `gather_shadow_woods_gloomcap` | `gloomcap_spore_white` | 粗粝幽蕈孢子 | `material` | 1 | 999 | 5 | `UNBOUND` / 可交易 | 1-3 | 1 | 100% |
| `shadow_woods_gloomcap_patch` | `gather_shadow_woods_gloomcap` | `gloomcap_spore_green` | 原生幽蕈孢子 | `material` | 2 | 999 | 16 | `UNBOUND` / 可交易 | 1-2 | 0.25 | 25% |
| `shadow_woods_gloomcap_patch` | `gather_shadow_woods_gloomcap` | `gloomcap_spore_blue` | 精炼幽蕈孢子 | `rare_material` | 3 | 999 | 105 | `UNBOUND` / 可交易 | 1 | 0.05 | 5% |
| `shadow_woods_gloomcap_patch` | `gather_shadow_woods_gloomcap` | `gloomcap_spore_purple` | 纯净幽蕈孢子 | `rare_material` | 4 | 999 | 150 | `UNBOUND` / 可交易 | 1 | 0.005 | 0.5% |
| `shadow_woods_gloomcap_patch` | `gather_shadow_woods_gloomcap` | `aeb_exchange_voucher` | AEB兑换券 | `aeb_voucher` | 5 | 99 | 300 | `BOUND` / 不可交易 | 1 | 0.0002 | 0.02% |
| `shadow_woods_gloomstone_outcrop` | `gather_shadow_woods_gloomstone` | `gloomstone_white` | 粗粝幽辉石 | `material` | 1 | 999 | 4 | `UNBOUND` / 可交易 | 1-3 | 1 | 100% |
| `shadow_woods_gloomstone_outcrop` | `gather_shadow_woods_gloomstone` | `gloomstone_green` | 原生幽辉石 | `material` | 2 | 999 | 14 | `UNBOUND` / 可交易 | 1-2 | 0.25 | 25% |
| `shadow_woods_gloomstone_outcrop` | `gather_shadow_woods_gloomstone` | `gloomstone_blue` | 精炼幽辉石 | `rare_material` | 3 | 999 | 95 | `UNBOUND` / 可交易 | 1 | 0.05 | 5% |
| `shadow_woods_gloomstone_outcrop` | `gather_shadow_woods_gloomstone` | `gloomstone_purple` | 纯净幽辉石 | `rare_material` | 4 | 999 | 135 | `UNBOUND` / 可交易 | 1 | 0.005 | 0.5% |
| `shadow_woods_gloomstone_outcrop` | `gather_shadow_woods_gloomstone` | `aeb_exchange_voucher` | AEB兑换券 | `aeb_voucher` | 5 | 99 | 300 | `BOUND` / 不可交易 | 1 | 0.0002 | 0.02% |
| `shadow_woods_blackwater_fishing_spot` | `gather_shadow_woods_blackwater_fishing` | `gloomfin_sweet` | 清甜幽鳍鱼 | `fish` | 1 | 999 | 6 | `UNBOUND` / 可交易 | 1-2 | 0.7 | 70% |
| `shadow_woods_blackwater_fishing_spot` | `gather_shadow_woods_blackwater_fishing` | `gloomfin_fresh` | 鲜嫩幽鳍鱼 | `fish` | 2 | 999 | 18 | `UNBOUND` / 可交易 | 1 | 0.18 | 18% |
| `shadow_woods_blackwater_fishing_spot` | `gather_shadow_woods_blackwater_fishing` | `gloomfin_silver` | 银纹幽鳍鱼 | `fish` | 3 | 999 | 110 | `UNBOUND` / 可交易 | 1 | 0.04 | 4% |
| `shadow_woods_blackwater_fishing_spot` | `gather_shadow_woods_blackwater_fishing` | `gloomfin_moonspotted` | 月斑幽鳍鱼 | `fish` | 4 | 999 | 155 | `UNBOUND` / 可交易 | 1 | 0.004 | 0.4% |
| `shadow_woods_blackwater_fishing_spot` | `gather_shadow_woods_blackwater_fishing` | 动态装备：`character_level_floor` | 破损装备 | `weapon` / `armor` / `accessory` | 1 | 1 | 按阶段：80/140/240/410/700/1190/2020 | `UNBOUND` / 可交易 | 1 | 0.03 | 3% |
| `shadow_woods_blackwater_fishing_spot` | `gather_shadow_woods_blackwater_fishing` | 动态装备：`character_level_floor` | 粗制装备 | `weapon` / `armor` / `accessory` | 2 | 1 | 按阶段：100/180/310/530/910/1550/2630 | `UNBOUND` / 可交易 | 1 | 0.015 | 1.5% |
| `shadow_woods_blackwater_fishing_spot` | `gather_shadow_woods_blackwater_fishing` | `aeb_exchange_voucher` | AEB兑换券 | `aeb_voucher` | 5 | 99 | 300 | `BOUND` / 不可交易 | 1 | 0.0001 | 0.01% |

## 钓鱼动态装备阶段

钓鱼装备条目使用 `equipmentStageMode: "character_level_floor"`。命中装备掉落后，服务端按角色等级向下取最近的装备阶段，并从 `sword`、`axe`、`bow`、`staff`、`helmet`、`chest`、`gloves`、`shoes`、`cloak`、`accessory` 中随机一个模板。

| 角色等级 | 装备阶段 | 示例模板 |
| ---: | ---: | --- |
| 1-4 | T1 | `ashbound_sword_t1` |
| 5-9 | T5 | `gloomhide_sword_t5` |
| 10-14 | T10 | `shadowiron_sword_t10` |
| 15-19 | T15 | `nightglass_sword_t15` |
| 20-24 | T20 | `eclipseguard_sword_t20` |
| 25-29 | T25 | `voidforged_sword_t25` |
| 30+ | T30 | `aeonblight_sword_t30` |

## 动态装备卖出价

动态装备卖出价按装备阶段和品质读取 `sellPriceGoldByStage`。钓鱼当前只掉落破损和粗制装备；完整价格表如下。

| 装备阶段 | 破损 R1 | 粗制 R2 | 精良 R3 | 卓越 R4 | 完美 R5 | 至臻 R6 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| T1 | 80 | 100 | 130 | 170 | 230 | 300 |
| T5 | 140 | 180 | 230 | 300 | 410 | 540 |
| T10 | 240 | 310 | 400 | 520 | 700 | 920 |
| T15 | 410 | 530 | 680 | 890 | 1200 | 1580 |
| T20 | 700 | 910 | 1170 | 1530 | 2070 | 2720 |
| T25 | 1190 | 1550 | 1990 | 2610 | 3520 | 4630 |
| T30 | 2020 | 2630 | 3370 | 4420 | 5970 | 7850 |

## 维护备注

- 新增采集点时，先在 `configs/economy/gathering.json` 增加 `nodeId`，再在 `configs/economy/loot_pools.json` 增加或复用对应 `lootPoolId`。
- 掉落池里的普通物品 `itemId` 必须存在于 `configs/economy/items.json`；固定装备 `itemId` 必须存在于 `configs/economy/equipment_templates.json`。
- 若掉落池条目使用 `equipmentStageMode: "character_level_floor"`，可以不写固定 `itemId`；服务端会用角色等级动态选择装备模板。
- `aeb_exchange_voucher` 当前作为采集掉落物品进入背包；它在物品配置中带有 `grantLockedAeb=30`，但采集结算不会自动把该物品兑换成 Locked AEB。
- 数据库落库字段为 `gathering_settlements.node_key`，保存的值就是请求里的 `nodeId`。
