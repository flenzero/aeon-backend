# Aeonblight 游戏内容图鉴

> 本文面向策划、客户端和社区读者。内容依据 `configs/economy/` 当前配置整理，覆盖 37 个静态目录物品、70 个装备模板和 5 个坐骑模板。
>
> “未配置自动产出”只表示当前经济 JSON 没有把它放进副本、活动、抽奖或合成产出；它仍可能通过 GM、运营活动或玩家交易流通。

## 命名、品质与获得方式

### 装备命名

装备名称由 **品质 + 阶段前缀 + 装备类型** 组成，例如 `卓越·夜璃法杖`。

- 阶段（T1/T5/...）代表装备成长层级，不代表品质。
- 品质由装备实例的 `rarity` 决定；同一模板可以生成不同品质。
- 装备模板 ID 保持英文技术 ID，例如 `nightglass_staff_t15`。

装备表中的“中文名／英文名”是**模板基础名**；下表的品质前缀会实际组合到每一个装备实例中。因此，每一个装备模板都有六个完整显示名，而非只拥有一个名称。

| rarity | 英文品质 | 中文品质 | 词缀数 |
| ---: | --- | --- | ---: |
| 1 | Damaged | 破损 | 1 |
| 2 | Crude | 粗制 | 2 |
| 3 | Fine | 精良 | 3 |
| 4 | Superior | 卓越 | 4 |
| 5 | Flawless | 完美 | 5 |
| 6 | Exalted | 至臻 | 6 |

### 完整装备显示名

完整英文名格式为：`<Quality English> <Stage English> <Type English>`。

完整中文名格式为：`<中文品质>·<阶段中文前缀><装备中文类型>`。

| 品质 | 完整英文名示例 | 完整中文名示例 |
| --- | --- | --- |
| 破损 / Damaged | Damaged Ashbound Sword | 破损·缚灰长剑 |
| 粗制 / Crude | Crude Gloomhide Axe | 粗制·幽皮战斧 |
| 精良 / Fine | Fine Shadowiron Bow | 精良·影铁长弓 |
| 卓越 / Superior | Superior Nightglass Staff | 卓越·夜璃法杖 |
| 完美 / Flawless | Flawless Eclipseguard Helmet | 完美·蚀日卫头盔 |
| 至臻 / Exalted | Exalted Aeonblight Amulet | 至臻·永劫蚀光护符 |

例如，模板 `nightglass_staff_t15` 的六种完整命名为：

| rarity | 英文全名 | 中文全名 |
| ---: | --- | --- |
| 1 | Damaged Nightglass Staff | 破损·夜璃法杖 |
| 2 | Crude Nightglass Staff | 粗制·夜璃法杖 |
| 3 | Fine Nightglass Staff | 精良·夜璃法杖 |
| 4 | Superior Nightglass Staff | 卓越·夜璃法杖 |
| 5 | Flawless Nightglass Staff | 完美·夜璃法杖 |
| 6 | Exalted Nightglass Staff | 至臻·夜璃法杖 |

当前抽奖中的装备品质权重只会产出精良、卓越或完美（80% / 15% / 5%）；副本和 Boss 掉落可配置为其他品质。

### 材料命名

材料以品质前缀命名。现有材料最高只配置到 rarity 4：

| rarity | 英文材料前缀 | 中文材料前缀 |
| ---: | --- | --- |
| 1 | Rough | 粗粝 |
| 2 | Native | 原生 |
| 3 | Refined | 精炼 |
| 4 | Pure | 纯净 |

旧的内部 ID 仍包含 `white`、`green`、`blue`、`purple` 后缀，以兼容现有配置与存档；玩家可见名称不再显示颜色品质。

### 获得方式缩写

| 缩写 | 含义 |
| --- | --- |
| 抽奖 | AEB 抽奖；装备要求角色等级不低于该装备阶段 |
| 副本 CH0/CH1/CH2 | 灰烬门槛／幽林／虚痕章节的普通层或章节 Boss 掉落 |
| 世界 Boss | 暗影利维坦参与、主奖励或贡献档奖励 |
| 采集 | 灰木、影铁、幽蕈孢子、幽辉石四个独立掉落池 |
| 种植 | 烬根收获掉落池 |
| 悬赏 | 悬赏任务奖励或用悬赏徽章抽取 |
| 未配置 | 当前没有配置化自动获得路径 |

## 收集品与普通物品

### 材料与稀有材料

| 技术 ID | 英文名 | 中文名 | 当前产出 | 功能 |
| --- | --- | --- | --- | --- |
| `shadow_iron` | Rough Shadow Iron Ore | 粗粝影铁矿 | CH2 普通副本 | 合成锈蚀军刀的材料。 |
| `gloomcap_spore` | Native Gloomcap Spore Cluster | 原生幽蕈孢子团 | 世界 Boss 参与奖励 | 3 个可压缩为 1 个纯净永劫碎片。 |
| `aeon_shard` | Pure Aeon Shard | 纯净永劫碎片 | 种植、世界 Boss、部分副本、合成 | 稀有材料；合成锈蚀军刀的材料。 |
| `ashwood_white` | Rough Ashwood | 粗粝灰木 | 采集；CH1 普通副本 | 基础木材；可作为普通悬赏采集目标。 |
| `ashwood_green` | Native Ashwood | 原生灰木 | 采集 | 二阶灰木材料。 |
| `ashwood_blue` | Refined Ashwood | 精炼灰木 | 采集；抽奖 | 三阶稀有灰木；可作为普通悬赏采集目标。 |
| `ashwood_purple` | Pure Ashwood | 纯净灰木 | 采集；抽奖 | 四阶稀有灰木。 |
| `gloomstone_white` | Rough Gloomstone | 粗粝幽辉石 | 采集 | 基础矿石材料。 |
| `gloomstone_green` | Native Gloomstone | 原生幽辉石 | 采集 | 二阶幽辉石；可作为普通悬赏采集目标。 |
| `gloomstone_blue` | Refined Gloomstone | 精炼幽辉石 | 采集；抽奖 | 三阶稀有幽辉石。 |
| `gloomstone_purple` | Pure Gloomstone | 纯净幽辉石 | 采集；抽奖 | 四阶稀有幽辉石。 |
| `shadow_iron_white` | Rough Shadow Iron | 粗粝影铁 | 采集 | 基础影铁变体。 |
| `shadow_iron_green` | Native Shadow Iron | 原生影铁 | 采集 | 二阶影铁变体。 |
| `shadow_iron_blue` | Refined Shadow Iron | 精炼影铁 | 采集；抽奖 | 三阶稀有影铁；可作为普通悬赏采集目标。 |
| `shadow_iron_purple` | Pure Shadow Iron | 纯净影铁 | 采集；抽奖 | 四阶稀有影铁。 |
| `gloomcap_spore_white` | Rough Gloomcap Spore | 粗粝幽蕈孢子 | 采集 | 基础幽蕈孢子变体。 |
| `gloomcap_spore_green` | Native Gloomcap Spore | 原生幽蕈孢子 | 采集 | 二阶幽蕈孢子变体。 |
| `gloomcap_spore_blue` | Refined Gloomcap Spore | 精炼幽蕈孢子 | 采集；抽奖 | 三阶稀有幽蕈孢子。 |
| `gloomcap_spore_purple` | Pure Gloomcap Spore | 纯净幽蕈孢子 | 采集；抽奖 | 四阶稀有幽蕈孢子；可作为普通悬赏采集目标。 |

### 种植、通行与悬赏物品

| 技术 ID | 英文名 | 中文名 | 当前产出 | 功能 |
| --- | --- | --- | --- | --- |
| `emberroot_seed` | Emberroot Seed | 烬根种子 | 未配置 | 种植烬根所需种子。 |
| `emberroot` | Emberroot | 烬根 | 种植收获 | 作物产物。 |
| `boss_ticket_ashen_threshold` | Ashen Threshold Sigil | 烬灰门槛印记 | CH0 普通副本、悬赏抽取、抽奖 | 进入 CH0 Boss 的门票。 |
| `boss_ticket_gloomwood` | Gloomwood Hunt Seal | 幽林狩猎印 | CH1 普通副本、世界 Boss 主奖励、悬赏抽取、抽奖 | 进入 CH1 Boss 的门票。 |
| `boss_ticket_voidscar` | Voidscar Requiem Key | 虚痕安魂钥 | CH2 普通副本、悬赏抽取、抽奖 | 进入 CH2 Boss 的门票。 |
| `bounty_badge_common` | Common Bounty Badge | 普通委托徽章 | 普通悬赏任务 | 用于普通徽章抽取。 |
| `bounty_badge_rare` | Fine Bounty Badge | 精良委托徽章 | 稀有悬赏任务 | 用于稀有徽章抽取。 |
| `market_stall_permit` | Market Stall Permit | 集市摊位许可证 | 未配置 | 每次使用扩展 2 个市场挂单槽。 |
| `aeb_exchange_voucher` | AEB Exchange Voucher | AEB兑换券 | 四类采集点，单次 0.02% 概率 | AEB 兑换凭证；当前仅配置掉落和库存，兑换 AEB 的业务接口尚未实现。 |

### 强化石

强化装备 +1 至 +5 只消耗金币；从 +6 起还需要对应装备阶段的强化石。当前强化石没有配置化自动产出。

| 技术 ID | 英文名 | 中文名 | 对应阶段 | 功能 |
| --- | --- | --- | ---: | --- |
| `enhancement_stone_t1` | Ashbound Enhancement Stone | 缚灰强化石 | T1 | T1 装备 +6 至 +10 强化材料。 |
| `enhancement_stone_t5` | Gloomhide Enhancement Stone | 幽皮强化石 | T5 | T5 装备 +6 至 +10 强化材料。 |
| `enhancement_stone_t10` | Shadowiron Enhancement Stone | 影铁强化石 | T10 | T10 装备 +6 至 +10 强化材料。 |
| `enhancement_stone_t15` | Nightglass Enhancement Stone | 夜璃强化石 | T15 | T15 装备 +6 至 +10 强化材料。 |
| `enhancement_stone_t20` | Eclipseguard Enhancement Stone | 蚀日卫强化石 | T20 | T20 装备 +6 至 +10 强化材料。 |
| `enhancement_stone_t25` | Voidforged Enhancement Stone | 虚铸强化石 | T25 | T25 装备 +6 至 +10 强化材料。 |
| `enhancement_stone_t30` | Aeonblight Enhancement Stone | 永劫蚀光强化石 | T30 | T30 装备 +6 至 +10 强化材料。 |

### 旧版装备与合成

| 技术 ID | 英文名 | 中文名 | 当前产出 | 功能 |
| --- | --- | --- | --- | --- |
| `rusted_saber` | Rusted Saber | 锈蚀军刀 | 合成：5 个粗粝影铁矿 + 2 个纯净永劫碎片 | 旧版单手武器；可产生词缀，但不使用当前装备模板的实时属性模型。 |
| `nightglass_staff` | Nightglass Staff | 夜璃法杖 | 未配置 | 旧版法杖；保留兼容，当前不使用实时装备模板。 |

## 装备

### 通用功能

- 装备是唯一实例：每件拥有 `equipmentUid`、品质、耐久、词缀和强化等级。
- 可以放入背包、仓库、市场，穿戴到指定槽位；耐久为零时无法穿戴，修理消耗 AEB。
- 每件装备有 1 至 6 条随机词缀，数量等于品质；强化随机提升其中一条词缀，最高 +10。
- 稀有度达到精良（3）或以上时可支付 AEB 发起 NFT Mint；当前链上确认实现为失败关闭。
- 所有下表装备均可从抽奖获得，前提是角色等级不低于其 T 阶段；“额外掉落”列补充当前副本和世界 Boss 的配置化来源。

### 装备类型、槽位与基础作用

| 系列 | 英文类型 | 中文类型 | 槽位 | 武器类型 | 基础作用 |
| --- | --- | --- | ---: | ---: | --- |
| `sword` | Sword | 长剑 | 0 | 1 | 攻击 +8、防御 +1、最大生命 +4。 |
| `axe` | Axe | 战斧 | 0 | 2 | 攻击 +12、最大生命 +3、攻击速度 -8%。 |
| `bow` | Bow | 长弓 | 0 | 3 | 攻击 +6；随阶段提高攻击速度。 |
| `staff` | Staff | 法杖 | 0 | 4 | 攻击 +7。 |
| `helmet` | Helmet | 头盔 | 1 | 0 | 防御 +1、最大生命 +4。 |
| `chest` | Chestplate | 胸甲 | 2 | 0 | 防御 +3、最大生命 +10。 |
| `cloak` | Cloak | 披风 | 3 | 0 | 防御 +1、最大生命 +5；随阶段提高闪避。 |
| `gloves` | Gloves | 手套 | 4 | 0 | 攻击 +2；随阶段提高攻击速度。 |
| `accessory` | Amulet | 护符 | 5 | 0 | 攻击 +1、最大生命 +5。 |
| `shoes` | Boots | 战靴 | 6 | 0 | 防御 +1、最大生命 +4；随阶段提高闪避。 |

装备槽位按角色界面两列从上到下编号：左一武器 `0`、右一头盔 `1`、左二胸甲 `2`、右二披风 `3`、左三手套 `4`、右三饰品 `5`、左四战靴 `6`、右四坐骑 `7`。武器类型为 `0 none`、`1 sword`、`2 axe`、`3 bow`、`4 staff`。

> 同一类型的基础数值会乘以阶段倍率和品质倍率；T10 以后各模板还可能追加暴击、暴伤、攻速或闪避等百分比属性。

### T1：Ashbound / 缚灰

额外掉落：CH0 普通副本掉落除法杖外的全部 T1 模板。

| 技术 ID | 英文名 | 中文名 |
| --- | --- | --- |
| `ashbound_sword_t1` | Ashbound Sword | 缚灰长剑 |
| `ashbound_axe_t1` | Ashbound Axe | 缚灰战斧 |
| `ashbound_bow_t1` | Ashbound Bow | 缚灰长弓 |
| `ashbound_staff_t1` | Ashbound Staff | 缚灰法杖 |
| `ashbound_helmet_t1` | Ashbound Helmet | 缚灰头盔 |
| `ashbound_chest_t1` | Ashbound Chestplate | 缚灰胸甲 |
| `ashbound_gloves_t1` | Ashbound Gloves | 缚灰手套 |
| `ashbound_shoes_t1` | Ashbound Boots | 缚灰战靴 |
| `ashbound_cloak_t1` | Ashbound Cloak | 缚灰披风 |
| `ashbound_accessory_t1` | Ashbound Amulet | 缚灰护符 |

### T5：Gloomhide / 幽皮

额外掉落：CH0 Boss 掉落长剑、长弓、头盔、胸甲、手套、护符。

| 技术 ID | 英文名 | 中文名 |
| --- | --- | --- |
| `gloomhide_sword_t5` | Gloomhide Sword | 幽皮长剑 |
| `gloomhide_axe_t5` | Gloomhide Axe | 幽皮战斧 |
| `gloomhide_bow_t5` | Gloomhide Bow | 幽皮长弓 |
| `gloomhide_staff_t5` | Gloomhide Staff | 幽皮法杖 |
| `gloomhide_helmet_t5` | Gloomhide Helmet | 幽皮头盔 |
| `gloomhide_chest_t5` | Gloomhide Chestplate | 幽皮胸甲 |
| `gloomhide_gloves_t5` | Gloomhide Gloves | 幽皮手套 |
| `gloomhide_shoes_t5` | Gloomhide Boots | 幽皮战靴 |
| `gloomhide_cloak_t5` | Gloomhide Cloak | 幽皮披风 |
| `gloomhide_accessory_t5` | Gloomhide Amulet | 幽皮护符 |

### T10：Shadowiron / 影铁

额外掉落：CH1 普通副本掉落全部 T10 模板。

| 技术 ID | 英文名 | 中文名 |
| --- | --- | --- |
| `shadowiron_sword_t10` | Shadowiron Sword | 影铁长剑 |
| `shadowiron_axe_t10` | Shadowiron Axe | 影铁战斧 |
| `shadowiron_bow_t10` | Shadowiron Bow | 影铁长弓 |
| `shadowiron_staff_t10` | Shadowiron Staff | 影铁法杖 |
| `shadowiron_helmet_t10` | Shadowiron Helmet | 影铁头盔 |
| `shadowiron_chest_t10` | Shadowiron Chestplate | 影铁胸甲 |
| `shadowiron_gloves_t10` | Shadowiron Gloves | 影铁手套 |
| `shadowiron_shoes_t10` | Shadowiron Boots | 影铁战靴 |
| `shadowiron_cloak_t10` | Shadowiron Cloak | 影铁披风 |
| `shadowiron_accessory_t10` | Shadowiron Amulet | 影铁护符 |

### T15：Nightglass / 夜璃

额外掉落：CH1 Boss 掉落长剑、长弓、法杖、胸甲、披风、护符；世界 Boss 银档额外掉落夜璃法杖。

| 技术 ID | 英文名 | 中文名 |
| --- | --- | --- |
| `nightglass_sword_t15` | Nightglass Sword | 夜璃长剑 |
| `nightglass_axe_t15` | Nightglass Axe | 夜璃战斧 |
| `nightglass_bow_t15` | Nightglass Bow | 夜璃长弓 |
| `nightglass_staff_t15` | Nightglass Staff | 夜璃法杖 |
| `nightglass_helmet_t15` | Nightglass Helmet | 夜璃头盔 |
| `nightglass_chest_t15` | Nightglass Chestplate | 夜璃胸甲 |
| `nightglass_gloves_t15` | Nightglass Gloves | 夜璃手套 |
| `nightglass_shoes_t15` | Nightglass Boots | 夜璃战靴 |
| `nightglass_cloak_t15` | Nightglass Cloak | 夜璃披风 |
| `nightglass_accessory_t15` | Nightglass Amulet | 夜璃护符 |

### T20：Eclipseguard / 蚀日卫

额外掉落：CH2 普通副本掉落除手套外的全部 T20 模板；世界 Boss 金档额外掉落蚀日卫法杖。

| 技术 ID | 英文名 | 中文名 |
| --- | --- | --- |
| `eclipseguard_sword_t20` | Eclipseguard Sword | 蚀日卫长剑 |
| `eclipseguard_axe_t20` | Eclipseguard Axe | 蚀日卫战斧 |
| `eclipseguard_bow_t20` | Eclipseguard Bow | 蚀日卫长弓 |
| `eclipseguard_staff_t20` | Eclipseguard Staff | 蚀日卫法杖 |
| `eclipseguard_helmet_t20` | Eclipseguard Helmet | 蚀日卫头盔 |
| `eclipseguard_chest_t20` | Eclipseguard Chestplate | 蚀日卫胸甲 |
| `eclipseguard_gloves_t20` | Eclipseguard Gloves | 蚀日卫手套 |
| `eclipseguard_shoes_t20` | Eclipseguard Boots | 蚀日卫战靴 |
| `eclipseguard_cloak_t20` | Eclipseguard Cloak | 蚀日卫披风 |
| `eclipseguard_accessory_t20` | Eclipseguard Amulet | 蚀日卫护符 |

### T25：Voidforged / 虚铸

额外掉落：CH2 Boss 掉落战斧、长弓、法杖、胸甲、护符；其余模板目前只在抽奖池中可得。

| 技术 ID | 英文名 | 中文名 |
| --- | --- | --- |
| `voidforged_sword_t25` | Voidforged Sword | 虚铸长剑 |
| `voidforged_axe_t25` | Voidforged Axe | 虚铸战斧 |
| `voidforged_bow_t25` | Voidforged Bow | 虚铸长弓 |
| `voidforged_staff_t25` | Voidforged Staff | 虚铸法杖 |
| `voidforged_helmet_t25` | Voidforged Helmet | 虚铸头盔 |
| `voidforged_chest_t25` | Voidforged Chestplate | 虚铸胸甲 |
| `voidforged_gloves_t25` | Voidforged Gloves | 虚铸手套 |
| `voidforged_shoes_t25` | Voidforged Boots | 虚铸战靴 |
| `voidforged_cloak_t25` | Voidforged Cloak | 虚铸披风 |
| `voidforged_accessory_t25` | Voidforged Amulet | 虚铸护符 |

### T30：Aeonblight / 永劫蚀光

额外掉落：CH2 Boss 掉落长剑、胸甲、护符；其余模板目前只在抽奖池中可得。

| 技术 ID | 英文名 | 中文名 |
| --- | --- | --- |
| `aeonblight_sword_t30` | Aeonblight Sword | 永劫蚀光长剑 |
| `aeonblight_axe_t30` | Aeonblight Axe | 永劫蚀光战斧 |
| `aeonblight_bow_t30` | Aeonblight Bow | 永劫蚀光长弓 |
| `aeonblight_staff_t30` | Aeonblight Staff | 永劫蚀光法杖 |
| `aeonblight_helmet_t30` | Aeonblight Helmet | 永劫蚀光头盔 |
| `aeonblight_chest_t30` | Aeonblight Chestplate | 永劫蚀光胸甲 |
| `aeonblight_gloves_t30` | Aeonblight Gloves | 永劫蚀光手套 |
| `aeonblight_shoes_t30` | Aeonblight Boots | 永劫蚀光战靴 |
| `aeonblight_cloak_t30` | Aeonblight Cloak | 永劫蚀光披风 |
| `aeonblight_accessory_t30` | Aeonblight Amulet | 永劫蚀光护符 |

## 装备实例全名索引（420 个）

下表将 70 个装备模板按六档品质完全展开。每个单元格均包含完整英文名和完整中文名；这些是同一模板在不同 `rarity` 下的可见名称，不会创建新的 `itemId`。

| 模板 ID | 破损 / Damaged | 粗制 / Crude | 精良 / Fine | 卓越 / Superior | 完美 / Flawless | 至臻 / Exalted |
| --- | --- | --- | --- | --- | --- | --- |
| `ashbound_sword_t1` | Damaged Ashbound Sword<br>破损·缚灰长剑 | Crude Ashbound Sword<br>粗制·缚灰长剑 | Fine Ashbound Sword<br>精良·缚灰长剑 | Superior Ashbound Sword<br>卓越·缚灰长剑 | Flawless Ashbound Sword<br>完美·缚灰长剑 | Exalted Ashbound Sword<br>至臻·缚灰长剑 |
| `ashbound_axe_t1` | Damaged Ashbound Axe<br>破损·缚灰战斧 | Crude Ashbound Axe<br>粗制·缚灰战斧 | Fine Ashbound Axe<br>精良·缚灰战斧 | Superior Ashbound Axe<br>卓越·缚灰战斧 | Flawless Ashbound Axe<br>完美·缚灰战斧 | Exalted Ashbound Axe<br>至臻·缚灰战斧 |
| `ashbound_bow_t1` | Damaged Ashbound Bow<br>破损·缚灰长弓 | Crude Ashbound Bow<br>粗制·缚灰长弓 | Fine Ashbound Bow<br>精良·缚灰长弓 | Superior Ashbound Bow<br>卓越·缚灰长弓 | Flawless Ashbound Bow<br>完美·缚灰长弓 | Exalted Ashbound Bow<br>至臻·缚灰长弓 |
| `ashbound_staff_t1` | Damaged Ashbound Staff<br>破损·缚灰法杖 | Crude Ashbound Staff<br>粗制·缚灰法杖 | Fine Ashbound Staff<br>精良·缚灰法杖 | Superior Ashbound Staff<br>卓越·缚灰法杖 | Flawless Ashbound Staff<br>完美·缚灰法杖 | Exalted Ashbound Staff<br>至臻·缚灰法杖 |
| `ashbound_helmet_t1` | Damaged Ashbound Helmet<br>破损·缚灰头盔 | Crude Ashbound Helmet<br>粗制·缚灰头盔 | Fine Ashbound Helmet<br>精良·缚灰头盔 | Superior Ashbound Helmet<br>卓越·缚灰头盔 | Flawless Ashbound Helmet<br>完美·缚灰头盔 | Exalted Ashbound Helmet<br>至臻·缚灰头盔 |
| `ashbound_chest_t1` | Damaged Ashbound Chestplate<br>破损·缚灰胸甲 | Crude Ashbound Chestplate<br>粗制·缚灰胸甲 | Fine Ashbound Chestplate<br>精良·缚灰胸甲 | Superior Ashbound Chestplate<br>卓越·缚灰胸甲 | Flawless Ashbound Chestplate<br>完美·缚灰胸甲 | Exalted Ashbound Chestplate<br>至臻·缚灰胸甲 |
| `ashbound_gloves_t1` | Damaged Ashbound Gloves<br>破损·缚灰手套 | Crude Ashbound Gloves<br>粗制·缚灰手套 | Fine Ashbound Gloves<br>精良·缚灰手套 | Superior Ashbound Gloves<br>卓越·缚灰手套 | Flawless Ashbound Gloves<br>完美·缚灰手套 | Exalted Ashbound Gloves<br>至臻·缚灰手套 |
| `ashbound_shoes_t1` | Damaged Ashbound Boots<br>破损·缚灰战靴 | Crude Ashbound Boots<br>粗制·缚灰战靴 | Fine Ashbound Boots<br>精良·缚灰战靴 | Superior Ashbound Boots<br>卓越·缚灰战靴 | Flawless Ashbound Boots<br>完美·缚灰战靴 | Exalted Ashbound Boots<br>至臻·缚灰战靴 |
| `ashbound_cloak_t1` | Damaged Ashbound Cloak<br>破损·缚灰披风 | Crude Ashbound Cloak<br>粗制·缚灰披风 | Fine Ashbound Cloak<br>精良·缚灰披风 | Superior Ashbound Cloak<br>卓越·缚灰披风 | Flawless Ashbound Cloak<br>完美·缚灰披风 | Exalted Ashbound Cloak<br>至臻·缚灰披风 |
| `ashbound_accessory_t1` | Damaged Ashbound Amulet<br>破损·缚灰护符 | Crude Ashbound Amulet<br>粗制·缚灰护符 | Fine Ashbound Amulet<br>精良·缚灰护符 | Superior Ashbound Amulet<br>卓越·缚灰护符 | Flawless Ashbound Amulet<br>完美·缚灰护符 | Exalted Ashbound Amulet<br>至臻·缚灰护符 |
| `gloomhide_sword_t5` | Damaged Gloomhide Sword<br>破损·幽皮长剑 | Crude Gloomhide Sword<br>粗制·幽皮长剑 | Fine Gloomhide Sword<br>精良·幽皮长剑 | Superior Gloomhide Sword<br>卓越·幽皮长剑 | Flawless Gloomhide Sword<br>完美·幽皮长剑 | Exalted Gloomhide Sword<br>至臻·幽皮长剑 |
| `gloomhide_axe_t5` | Damaged Gloomhide Axe<br>破损·幽皮战斧 | Crude Gloomhide Axe<br>粗制·幽皮战斧 | Fine Gloomhide Axe<br>精良·幽皮战斧 | Superior Gloomhide Axe<br>卓越·幽皮战斧 | Flawless Gloomhide Axe<br>完美·幽皮战斧 | Exalted Gloomhide Axe<br>至臻·幽皮战斧 |
| `gloomhide_bow_t5` | Damaged Gloomhide Bow<br>破损·幽皮长弓 | Crude Gloomhide Bow<br>粗制·幽皮长弓 | Fine Gloomhide Bow<br>精良·幽皮长弓 | Superior Gloomhide Bow<br>卓越·幽皮长弓 | Flawless Gloomhide Bow<br>完美·幽皮长弓 | Exalted Gloomhide Bow<br>至臻·幽皮长弓 |
| `gloomhide_staff_t5` | Damaged Gloomhide Staff<br>破损·幽皮法杖 | Crude Gloomhide Staff<br>粗制·幽皮法杖 | Fine Gloomhide Staff<br>精良·幽皮法杖 | Superior Gloomhide Staff<br>卓越·幽皮法杖 | Flawless Gloomhide Staff<br>完美·幽皮法杖 | Exalted Gloomhide Staff<br>至臻·幽皮法杖 |
| `gloomhide_helmet_t5` | Damaged Gloomhide Helmet<br>破损·幽皮头盔 | Crude Gloomhide Helmet<br>粗制·幽皮头盔 | Fine Gloomhide Helmet<br>精良·幽皮头盔 | Superior Gloomhide Helmet<br>卓越·幽皮头盔 | Flawless Gloomhide Helmet<br>完美·幽皮头盔 | Exalted Gloomhide Helmet<br>至臻·幽皮头盔 |
| `gloomhide_chest_t5` | Damaged Gloomhide Chestplate<br>破损·幽皮胸甲 | Crude Gloomhide Chestplate<br>粗制·幽皮胸甲 | Fine Gloomhide Chestplate<br>精良·幽皮胸甲 | Superior Gloomhide Chestplate<br>卓越·幽皮胸甲 | Flawless Gloomhide Chestplate<br>完美·幽皮胸甲 | Exalted Gloomhide Chestplate<br>至臻·幽皮胸甲 |
| `gloomhide_gloves_t5` | Damaged Gloomhide Gloves<br>破损·幽皮手套 | Crude Gloomhide Gloves<br>粗制·幽皮手套 | Fine Gloomhide Gloves<br>精良·幽皮手套 | Superior Gloomhide Gloves<br>卓越·幽皮手套 | Flawless Gloomhide Gloves<br>完美·幽皮手套 | Exalted Gloomhide Gloves<br>至臻·幽皮手套 |
| `gloomhide_shoes_t5` | Damaged Gloomhide Boots<br>破损·幽皮战靴 | Crude Gloomhide Boots<br>粗制·幽皮战靴 | Fine Gloomhide Boots<br>精良·幽皮战靴 | Superior Gloomhide Boots<br>卓越·幽皮战靴 | Flawless Gloomhide Boots<br>完美·幽皮战靴 | Exalted Gloomhide Boots<br>至臻·幽皮战靴 |
| `gloomhide_cloak_t5` | Damaged Gloomhide Cloak<br>破损·幽皮披风 | Crude Gloomhide Cloak<br>粗制·幽皮披风 | Fine Gloomhide Cloak<br>精良·幽皮披风 | Superior Gloomhide Cloak<br>卓越·幽皮披风 | Flawless Gloomhide Cloak<br>完美·幽皮披风 | Exalted Gloomhide Cloak<br>至臻·幽皮披风 |
| `gloomhide_accessory_t5` | Damaged Gloomhide Amulet<br>破损·幽皮护符 | Crude Gloomhide Amulet<br>粗制·幽皮护符 | Fine Gloomhide Amulet<br>精良·幽皮护符 | Superior Gloomhide Amulet<br>卓越·幽皮护符 | Flawless Gloomhide Amulet<br>完美·幽皮护符 | Exalted Gloomhide Amulet<br>至臻·幽皮护符 |
| `shadowiron_sword_t10` | Damaged Shadowiron Sword<br>破损·影铁长剑 | Crude Shadowiron Sword<br>粗制·影铁长剑 | Fine Shadowiron Sword<br>精良·影铁长剑 | Superior Shadowiron Sword<br>卓越·影铁长剑 | Flawless Shadowiron Sword<br>完美·影铁长剑 | Exalted Shadowiron Sword<br>至臻·影铁长剑 |
| `shadowiron_axe_t10` | Damaged Shadowiron Axe<br>破损·影铁战斧 | Crude Shadowiron Axe<br>粗制·影铁战斧 | Fine Shadowiron Axe<br>精良·影铁战斧 | Superior Shadowiron Axe<br>卓越·影铁战斧 | Flawless Shadowiron Axe<br>完美·影铁战斧 | Exalted Shadowiron Axe<br>至臻·影铁战斧 |
| `shadowiron_bow_t10` | Damaged Shadowiron Bow<br>破损·影铁长弓 | Crude Shadowiron Bow<br>粗制·影铁长弓 | Fine Shadowiron Bow<br>精良·影铁长弓 | Superior Shadowiron Bow<br>卓越·影铁长弓 | Flawless Shadowiron Bow<br>完美·影铁长弓 | Exalted Shadowiron Bow<br>至臻·影铁长弓 |
| `shadowiron_staff_t10` | Damaged Shadowiron Staff<br>破损·影铁法杖 | Crude Shadowiron Staff<br>粗制·影铁法杖 | Fine Shadowiron Staff<br>精良·影铁法杖 | Superior Shadowiron Staff<br>卓越·影铁法杖 | Flawless Shadowiron Staff<br>完美·影铁法杖 | Exalted Shadowiron Staff<br>至臻·影铁法杖 |
| `shadowiron_helmet_t10` | Damaged Shadowiron Helmet<br>破损·影铁头盔 | Crude Shadowiron Helmet<br>粗制·影铁头盔 | Fine Shadowiron Helmet<br>精良·影铁头盔 | Superior Shadowiron Helmet<br>卓越·影铁头盔 | Flawless Shadowiron Helmet<br>完美·影铁头盔 | Exalted Shadowiron Helmet<br>至臻·影铁头盔 |
| `shadowiron_chest_t10` | Damaged Shadowiron Chestplate<br>破损·影铁胸甲 | Crude Shadowiron Chestplate<br>粗制·影铁胸甲 | Fine Shadowiron Chestplate<br>精良·影铁胸甲 | Superior Shadowiron Chestplate<br>卓越·影铁胸甲 | Flawless Shadowiron Chestplate<br>完美·影铁胸甲 | Exalted Shadowiron Chestplate<br>至臻·影铁胸甲 |
| `shadowiron_gloves_t10` | Damaged Shadowiron Gloves<br>破损·影铁手套 | Crude Shadowiron Gloves<br>粗制·影铁手套 | Fine Shadowiron Gloves<br>精良·影铁手套 | Superior Shadowiron Gloves<br>卓越·影铁手套 | Flawless Shadowiron Gloves<br>完美·影铁手套 | Exalted Shadowiron Gloves<br>至臻·影铁手套 |
| `shadowiron_shoes_t10` | Damaged Shadowiron Boots<br>破损·影铁战靴 | Crude Shadowiron Boots<br>粗制·影铁战靴 | Fine Shadowiron Boots<br>精良·影铁战靴 | Superior Shadowiron Boots<br>卓越·影铁战靴 | Flawless Shadowiron Boots<br>完美·影铁战靴 | Exalted Shadowiron Boots<br>至臻·影铁战靴 |
| `shadowiron_cloak_t10` | Damaged Shadowiron Cloak<br>破损·影铁披风 | Crude Shadowiron Cloak<br>粗制·影铁披风 | Fine Shadowiron Cloak<br>精良·影铁披风 | Superior Shadowiron Cloak<br>卓越·影铁披风 | Flawless Shadowiron Cloak<br>完美·影铁披风 | Exalted Shadowiron Cloak<br>至臻·影铁披风 |
| `shadowiron_accessory_t10` | Damaged Shadowiron Amulet<br>破损·影铁护符 | Crude Shadowiron Amulet<br>粗制·影铁护符 | Fine Shadowiron Amulet<br>精良·影铁护符 | Superior Shadowiron Amulet<br>卓越·影铁护符 | Flawless Shadowiron Amulet<br>完美·影铁护符 | Exalted Shadowiron Amulet<br>至臻·影铁护符 |
| `nightglass_sword_t15` | Damaged Nightglass Sword<br>破损·夜璃长剑 | Crude Nightglass Sword<br>粗制·夜璃长剑 | Fine Nightglass Sword<br>精良·夜璃长剑 | Superior Nightglass Sword<br>卓越·夜璃长剑 | Flawless Nightglass Sword<br>完美·夜璃长剑 | Exalted Nightglass Sword<br>至臻·夜璃长剑 |
| `nightglass_axe_t15` | Damaged Nightglass Axe<br>破损·夜璃战斧 | Crude Nightglass Axe<br>粗制·夜璃战斧 | Fine Nightglass Axe<br>精良·夜璃战斧 | Superior Nightglass Axe<br>卓越·夜璃战斧 | Flawless Nightglass Axe<br>完美·夜璃战斧 | Exalted Nightglass Axe<br>至臻·夜璃战斧 |
| `nightglass_bow_t15` | Damaged Nightglass Bow<br>破损·夜璃长弓 | Crude Nightglass Bow<br>粗制·夜璃长弓 | Fine Nightglass Bow<br>精良·夜璃长弓 | Superior Nightglass Bow<br>卓越·夜璃长弓 | Flawless Nightglass Bow<br>完美·夜璃长弓 | Exalted Nightglass Bow<br>至臻·夜璃长弓 |
| `nightglass_staff_t15` | Damaged Nightglass Staff<br>破损·夜璃法杖 | Crude Nightglass Staff<br>粗制·夜璃法杖 | Fine Nightglass Staff<br>精良·夜璃法杖 | Superior Nightglass Staff<br>卓越·夜璃法杖 | Flawless Nightglass Staff<br>完美·夜璃法杖 | Exalted Nightglass Staff<br>至臻·夜璃法杖 |
| `nightglass_helmet_t15` | Damaged Nightglass Helmet<br>破损·夜璃头盔 | Crude Nightglass Helmet<br>粗制·夜璃头盔 | Fine Nightglass Helmet<br>精良·夜璃头盔 | Superior Nightglass Helmet<br>卓越·夜璃头盔 | Flawless Nightglass Helmet<br>完美·夜璃头盔 | Exalted Nightglass Helmet<br>至臻·夜璃头盔 |
| `nightglass_chest_t15` | Damaged Nightglass Chestplate<br>破损·夜璃胸甲 | Crude Nightglass Chestplate<br>粗制·夜璃胸甲 | Fine Nightglass Chestplate<br>精良·夜璃胸甲 | Superior Nightglass Chestplate<br>卓越·夜璃胸甲 | Flawless Nightglass Chestplate<br>完美·夜璃胸甲 | Exalted Nightglass Chestplate<br>至臻·夜璃胸甲 |
| `nightglass_gloves_t15` | Damaged Nightglass Gloves<br>破损·夜璃手套 | Crude Nightglass Gloves<br>粗制·夜璃手套 | Fine Nightglass Gloves<br>精良·夜璃手套 | Superior Nightglass Gloves<br>卓越·夜璃手套 | Flawless Nightglass Gloves<br>完美·夜璃手套 | Exalted Nightglass Gloves<br>至臻·夜璃手套 |
| `nightglass_shoes_t15` | Damaged Nightglass Boots<br>破损·夜璃战靴 | Crude Nightglass Boots<br>粗制·夜璃战靴 | Fine Nightglass Boots<br>精良·夜璃战靴 | Superior Nightglass Boots<br>卓越·夜璃战靴 | Flawless Nightglass Boots<br>完美·夜璃战靴 | Exalted Nightglass Boots<br>至臻·夜璃战靴 |
| `nightglass_cloak_t15` | Damaged Nightglass Cloak<br>破损·夜璃披风 | Crude Nightglass Cloak<br>粗制·夜璃披风 | Fine Nightglass Cloak<br>精良·夜璃披风 | Superior Nightglass Cloak<br>卓越·夜璃披风 | Flawless Nightglass Cloak<br>完美·夜璃披风 | Exalted Nightglass Cloak<br>至臻·夜璃披风 |
| `nightglass_accessory_t15` | Damaged Nightglass Amulet<br>破损·夜璃护符 | Crude Nightglass Amulet<br>粗制·夜璃护符 | Fine Nightglass Amulet<br>精良·夜璃护符 | Superior Nightglass Amulet<br>卓越·夜璃护符 | Flawless Nightglass Amulet<br>完美·夜璃护符 | Exalted Nightglass Amulet<br>至臻·夜璃护符 |
| `eclipseguard_sword_t20` | Damaged Eclipseguard Sword<br>破损·蚀日卫长剑 | Crude Eclipseguard Sword<br>粗制·蚀日卫长剑 | Fine Eclipseguard Sword<br>精良·蚀日卫长剑 | Superior Eclipseguard Sword<br>卓越·蚀日卫长剑 | Flawless Eclipseguard Sword<br>完美·蚀日卫长剑 | Exalted Eclipseguard Sword<br>至臻·蚀日卫长剑 |
| `eclipseguard_axe_t20` | Damaged Eclipseguard Axe<br>破损·蚀日卫战斧 | Crude Eclipseguard Axe<br>粗制·蚀日卫战斧 | Fine Eclipseguard Axe<br>精良·蚀日卫战斧 | Superior Eclipseguard Axe<br>卓越·蚀日卫战斧 | Flawless Eclipseguard Axe<br>完美·蚀日卫战斧 | Exalted Eclipseguard Axe<br>至臻·蚀日卫战斧 |
| `eclipseguard_bow_t20` | Damaged Eclipseguard Bow<br>破损·蚀日卫长弓 | Crude Eclipseguard Bow<br>粗制·蚀日卫长弓 | Fine Eclipseguard Bow<br>精良·蚀日卫长弓 | Superior Eclipseguard Bow<br>卓越·蚀日卫长弓 | Flawless Eclipseguard Bow<br>完美·蚀日卫长弓 | Exalted Eclipseguard Bow<br>至臻·蚀日卫长弓 |
| `eclipseguard_staff_t20` | Damaged Eclipseguard Staff<br>破损·蚀日卫法杖 | Crude Eclipseguard Staff<br>粗制·蚀日卫法杖 | Fine Eclipseguard Staff<br>精良·蚀日卫法杖 | Superior Eclipseguard Staff<br>卓越·蚀日卫法杖 | Flawless Eclipseguard Staff<br>完美·蚀日卫法杖 | Exalted Eclipseguard Staff<br>至臻·蚀日卫法杖 |
| `eclipseguard_helmet_t20` | Damaged Eclipseguard Helmet<br>破损·蚀日卫头盔 | Crude Eclipseguard Helmet<br>粗制·蚀日卫头盔 | Fine Eclipseguard Helmet<br>精良·蚀日卫头盔 | Superior Eclipseguard Helmet<br>卓越·蚀日卫头盔 | Flawless Eclipseguard Helmet<br>完美·蚀日卫头盔 | Exalted Eclipseguard Helmet<br>至臻·蚀日卫头盔 |
| `eclipseguard_chest_t20` | Damaged Eclipseguard Chestplate<br>破损·蚀日卫胸甲 | Crude Eclipseguard Chestplate<br>粗制·蚀日卫胸甲 | Fine Eclipseguard Chestplate<br>精良·蚀日卫胸甲 | Superior Eclipseguard Chestplate<br>卓越·蚀日卫胸甲 | Flawless Eclipseguard Chestplate<br>完美·蚀日卫胸甲 | Exalted Eclipseguard Chestplate<br>至臻·蚀日卫胸甲 |
| `eclipseguard_gloves_t20` | Damaged Eclipseguard Gloves<br>破损·蚀日卫手套 | Crude Eclipseguard Gloves<br>粗制·蚀日卫手套 | Fine Eclipseguard Gloves<br>精良·蚀日卫手套 | Superior Eclipseguard Gloves<br>卓越·蚀日卫手套 | Flawless Eclipseguard Gloves<br>完美·蚀日卫手套 | Exalted Eclipseguard Gloves<br>至臻·蚀日卫手套 |
| `eclipseguard_shoes_t20` | Damaged Eclipseguard Boots<br>破损·蚀日卫战靴 | Crude Eclipseguard Boots<br>粗制·蚀日卫战靴 | Fine Eclipseguard Boots<br>精良·蚀日卫战靴 | Superior Eclipseguard Boots<br>卓越·蚀日卫战靴 | Flawless Eclipseguard Boots<br>完美·蚀日卫战靴 | Exalted Eclipseguard Boots<br>至臻·蚀日卫战靴 |
| `eclipseguard_cloak_t20` | Damaged Eclipseguard Cloak<br>破损·蚀日卫披风 | Crude Eclipseguard Cloak<br>粗制·蚀日卫披风 | Fine Eclipseguard Cloak<br>精良·蚀日卫披风 | Superior Eclipseguard Cloak<br>卓越·蚀日卫披风 | Flawless Eclipseguard Cloak<br>完美·蚀日卫披风 | Exalted Eclipseguard Cloak<br>至臻·蚀日卫披风 |
| `eclipseguard_accessory_t20` | Damaged Eclipseguard Amulet<br>破损·蚀日卫护符 | Crude Eclipseguard Amulet<br>粗制·蚀日卫护符 | Fine Eclipseguard Amulet<br>精良·蚀日卫护符 | Superior Eclipseguard Amulet<br>卓越·蚀日卫护符 | Flawless Eclipseguard Amulet<br>完美·蚀日卫护符 | Exalted Eclipseguard Amulet<br>至臻·蚀日卫护符 |
| `voidforged_sword_t25` | Damaged Voidforged Sword<br>破损·虚铸长剑 | Crude Voidforged Sword<br>粗制·虚铸长剑 | Fine Voidforged Sword<br>精良·虚铸长剑 | Superior Voidforged Sword<br>卓越·虚铸长剑 | Flawless Voidforged Sword<br>完美·虚铸长剑 | Exalted Voidforged Sword<br>至臻·虚铸长剑 |
| `voidforged_axe_t25` | Damaged Voidforged Axe<br>破损·虚铸战斧 | Crude Voidforged Axe<br>粗制·虚铸战斧 | Fine Voidforged Axe<br>精良·虚铸战斧 | Superior Voidforged Axe<br>卓越·虚铸战斧 | Flawless Voidforged Axe<br>完美·虚铸战斧 | Exalted Voidforged Axe<br>至臻·虚铸战斧 |
| `voidforged_bow_t25` | Damaged Voidforged Bow<br>破损·虚铸长弓 | Crude Voidforged Bow<br>粗制·虚铸长弓 | Fine Voidforged Bow<br>精良·虚铸长弓 | Superior Voidforged Bow<br>卓越·虚铸长弓 | Flawless Voidforged Bow<br>完美·虚铸长弓 | Exalted Voidforged Bow<br>至臻·虚铸长弓 |
| `voidforged_staff_t25` | Damaged Voidforged Staff<br>破损·虚铸法杖 | Crude Voidforged Staff<br>粗制·虚铸法杖 | Fine Voidforged Staff<br>精良·虚铸法杖 | Superior Voidforged Staff<br>卓越·虚铸法杖 | Flawless Voidforged Staff<br>完美·虚铸法杖 | Exalted Voidforged Staff<br>至臻·虚铸法杖 |
| `voidforged_helmet_t25` | Damaged Voidforged Helmet<br>破损·虚铸头盔 | Crude Voidforged Helmet<br>粗制·虚铸头盔 | Fine Voidforged Helmet<br>精良·虚铸头盔 | Superior Voidforged Helmet<br>卓越·虚铸头盔 | Flawless Voidforged Helmet<br>完美·虚铸头盔 | Exalted Voidforged Helmet<br>至臻·虚铸头盔 |
| `voidforged_chest_t25` | Damaged Voidforged Chestplate<br>破损·虚铸胸甲 | Crude Voidforged Chestplate<br>粗制·虚铸胸甲 | Fine Voidforged Chestplate<br>精良·虚铸胸甲 | Superior Voidforged Chestplate<br>卓越·虚铸胸甲 | Flawless Voidforged Chestplate<br>完美·虚铸胸甲 | Exalted Voidforged Chestplate<br>至臻·虚铸胸甲 |
| `voidforged_gloves_t25` | Damaged Voidforged Gloves<br>破损·虚铸手套 | Crude Voidforged Gloves<br>粗制·虚铸手套 | Fine Voidforged Gloves<br>精良·虚铸手套 | Superior Voidforged Gloves<br>卓越·虚铸手套 | Flawless Voidforged Gloves<br>完美·虚铸手套 | Exalted Voidforged Gloves<br>至臻·虚铸手套 |
| `voidforged_shoes_t25` | Damaged Voidforged Boots<br>破损·虚铸战靴 | Crude Voidforged Boots<br>粗制·虚铸战靴 | Fine Voidforged Boots<br>精良·虚铸战靴 | Superior Voidforged Boots<br>卓越·虚铸战靴 | Flawless Voidforged Boots<br>完美·虚铸战靴 | Exalted Voidforged Boots<br>至臻·虚铸战靴 |
| `voidforged_cloak_t25` | Damaged Voidforged Cloak<br>破损·虚铸披风 | Crude Voidforged Cloak<br>粗制·虚铸披风 | Fine Voidforged Cloak<br>精良·虚铸披风 | Superior Voidforged Cloak<br>卓越·虚铸披风 | Flawless Voidforged Cloak<br>完美·虚铸披风 | Exalted Voidforged Cloak<br>至臻·虚铸披风 |
| `voidforged_accessory_t25` | Damaged Voidforged Amulet<br>破损·虚铸护符 | Crude Voidforged Amulet<br>粗制·虚铸护符 | Fine Voidforged Amulet<br>精良·虚铸护符 | Superior Voidforged Amulet<br>卓越·虚铸护符 | Flawless Voidforged Amulet<br>完美·虚铸护符 | Exalted Voidforged Amulet<br>至臻·虚铸护符 |
| `aeonblight_sword_t30` | Damaged Aeonblight Sword<br>破损·永劫蚀光长剑 | Crude Aeonblight Sword<br>粗制·永劫蚀光长剑 | Fine Aeonblight Sword<br>精良·永劫蚀光长剑 | Superior Aeonblight Sword<br>卓越·永劫蚀光长剑 | Flawless Aeonblight Sword<br>完美·永劫蚀光长剑 | Exalted Aeonblight Sword<br>至臻·永劫蚀光长剑 |
| `aeonblight_axe_t30` | Damaged Aeonblight Axe<br>破损·永劫蚀光战斧 | Crude Aeonblight Axe<br>粗制·永劫蚀光战斧 | Fine Aeonblight Axe<br>精良·永劫蚀光战斧 | Superior Aeonblight Axe<br>卓越·永劫蚀光战斧 | Flawless Aeonblight Axe<br>完美·永劫蚀光战斧 | Exalted Aeonblight Axe<br>至臻·永劫蚀光战斧 |
| `aeonblight_bow_t30` | Damaged Aeonblight Bow<br>破损·永劫蚀光长弓 | Crude Aeonblight Bow<br>粗制·永劫蚀光长弓 | Fine Aeonblight Bow<br>精良·永劫蚀光长弓 | Superior Aeonblight Bow<br>卓越·永劫蚀光长弓 | Flawless Aeonblight Bow<br>完美·永劫蚀光长弓 | Exalted Aeonblight Bow<br>至臻·永劫蚀光长弓 |
| `aeonblight_staff_t30` | Damaged Aeonblight Staff<br>破损·永劫蚀光法杖 | Crude Aeonblight Staff<br>粗制·永劫蚀光法杖 | Fine Aeonblight Staff<br>精良·永劫蚀光法杖 | Superior Aeonblight Staff<br>卓越·永劫蚀光法杖 | Flawless Aeonblight Staff<br>完美·永劫蚀光法杖 | Exalted Aeonblight Staff<br>至臻·永劫蚀光法杖 |
| `aeonblight_helmet_t30` | Damaged Aeonblight Helmet<br>破损·永劫蚀光头盔 | Crude Aeonblight Helmet<br>粗制·永劫蚀光头盔 | Fine Aeonblight Helmet<br>精良·永劫蚀光头盔 | Superior Aeonblight Helmet<br>卓越·永劫蚀光头盔 | Flawless Aeonblight Helmet<br>完美·永劫蚀光头盔 | Exalted Aeonblight Helmet<br>至臻·永劫蚀光头盔 |
| `aeonblight_chest_t30` | Damaged Aeonblight Chestplate<br>破损·永劫蚀光胸甲 | Crude Aeonblight Chestplate<br>粗制·永劫蚀光胸甲 | Fine Aeonblight Chestplate<br>精良·永劫蚀光胸甲 | Superior Aeonblight Chestplate<br>卓越·永劫蚀光胸甲 | Flawless Aeonblight Chestplate<br>完美·永劫蚀光胸甲 | Exalted Aeonblight Chestplate<br>至臻·永劫蚀光胸甲 |
| `aeonblight_gloves_t30` | Damaged Aeonblight Gloves<br>破损·永劫蚀光手套 | Crude Aeonblight Gloves<br>粗制·永劫蚀光手套 | Fine Aeonblight Gloves<br>精良·永劫蚀光手套 | Superior Aeonblight Gloves<br>卓越·永劫蚀光手套 | Flawless Aeonblight Gloves<br>完美·永劫蚀光手套 | Exalted Aeonblight Gloves<br>至臻·永劫蚀光手套 |
| `aeonblight_shoes_t30` | Damaged Aeonblight Boots<br>破损·永劫蚀光战靴 | Crude Aeonblight Boots<br>粗制·永劫蚀光战靴 | Fine Aeonblight Boots<br>精良·永劫蚀光战靴 | Superior Aeonblight Boots<br>卓越·永劫蚀光战靴 | Flawless Aeonblight Boots<br>完美·永劫蚀光战靴 | Exalted Aeonblight Boots<br>至臻·永劫蚀光战靴 |
| `aeonblight_cloak_t30` | Damaged Aeonblight Cloak<br>破损·永劫蚀光披风 | Crude Aeonblight Cloak<br>粗制·永劫蚀光披风 | Fine Aeonblight Cloak<br>精良·永劫蚀光披风 | Superior Aeonblight Cloak<br>卓越·永劫蚀光披风 | Flawless Aeonblight Cloak<br>完美·永劫蚀光披风 | Exalted Aeonblight Cloak<br>至臻·永劫蚀光披风 |
| `aeonblight_accessory_t30` | Damaged Aeonblight Amulet<br>破损·永劫蚀光护符 | Crude Aeonblight Amulet<br>粗制·永劫蚀光护符 | Fine Aeonblight Amulet<br>精良·永劫蚀光护符 | Superior Aeonblight Amulet<br>卓越·永劫蚀光护符 | Flawless Aeonblight Amulet<br>完美·永劫蚀光护符 | Exalted Aeonblight Amulet<br>至臻·永劫蚀光护符 |

## 坐骑

坐骑是 `mount` 分类的唯一装备实例，统一占用槽位 7。当前都只由抽奖获得，固定为完美品质；每只提供最终最大生命 +5%、最终攻击 +5%、移动速度 +25%。

| 技术 ID | 英文名 | 中文名 | 当前产出 | 功能 |
| --- | --- | --- | --- | --- |
| `mount_carrion_scarab` | Carrion Scarab | 腐甲尸甲虫 | 抽奖 | 坐骑；提供固定最终属性加成。 |
| `mount_grimfang_wolf` | Grimfang Wolf | 冷牙灰狼 | 抽奖 | 坐骑；提供固定最终属性加成。 |
| `mount_mire_tusker` | Mire Tusker | 泥沼獠猪 | 抽奖 | 坐骑；提供固定最终属性加成。 |
| `mount_bramble_mane` | Bramble Mane | 棘鬃战狼 | 抽奖 | 坐骑；提供固定最终属性加成。 |
| `mount_dreadhorn_ox` | Dreadhorn Ox | 恐角蛮牛 | 抽奖 | 坐骑；提供固定最终属性加成。 |

## 配方与活动产出总览

### 合成

| 配方 | 英文名 | 中文说明 | 消耗 | 产出 |
| --- | --- | --- | --- | --- |
| `compress_aeon_shard` | Compress Aeon Shard | 压缩永劫碎片 | 原生幽蕈孢子团 ×3 | 纯净永劫碎片 ×1 |
| `forge_rusted_saber` | Forge Rusted Saber | 锻造锈蚀军刀 | 粗粝影铁矿 ×5、纯净永劫碎片 ×2 | 锈蚀军刀 ×1 |

### 当前活动内容

| 活动 ID | 英文名 | 中文名 | 行为 | 关联掉落 |
| --- | --- | --- | --- | --- |
| `shadow_woods_ashwood_grove` | Shadow Woods Ashwood Grove | 暗影林地灰木林 | 伐木；消耗 1 点体力，240 秒刷新 | 灰木四品质：粗粝必得 1–3 个，原生 25%（1–2 个），精炼 5%，纯净 0.5%；AEB兑换券 0.02%。 |
| `shadow_woods_iron_vein` | Shadow Woods Iron Vein | 暗影林地影铁矿脉 | 采矿；消耗 1 点体力，300 秒刷新 | 影铁四品质：粗粝必得 1–3 个，原生 25%（1–2 个），精炼 5%，纯净 0.5%；AEB兑换券 0.02%。 |
| `shadow_woods_gloomcap_patch` | Shadow Woods Gloomcap Patch | 暗影林地幽蕈丛 | 采集；消耗 1 点体力，180 秒刷新 | 幽蕈孢子四品质：粗粝必得 1–3 个，原生 25%（1–2 个），精炼 5%，纯净 0.5%；AEB兑换券 0.02%。 |
| `shadow_woods_gloomstone_outcrop` | Shadow Woods Gloomstone Outcrop | 暗影林地幽辉石露头 | 采矿；消耗 1 点体力，420 秒刷新 | 幽辉石四品质：粗粝必得 1–3 个，原生 25%（1–2 个），精炼 5%，纯净 0.5%；AEB兑换券 0.02%。 |
| `emberroot` | Emberroot | 烬根 | 种植；成长 3600 秒 | 烬根 ×2–5；低概率纯净永劫碎片。 |

## 维护规则

- 新增静态物品时，在 `items.json` 写入英文技术 ID、中文 `displayName`、分类、稀有度和堆叠规则。
- 新增装备时，在 `equipment_templates.json` 新增模板；不要把装备品质写进模板 ID，品质应由实例 rarity 表达。
- 新增采集或种植内容时，需要同时更新 `gathering.json` 或 `farming.json`、`loot_pools.json` 与 `items.json`。
- 若要把旧的颜色后缀材料 ID 改为品质前缀 ID，必须同时迁移数据库库存、仓库、市场挂单、配方、掉落池、悬赏和客户端缓存；不能只改 `items.json`。
