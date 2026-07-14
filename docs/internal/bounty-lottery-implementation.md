# 抽奖与悬赏板开发契约

版本：2026-07-13  
上下文：`economy-api`、`economy-worker`、`admin-api`

本文是抽奖和悬赏任务板的实现基线。配置文件不是功能上线的
证明；只有满足本文的持久化、支付履约、路由和测试条件，功能才可
标记为完成。

## 1. 当前真实状态

| 模块 | 已有 | 尚未完成 |
| --- | --- | --- |
| 装备强化 | 配置、存储事务、`/equipment/enhance` | 强化石自然掉落路径 |
| NPC 回收 | 回收、NFT 拒绝、7 天清理 Worker 路由 | 超级管理员窗口内恢复 |
| AEB 抽奖 | `lottery.json`、`LOTTERY_DRAW` Payment Order、开奖快照、确认后 Loot Tray 履约、`POST /api/economy/lottery/draw` | 订单/抽奖结果查询页、PostgreSQL 集成测试 |
| 悬赏板 | `bounties.json`、任务生成器、数据库迁移 | 任务板存储操作、全部玩家接口、Payment Order 履约、徽章抽奖、测试 |

不得将悬赏板配置或迁移文件称为“任务系统已上线”。

## 2. 抽奖契约

### 2.1 调用与支付

- 路由：`POST /api/economy/lottery/draw`，能力为 `economy.gameplay`。
- 请求：`opId`、`characterId`、`count`；`count` 范围为 1–10。
- 价格：30 AEB / 抽；一次订单金额为 `30 * count`。
- 创建时写 `LOTTERY_DRAW` Payment Order，付款接收地址使用当前
  `SOLANA_DEPOSIT_WALLET`。
- 付款通过既有提交/确认链路进入 `FULFILLED` 后，服务端将快照奖励写
  入 Loot Tray；客户端仍使用既有 Loot Tray 领取接口。

### 2.2 奖励快照

- 类别权重：装备 85%，稀有材料 13.5%，Boss 门票 1%，坐骑 0.5%。
- 装备：只允许蓝/紫/金，权重 80/15/5；阶段不高于角色当前等级。
- 稀有材料：只从蓝色/紫色采集物候选池选择。
- Boss 门票：从 `lottery.json` 的普通候选列表选择。
- 坐骑：五只固定橙色坐骑。
- 下单时必须保存：价格、次数、类别权重、装备品质权重、候选集与完整
  `DungeonRewardPlan`。确认付款时禁止重新随机。

### 2.3 必须补充的测试

1. 同一 `opId` 重放只能返回同一订单和同一计划。
2. 配置修改后，已创建订单的计划保持不变。
3. 同一链上交易不能履约两个订单。
4. 付款确认重复调用不重复写 Loot Tray。
5. 角色等级 1 时不会生成高于 1 阶的装备。

## 3. 悬赏任务板契约

### 3.1 槽位与解锁

| 槽位 | 所有权 | 初始状态 / 解锁成本 |
| ---: | --- | --- |
| 1 | Character | 默认可用 |
| 2 | Character | 3,000 Gold |
| 3 | Account，共享至所有角色 | 300 AEB Payment Order |
| 4 | Account，共享至所有角色 | 600 AEB Payment Order |
| 5 | Account，共享至所有角色 | 900 AEB Payment Order |

- 槽位 3–5 不要求顺序购买。
- AEB 槽位付款确认后写入 `bounty_account_slot_unlocks`；不能只依赖前端
  或 Payment Order payload。
- 任务归属始终是 Character；账号共享槽位仅表示每个角色都能使用该槽位。

### 3.2 任务与刷新

- 每个已解锁槽位只保留一个 `ACTIVE` 或 `COMPLETED` 任务。
- 免费刷新：4 小时一次；只替换 `ACTIVE` 的普通任务。
- Gold 刷新：300 Gold；同样只替换可替换的普通任务。
- 高级刷新：30 AEB Payment Order；在可替换任务中**恰好一条**替换为稀有
  `submit_equipment`，其他为普通任务。
- `COMPLETED`、`CLAIMED`、显式锁定任务不得被刷新覆盖。
- 正常任务：精确质量材料采集/提交，或经验证的副本击杀。
- 稀有任务：提交背包内蓝色及以上、非 NFT、未上架的装备；当前不校验
  特定副词条，提交后消耗装备。

### 3.3 进度、领取和反作弊

- 采集进度只由已验证的 `gathering/settle` 成功事务推进。
- 战斗进度由游戏服提交 `dungeonRunId` 和敌人击杀事实；后端以该层敌人
  配置验证上限，并对 `dungeonRunId` 去重。
- 完成时转为 `COMPLETED`，领取时原子写奖励并转为 `CLAIMED`。
- 普通任务奖励 1 `bounty_badge_common`；稀有任务奖励 1
  `bounty_badge_rare`；任务本身不直接发 Gold/AEB/门票。

### 3.4 徽章抽奖

| 徽章 | 奖励池 |
| --- | --- |
| Common | Gold 60%（500–2000）、低阶门票 30%、Locked AEB 10%（1–3） |
| Rare | Gold 45%（1000–5000）、中阶门票 25%、高阶门票 15%、Locked AEB 15%（3–10） |

- 扣除徽章、抽取、Gold/物品/Locked AEB 发放和账本必须在同一幂等事务内。
- Locked AEB 必须走既有锁仓记录，不能直接增加可提现余额。

## 4. 待实现 API

| 路由 | 能力 | 目的 |
| --- | --- | --- |
| `GET /api/economy/bounty/board` | `economy.gameplay` | 返回角色可用槽位、刷新冷却与任务；首次访问补齐空槽位 |
| `POST /api/economy/bounty/slots/unlock-gold` | `economy.gameplay` | 解锁槽位 2 |
| `POST /api/economy/bounty/slots/unlock-aeb` | `economy.gameplay` | 创建槽位 3–5 AEB Payment Order |
| `POST /api/economy/bounty/refresh` | `economy.gameplay` | `free` / `gold` / `premium` 刷新 |
| `POST /api/economy/bounty/progress/combat` | `economy.gameplay` | 由游戏服提交可验证副本击杀事实 |
| `POST /api/economy/bounty/submit-equipment` | `economy.gameplay` | 消耗合格装备并完成稀有任务 |
| `POST /api/economy/bounty/claim` | `economy.gameplay` | 领取任务徽章 |
| `POST /api/economy/bounty/badges/draw` | `economy.gameplay` | 消耗徽章并开奖 |

## 5. 数据库与迁移

`20260713_bounty_board_v1` 已定义：

- `bounty_account_slot_unlocks`
- `bounty_character_slots`
- `bounty_tasks`
- `bounty_refreshes`

后续任何字段修改必须新增 append-only 更新脚本，并同时折叠进
`migrations/aeonblight_full_schema.sql`。应用启动不得自动执行迁移。

## 6. 完成定义

仅当以下全部成立，悬赏板才可标记“完成”：

1. 八个 API 已注册、鉴权、文档化并有错误码；
2. 所有写操作有 `opId` 并通过 Postgres 幂等重放测试；
3. 普通/Gold/高级刷新、五槽位及 Payment Order 履约测试通过；
4. 采集、战斗、装备提交三种进度均不能通过伪造请求完成；
5. 徽章抽奖的 Gold、物品、Locked AEB 与账本一致；
6. 数据库迁移在空库 bootstrap 与既有库 up 均可执行；
7. API 文档只列出实际已注册且测试通过的接口。
