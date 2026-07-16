# Aeonblight 对外接口详解

版本日期：2026-07-15  
适用服务：`account-api`、`economy-api`  
接口数量：99 条非管理员路由  
不包含管理员接口；管理员接口见 [管理员接口文档](admin-interface-reference.md)。快速索引见 [接口列表](interface-list.md)。

## 1. 通用约定

### 1.1 Base URL

```text
account-api  http://127.0.0.1:8081
economy-api  http://127.0.0.1:8082
```

生产环境应由网关提供 HTTPS 域名；本文路径不包含网关前缀。

### 1.2 JSON 响应信封

成功：

```json
{"ok":true,"data":{}}
```

失败：

```json
{"ok":false,"error":{"code":400,"message":"invalid JSON body"}}
```

- 创建资源时通常返回 HTTP `201`，其余成功通常为 `200`。
- 业务错误码位于 `error.code`，不能只根据 HTTP 状态判断具体原因。
- 时间字段使用 RFC3339；AEB、Gold、Gems、数量和价格均使用整数。

### 1.3 请求边界与幂等

- 请求必须为单个 JSON 文档；未知字段、空 Body、尾随第二个 JSON 文档均拒绝。
- JSON Body 最大 `1 MiB`；无参数命令也必须发送精确空对象 `{}`。
- `opId` 是调用方生成的幂等键。相同 `opId` 只能用于同一操作、同一账号/角色和完全相同的参数。
- 除特别说明外，`characterId` 可放在 Body；Body 为 `0` 时从 `X-Character-Id` 读取。
- `accountId` 通常从 `X-Account-Id` 读取，不应信任玩家客户端自行填写。

### 1.4 JWT 认证

```http
Authorization: Bearer <accessToken>
```

Access Token 当前有效期为 2 小时。涉及恢复、Launch 和在线状态的写操作还会校验持久化 Session；Redis 不能覆盖 PostgreSQL 中的注销或撤销状态。

### 1.5 Service Identity 认证

生产/预发布机器调用必须携带：

```http
X-Service-Id: game-server-shanghai-01
X-Service-Timestamp: 1783912200
X-Service-Nonce: request-unique-nonce-0001
X-Service-Signature: <base58-ed25519-signature>
```

签名覆盖 `serviceId`、Unix 时间、nonce、HTTP Method、escaped path/query 和 Body SHA-256。默认时间窗口 ±120 秒；nonce 长度 16–128，只允许字母、数字、`.`、`_`、`:`、`-`，且只能使用一次。

- `GAME_SERVER` 身份绑定 `subjectId/serverId`，不能代表其他游戏服。
- `development/test` 可使用 `X-Internal-Key`；生产/预发布拒绝共享 Internal Key。

### 1.6 常用请求头

| 请求头 | 类型 | 用途 |
| --- | --- | --- |
| `Authorization` | string | JWT Bearer |
| `Content-Type` | string | JSON 请求使用 `application/json` |
| `X-Account-Id` | positive int64 | 游戏服确认过的账号 ID |
| `X-Character-Id` | positive int64 | 游戏服确认过的角色 ID |
| `X-Game-Server-Id` | string | 仅 legacy development/test 模式为副本提供服务器 ID |

## 2. 核心响应模型

### 2.1 EconomySnapshot

主要字段：`accountId`、`characterId`、`gold`、`gems`、`stamina`、`bagSlots`、`bagExpandCount`、`hasLicense`、`level`、`exp`、`accountToken`、`inventory`、`warehouse`、`lootTray`、`equipmentItems`。

`accountToken`：`tokenBalance`、`withdrawableBalance`、`lockedBalance`、`externalBalance`、`unlockCredit`。

### 2.2 InventoryItem / EquipmentItem

- InventoryItem：`id`、`itemId`、`quantity`、`slot`、`durability`、`enhanceLevel`。
- EquipmentItem：`id`、`equipmentUid`、`itemId`、`rarity`、`enhanceLevel`、`durability`、`maxDurability`、`status`、`equipSlot`、`slot`、`weaponType`、`weaponTypeKey`、`nftContract?`、`nftTokenId?`、`affixes?`。
- 当前装备模板的派生字段：`resolvedBaseFlat`、`resolvedBasePercent`、`resolvedFlatStats`、`resolvedPercentStats`、`finalBonuses?`。这些字段由账号服按当前经济配置实时计算，**不代表数据库持久化白值**。
- 新装备的 `affixes` 在数据库只保存 `affixId`、`instanceId`、`enhanceHits`；API 响应额外带 `stat`、`value` 以供客户端展示。`value` 可因配置发布而变化，客户端不得把它回写为装备属性。
- `equipSlot` 是装备槽位，不是背包/仓库格子；`slot` 才是背包或仓库位置。
- `weaponType` 只表示武器子类型；非武器和坐骑返回 `0/none`。当前枚举：`0 none`、`1 sword`、`2 axe`、`3 bow`、`4 staff`。
- 坐骑固定使用 `equipSlot=7`，并在 `finalBonuses` 中返回 `finalMaxHp=0.05`、`finalAttack=0.05`、`moveSpeed=0.25`。

`equipSlot` 当前按角色界面两列从上到下编号：

| 值 | key | 界面位置 | 说明 |
| ---: | --- | --- | --- |
| `-1` | `none` | 未穿戴 | 背包、仓库、掉落盘等非穿戴状态 |
| `0` | `weapon` | 左一 | 武器槽；具体武器子类型看 `weaponType` |
| `1` | `helmet` | 右一 | 头盔 |
| `2` | `chest` | 左二 | 胸甲 |
| `3` | `cloak` | 右二 | 披风 |
| `4` | `gloves` | 左三 | 手套 |
| `5` | `accessory` | 右三 | 饰品/护符 |
| `6` | `shoes` | 左四 | 战靴 |
| `7` | `mount` | 右四 | 坐骑 |

### 2.3 PaymentOrder

主要字段：`id`、`accountId`、`characterId?`、`purpose`、`payAsset`、`amount`、`receiverWallet?`、`status`、`txSignature?`、`createdAt`、`expiresAt`、`submittedAt?`、`confirmedAt?`、`fulfilledAt?`。

### 2.4 Announcement

主要字段：`id`、`kind`、`status`、`templateCode?`、`displayMode`、`title`、`body`、`priority`、`scope`、`startsAt`、`endsAt?`、`eventType?`、`source?`、`refType?`、`refId?`、`itemId?`、`itemName?`、`equipmentUid?`、`rarity?`、`createdAt`。

- `kind=OPS_NOTICE`：活动开启、关闭、维护、版本更新等运营通知。
- `kind=RARE_REWARD`：真实发奖事务生成的稀有装备/坐骑炫耀公告。
- `displayMode=POPUP` 建议客户端弹窗展示；`BANNER` 建议公告栏/跑马灯展示。
- 公开公告接口只返回 `OPS_NOTICE`，不会返回玩家稀有掉落。

### 2.5 分页

- Marketplace 公共列表默认 `limit=20`；我的列表默认 `50`；最大均为 `100`。
- `offset` 必须为非负整数；非法值不会静默回退。

### 2.6 装备配置与服务端权威

- 常规装备阶段为 `1/5/10/15/20/25/30`，品质为白、绿、蓝、紫、金、红（`rarity=1..6`）；品质决定副词条数量 `1..6`。
- 副词条池与出现权重均由服务端配置决定。相同 `affixId` 最多出现两次；初始值相同，但两个 `instanceId` 的强化命中独立。
- 白值使用模板与品质配置；百分比白值为模板固定属性。吸血与自动回血只会作为副词条返回。
- 副本入口始终以服务端 `dungeons.json` 的 `enterCost` 为准；Boss 层（10/20/30）分别要求 `boss_ticket_ashen_threshold` / `boss_ticket_gloomwood` / `boss_ticket_voidscar`。请求体中的成本不会被接受。
- 当前副本为三章全局楼层 `1..30`；`maxExp` 与 `lootPoolId` 由配置决定。`enemyHpScale` / `enemyAtkScale` 为战斗透传字段，economy 结算不使用。

## 3. account-api

account-api 负责登录、Session、启动器公开信息、角色选择/存档、Launch Ticket 和 Online Presence。玩家客户端只直接调用 JWT/公开接口；`account.gameplay` 接口应由可信游戏服代调。

### 3.1 健康检查

#### GET `/health`

- 认证：公开。
- 响应：`service`、`status=ok`。只表示进程存活。

#### GET `/ready`

- 认证：公开。
- 检查 PostgreSQL、`REQUIRED_SCHEMA_VERSION`、Redis；Required 检查失败返回 HTTP `503`。

### 3.2 钱包登录与 Session

#### GET `/api/auth/wallet/nonce`

- Query：`walletAddress`（必填，合法 Solana base58 公钥）。
- 成功：`nonce`、完整待签 `message`、`expiresAt`；有效期 5 分钟。
- 错误：HTTP `400` / code `1005`。

#### POST `/api/auth/wallet`

```json
{"walletAddress":"<solana-wallet>","walletPlugin":"phantom","nonce":"nonce_...","signature":"<base58-signature>","deviceId":"device-01"}
```

- 认证：公开；签名必须针对 nonce 接口返回的完整 `message`。
- 成功：`account`、`accessToken`、`refreshToken`、`sessionId`、`walletPlugin?`、`expiresAt`。
- Nonce 一次性消费；错误签名不会提前消费 nonce。错误 code `1007`。

#### POST `/api/auth/refresh`

Body：`{"refreshToken":"refresh_..."}`。

- 成功：`accessToken`、新 `refreshToken`、`sessionId`、`expiresIn`。
- 旧 Refresh Token 立即失效。错误：HTTP `401` / code `1008`。

#### POST `/api/auth/logout`

Body：`{"sessionId":"session_...","refreshToken":"refresh_..."}`。

- 认证：JWT；两个字段至少提供一个。
- 成功：`status=revoked`、`sessionId`。错误 code `1009`。

#### GET `/api/auth/verify`

- 认证：JWT。
- 成功：账号 `id`、`username`、`walletAddress`、`isBanned`、`createdAt`、`lastLoginAt`。
- 账号不存在：HTTP `403` / code `9001`。

#### GET `/api/auth/session/redis`

- 认证：`account.ops`。
- 成功：`enabled`、`ok?`、`mode`、`error?`；mode 为 `redis+postgres` 或 `postgres-only`。

### 3.3 公开选服

#### GET `/api/public/servers`

- 认证：公开；用于主页/启动器选服。
- 成功：`servers` 数组，只含 `serverId`、`name`、`curPlayers`、`maxPlayers`、`status`、`queueLength`、`region?`。
- 不返回 `host`、`port`、`publicEndpoint`；连接信息只在 `POST /api/game/launch` 成功时返回。
- `curPlayers` 来自 Redis Online Presence 的有效 TTL key 聚合；Redis 禁用的测试/开发模式回落到 PostgreSQL `online_sessions` 的有效 `last_seen_at`。
- `status` 为 `online`、`full`、`offline`；`ONLINE` 且最近 30 秒有心跳、Redis 在线数低于 95% 容量时为 `online`。

#### GET `/api/public/servers/online`

- 认证：公开。
- 成功：同 `/api/public/servers`，但只返回 `status=online` 的服务器。

#### GET `/api/public/home/stats`

- 认证：公开；用于主页统计条。
- 成功：`onlinePlayers`、`monthlyActivePlayers`、`updatedAt`。
- `onlinePlayers` 为全服 Redis Online Presence 有效 TTL key 汇总，和 `/api/public/servers[*].curPlayers` 的求和口径一致。
- `monthlyActivePlayers` 为最近 30 天 `account_sessions.last_seen_at >= now-30d` 的去重账号数。

#### GET `/api/public/home/config`

- 认证：公开；用于主页无需重新发包即可读取公开配置。
- 成功：`contractAddress`、`tokenSymbol`、`gameClientBaseUrl`、`supportWallets`、`updatedAt`。
- `contractAddress` 来自 `SOLANA_TOKEN_MINT`；`tokenSymbol` 来自 `TOKEN_SYMBOL`，默认 `AEB`；`supportWallets` 来自逗号分隔的 `SUPPORT_WALLETS`，默认 `phantom,solflare,backpack,okx`。

#### GET `/api/public/leaderboards/clear-progress`

- 认证：公开；`limit` 默认 10，最大 100。
- 成功：`intro`、`items`、`updatedAt`。
- `items` 按 `highestFloorId DESC, firstReachedAt ASC, characterId ASC` 排序；`firstReachedAt` 来自 `characters.highest_cleared_at`，表示第一次达到当前最高进度的时间。

#### GET `/api/public/leaderboards/weekly-score`

- 认证：公开；`limit` 默认 10，最大 100。
- 成功：`intro`、`period`、`items`、`updatedAt`。
- 7 天周期从 `2026-07-01T00:00:00Z` 开始；每个已完成副本按 `dungeon_key` 中的 `floor` 加分，允许重复刷同一层累计。

### 3.4 角色、外观与运行态存档

#### GET `/api/character/list`

- 认证：`account.gameplay`；`X-Account-Id` 必填。
- 成功：`characters` 数组；字段为 `id`、`accountId`、`name`、`slotIndex`、`level`、`appearance`、`equipmentItems`、`createdAt`、`lastPlayed`、`hasLastPlayed`、`deleted`。
- `equipmentItems` 只包含当前已穿戴装备，用于角色选择/登录外观展示；背包、仓库、交易行装备不返回。装备字段复用 `EquipmentItem`，会带 `equipSlot`、`weaponType`、`weaponTypeKey` 和当前配置解析出的展示属性。

#### POST `/api/character/create`

Body：`{"name":"Knight","appearance":{"hair":"demo"}}`。

- 认证：`account.gameplay`；`X-Account-Id` 必填。
- 名称为 2-12 个中文、字母、数字或下划线；账号最多 3 个未删除角色，服务端分配 `slotIndex`。成功 HTTP `201`；错误 code `2005`/`2006`。

#### POST `/api/character/delete`

Body：`{"characterId":10}`；也支持 body 内 `accountId`。

- 认证：`account.gameplay`；默认 `X-Account-Id` 必填。
- 软删除角色并释放 `slotIndex`；成功返回空对象。

#### GET `/api/player/profile`

- 认证：`account.gameplay`；`X-Account-Id` 与 `X-Character-Id`/`characterId` 必填。
- 成功：`player`、`economy`、`appearanceJson`、`characterName`。

#### POST `/api/player/save`

Body：`{"characterId":10,"posX":10.5,"posY":20.25,"currentMap":"town","playTimeSec":360,"hunger":92.5}`。

- 认证：`account.gameplay`；默认 `X-Account-Id` 必填，也支持 body 内 `accountId`。
- 只写非经济运行态：`currentMap` → `character_states.map_id`，`posX/posY` → `character_states.position` 的 `x/y`，`playTimeSec` → `character_states.play_time_sec`，`hunger` → `character_states.hunger`，并刷新 `character_states.last_played_at`。
- `characters` 保留角色身份、槽位、外观、等级和进度；运行时存档统一由 `character_states` 按 `character_id` 一对一保存。
- 不接受或不处理金币、AEB、背包、装备、经验等经济字段。

### 3.5 Launch Ticket 与副本恢复

#### POST `/api/game/launch`

```json
{"sessionId":"session_...","serverId":"world-a"}
```

- 认证：JWT；校验账号和持久化 Session。
- `serverId` 可省略；省略时自动选择公开状态为 `online` 且在线人数最少的服务器。
- 主页不传 `characterId`；传入未知字段会被拒绝。角色选择、创建和副本恢复在客户端进入后完成。
- 成功：`status=ready`、`ticket`、`expiresIn`、`expiresAt`、`serverId`、`host`、`port`、`publicEndpoint?`、`walletAddress`、`walletPlugin?`、`gameUrl?`；Ticket 有效期 90 秒。
- `gameUrl` 仅在配置 `GAME_CLIENT_BASE_URL` 时返回，格式为基础 URL 加 `ticket/serverId/host/port/walletAddress/walletPlugin/publicEndpoint?` 查询参数。
- 错误 code `4013`。

#### GET `/api/game/dungeon/recovery`

- 认证：JWT；Query `characterId` 为必填正整数。
- 无恢复：`required=false`。
- 有恢复：`required=true`、`dungeonRunId`、`accountId`、`characterId`、`serverId`、`chapterId`、`floorId`、`startedAt`。
- PostgreSQL 确认 run 仍为 `STARTED`；Redis 旧值不能复活已结束副本。错误 code `4014`。

#### POST `/api/game/dungeon/recovery`

```json
{"characterId":10,"dungeonRunId":"uuid","action":"resume","sessionId":"session_..."}
```

- 认证：JWT，并再次持久化校验 Session。
- `resume`：返回 `RESUME_READY`、原 `serverId`、90 秒 Ticket、`expiresAt`。
- `abandon`：设置 `CANCELLED`，作废未消费 Ticket，不发 Exp/Loot/AEB。
- 重复 abandon 幂等。状态冲突：HTTP `409` / code `4015`。

#### POST `/api/game/launch/consume`

```json
{"ticket":"ticket_...","serverId":"world-a"}
```

- 认证：`account.gameplay`；身份 subject 必须等于 `serverId`。
- Ticket 必须未过期、未消费、绑定当前服务器，且对应 Session 仍为 `ACTIVE`。
- 成功：`accountId`、`walletAddress`、`walletPlugin?`、`sessionId`、`serverId`。不建立 Online Presence。
- 客户端在游戏内选角后，由游戏服调用 `POST /api/game/online/enter` 绑定 `characterId` 和 `connectionId`。
- 错误 code `4013`；越权 code `4030`。

### 3.6 游戏服与 Online Presence

#### POST `/api/game/servers/register`

```json
{"serverId":"world-a","displayName":"World A","region":"ap-east","host":"10.0.0.10","port":7001,"publicEndpoint":"wss://world-a.example.com","maxPlayers":500,"onlinePlayers":0,"status":"ONLINE"}
```

- 认证：`account.gameplay`；subject/serverId 强绑定。
- `serverId`、`host`、正数 `port` 必填；默认 `maxPlayers=50`、`status=ONLINE`。

#### POST `/api/game/servers/heartbeat`

Body：`{"serverId":"world-a","onlinePlayers":120}`。

- 认证：`account.gameplay`；subject/serverId 强绑定。
- 成功返回更新后的 GameServer，并刷新 `lastHeartbeatAt`。

#### GET `/api/game/servers`

- 认证：`account.gameplay`；Query `status?`。
- 成功：`items: GameServer[]`。

#### POST `/api/game/online/enter`

```json
{"accountId":1,"characterId":10,"sessionId":"session_...","serverId":"world-a","connectionId":"connection-01"}
```

- 认证：`account.gameplay`；subject/serverId 强绑定。
- Session 与角色归属必须有效；空 `connectionId` 由服务端生成；新状态替换同账号旧状态。

#### POST `/api/game/online/heartbeat`

Body：`{"accountId":1,"connectionId":"connection-01"}`。

- 只能刷新属于当前签名游戏服的状态；成功返回 OnlineSession。

#### POST `/api/game/online/leave`

Body：`{"accountId":1,"connectionId":"connection-01"}`。

- 只能移除当前游戏服的状态；成功返回被移除的 OnlineSession。

#### GET `/api/game/online`

- 认证：`account.gameplay`；Query/Header 提供 `accountId`。
- 只允许 OnlineSession 所属游戏服查询；不在线为 HTTP `404` / code `4204`。

#### GET `/api/game/online/server`

- 认证：`account.gameplay`；Query `serverId` 必填并匹配身份 subject。
- 成功：`items`、`count`。

#### POST `/api/game/online/sweep`

- 认证：`account.ops`；Body 必须为 `{}`。
- 清理早于 Online TTL 两倍的记录；成功：`swept`。

## 4. economy-api

所有 `economy.gameplay` 路由由游戏服调用，并携带 `X-Account-Id`；需要角色的接口还应携带 `X-Character-Id` 或在 Body 传 `characterId`。

推荐调用顺序：

1. 进入游戏后先读 `/api/economy/snapshot`。
2. 背包、装备、仓库、商店、抽奖和交易市场都使用 `opId` 做幂等。
3. 链上付款类接口先创建 Payment Order，再提交交易签名，最后由支付操作器确认履约。
4. Worker/Internal 路由只给后台服务使用，玩家客户端不应直连。

### 4.1 健康、公告与快照

#### GET `/health`

- 认证：公开；只表示进程存活。

#### GET `/ready`

- 认证：公开；检查 PostgreSQL、Schema、经济配置；`STUB_MODE=disabled` 时也检查 Solana RPC。
- Required 依赖失败返回 HTTP `503`。

#### GET `/api/announcements/active`

- 认证：公开。
- Query：`afterId?`（只返回 ID 大于该值的公告）、`limit?`（默认 50，最大 200）。
- 只返回当前有效的 `OPS_NOTICE`，即 `startsAt <= now` 且 `endsAt` 为空或晚于当前时间，且未撤销。
- 成功：`items: Announcement[]`、`count`、`afterId`。
- 用途：客户端、启动器、官网拉取活动/维护/更新等公开通知。

#### GET `/api/economy/announcements/active`

- 认证：`economy.gameplay`。
- Query：`afterId?`、`limit?`。
- 返回当前有效的全服公告流，包括 `OPS_NOTICE` 和 `RARE_REWARD`。
- 用途：游戏服同步运维通知，以及展示真实发奖事务生成的稀有装备/坐骑弹窗。

#### GET `/api/economy/snapshot`

- 认证：`economy.gameplay`；Header：`X-Account-Id`、`X-Character-Id`。
- 成功：EconomySnapshot；不存在为 HTTP `404` / code `2008`。

### 4.2 仓库、装备与装备回收

以下四个接口共用 Body；不相关字段省略，省略 `slotIndex`/`equipSlot` 时内部值为 `-1`。

```json
{"opId":"unique-operation-id","characterId":10,"slotIndex":3,"quantity":1,"equipmentUid":"eq_...","equipSlot":0}
```

| 方法与路径 | 用途 | 主要字段 | 成功响应 |
| --- | --- | --- | --- |
| POST `/api/economy/warehouse/deposit` | 背包 → 仓库 | 物品用 `slotIndex+quantity`，装备用 `equipmentUid` | `snapshot` |
| POST `/api/economy/warehouse/withdraw` | 仓库 → 背包 | 同上 | `snapshot` |
| POST `/api/economy/equipment/equip` | 穿戴装备 | `equipmentUid`、`equipSlot` | `snapshot` |
| POST `/api/economy/equipment/unequip` | 卸下装备 | `equipmentUid` | `snapshot` |

认证均为 `economy.gameplay`；错误通常为 HTTP `400` / code `3001`。

#### POST `/api/economy/equipment/repair`

Body：`{"opId":"repair-01","characterId":10,"equipmentUid":"eq_..."}`。

- 认证：`economy.gameplay`。
- 不能修复已删除、销毁、上架、上链或锁定 Mint 的装备。
- 成功：`equipment`、`costToken`、`repairedPoints`、`snapshot`；错误 code `3601`。

#### POST `/api/economy/equipment/enhance`

```json
{"opId":"enhance-01","characterId":10,"equipmentUid":"eq_..."}
```

- 认证：`economy.gameplay`。
- 只允许当前装备模板、未关联 NFT、且位于背包或仓库的装备；坐骑和旧模板装备不适用。
- 配置当前开放上限为 `+10`。每次必定成功，随机选中一个**词条实例**，将该实例的 `enhanceHits` 加一；同名词条的两个实例相互独立。
- `+1` 到 `+5` 消耗配置金币；`+6` 到 `+10` 额外消耗与装备阶段相同的强化石。费用、阶段、品质和最高等级全部由服务端配置决定，客户端不能传入费用或强化石 ID。
- 成功：`equipment`、`goldCost`、`stoneItemId?`、`stoneQuantity?`、`enhancedAffix`、`snapshot`。`enhancedAffix.value` 是实时展示值；每次命中按初始配置值增加 10%。
- 强化上限、余额不足、石头不足、NFT/位置不合法均返回 HTTP `400` / code `3602`。

#### POST `/api/economy/equipment/npc-recycle`

```json
{"opId":"npc-recycle-01","characterId":10,"equipmentUid":"eq_..."}
```

- 认证：`economy.gameplay`。
- 仅允许背包内、当前装备模板的非 NFT 装备。已存在 `MINT_REQUESTED` 或 `MINTED` NFT Asset 的装备一律拒绝。
- 回收价只由装备模板的阶段与品质配置决定，不受副词条或强化影响；服务端将金币加入角色钱包。
- 装备状态变为 `NPC_RECYCLED`，不再出现在经济快照中；返回 `goldCredit`、`expiresAt`、`snapshot`。
- `expiresAt` 固定为回收后 7 天。在窗口到期前仅可由后续超级管理员恢复能力处理；到期物理删除后不可回溯。错误 code `3603`。

> 部署此接口前必须应用数据库更新 `20260713_equipment_npc_recycle_v1`；详见 `migrations/updates/`。

### 4.3 商店、抽奖与悬赏

#### GET `/api/economy/shop/catalog`

查询普通商人的当前货架。杂货商：`/api/economy/shop/catalog?shopId=general_merchant`；装备商：`/api/economy/shop/catalog?shopId=equipment_merchant`。不传 `shopId` 时默认 `general_merchant`。

- 认证：`economy.gameplay`。
- 返回 `items[]`，包含 `slotIndex`、`itemId`、`buyCurrency`、`buyPrice`、`dailyLimit`、`purchasedToday`、`remainingToday`、`available`。
- 普通商店按角色独立统计每日槽位限购，营业日按商店 `dailyLimitTimezone`（默认 `Asia/Shanghai`）切天，返回 `businessDate` 与 `nextResetAt`。
- 装备商只返回角色等级当前可装备的蓝色装备；神秘商人不走此接口，继续使用 `/api/economy/shop/mystery`。

#### GET `/api/economy/shop/mystery`

查询神秘商人当前货架。不传 `shopId` 时默认 `mystery_merchant`。

- 认证：`economy.gameplay`。
- 神秘商人槽位没有单独解锁接口，按角色等级自动解锁，并在返回中给出 `unlockedSlots` 与 `maxSlots`。
- 当前槽位规则：`0-9级` 开 1 槽，`10-19级` 开 2 槽，`20-29级` 开 3 槽，`30级+` 开 4 槽，当前最多 4 槽。
- 返回 `offers[]` 只包含已解锁槽位的商品；每个商品带 `slotIndex`、`itemId`、`quantity`、`rarity`、`goldPrice` 或 `tokenPrice`、`discountBps`、`purchased`。
- 如果角色第一次打开神秘商人，或当前货架所有商品都已购买完，后端会免费生成一轮新货架；只要仍有任意未购买商品，查看接口不会自动刷新。
- `freeRefreshAvailable` 表示当前是否已售罄并可免费刷新；`nextManualRefreshTokenCost` 表示下一次手动刷新需要消耗的 AEB。

#### POST `/api/economy/shop/mystery/refresh`

手动刷新神秘商人货架。

- 认证：`economy.gameplay`。
- Body：`{"opId":"mystery-refresh-01","characterId":10,"shopId":"mystery_merchant"}`；不传 `shopId` 时默认 `mystery_merchant`。
- 手动刷新消耗 AEB，可以使用锁定 AEB；刷新价格每天按角色独立重置，第一次 10，第二次 15，每次 +5，最高 50，之后当天每次都是 50。
- 刷新后仍按角色等级只生成已解锁槽位的商品。

#### POST `/api/economy/shop/buy`

Body：`{"opId":"buy-01","characterId":10,"shopId":"shop","itemId":"ashwood_white","quantity":1}`。

- 认证：`economy.gameplay`。
- 商品可买性由 `configs/economy/shops.json` 决定；普通商人的槽位和每日限量也由 `shops.json` 决定。
- 价格和币种默认由 `items.json` 的 `buyPrice`、`buyCurrency`、`grantGold` 决定；普通商店条目可以用 `sellItems[].buyPrice` 覆盖买价。
- `buyCurrency=0`：扣角色金币并立即交割，返回 `snapshot`。
- `buyCurrency=1`：创建 `SHOP_BUY` Payment Order，付款确认后按订单 payload 交割；返回 `order`。
- 普通物品进入 `inventory_items`；装备类物品创建唯一 `equipment_items` 实例；`grantGold > 0` 的商品只发金币。
- `buyPrice=0` 或未配置价格时拒绝成交。
- 普通商店购买会扣减对应 `slotIndex` 的今日剩余数量；超过 `dailyLimit` 时拒绝成交。

#### POST `/api/economy/shop/sell`

普通物品 Body：`{"opId":"sell-01","characterId":10,"shopId":"shop","slotIndex":0,"quantity":1}`。

装备 Body：`{"opId":"sell-eq-01","characterId":10,"shopId":"shop","equipmentUid":"eq_..."}`。

- 认证：`economy.gameplay`。
- 出售价由 `items.json` 的 `sellPrice` 决定；`sellPrice=0` 或未配置时拒绝成交。
- 普通物品必须在背包指定槽；装备必须是背包内非 NFT 装备。成功后资产变为 `CONSUMED` 并给角色金币，返回 `snapshot`。

#### POST `/api/economy/lottery/draw`

```json
{"opId":"lottery-01","characterId":10,"count":10}
```

- 认证：`economy.gameplay`；`count` 为 1 到 10。
- 服务端按当前 `lottery.json` 创建 `LOTTERY_DRAW` Payment Order，每抽固定 30 AEB，并在订单创建时快照类别权重、品质权重和完整奖励计划。

#### 悬赏板

所有接口认证均为 `economy.gameplay`。写操作必须携带唯一的 `opId`；未传
`characterId` 时使用 `X-Character-Id`。

| 路径 | Body | 说明 |
| --- | --- | --- |
| GET `/api/economy/bounty/board` | — | 返回五个槽位、解锁状态、当前任务与免费刷新时间；首次读取会补齐已解锁空槽。 |
| POST `/api/economy/bounty/slots/unlock-gold` | `{"opId":"...","characterId":10}` | 扣除 3,000 Gold 解锁槽位 2。 |
| POST `/api/economy/bounty/slots/unlock-aeb` | `{"opId":"...","characterId":10,"slotIndex":3}` | 创建槽位 3–5 的 Payment Order；确认付款后账号解锁。 |
| POST `/api/economy/bounty/refresh` | `{"opId":"...","characterId":10,"mode":"free|gold|premium"}` | 只替换 ACTIVE 普通任务；premium 创建 Payment Order，确认后恰有一条替换为稀有装备提交任务。 |
| POST `/api/economy/bounty/progress/combat` | `{"opId":"...","characterId":10,"dungeonRunId":"uuid"}` | 仅 GAME_SERVER 身份可用；后端只采用已结算副本中持久化的击杀事实，且一个 run 只能提交一次。 |
| POST `/api/economy/bounty/submit-equipment` | `{"opId":"...","characterId":10,"slotIndex":1,"equipmentUid":"..."}` | 仅消耗背包中满足稀有度、非 NFT、未上架的装备。 |
| POST `/api/economy/bounty/claim` | `{"opId":"...","characterId":10,"slotIndex":1}` | 原子写入任务徽章并将任务标记 CLAIMED。 |
| POST `/api/economy/bounty/badges/draw` | `{"opId":"...","characterId":10,"badge":"common|rare"}` | 原子扣徽章并发 Gold、物品或 Locked AEB。 |

- 涉及 AEB 的悬赏操作通过既有 `/api/economy/internal/payments/confirm` 确认后履约。
- 配置后续更新不会影响已经创建的 Payment Order。
- 参数、配置、角色等级或付款接收钱包不合法：HTTP `400` / code `3610`。

### 4.4 NFT

#### POST `/api/economy/nft/mint/request`

Body：`{"opId":"mint-request-01","characterId":10,"equipmentUid":"eq_..."}`。

- 认证：`economy.gameplay`。
- 按稀有度快照 AEB 费用，锁定装备并创建后台请求。
- 成功：`request`、`asset`、`snapshot`；错误 code `3901`。

#### POST `/api/economy/nft/mint/cancel`

Body：`{"opId":"mint-cancel-01","requestId":1001}`。

- 认证：`economy.gameplay`。
- 只能取消所属账号的可取消请求；按原 locked/withdrawable/external 类别退款。
- 成功：NFTMintRequestResult；错误 code `3902`。

#### POST `/api/economy/internal/nft/mint/confirm`

```json
{"opId":"mint-confirm-01","requestId":1001,"mintAddress":"<asset-address>","txSignature":"<solana-signature>","metadataUri":"https://..."}
```

- 认证：`economy.mint`。
- `STUB_MODE=disabled` 当前返回 HTTP `503` / code `3903`，直到 Metaplex Core 验证适配器接入。
- Development/test Stub 模式成功返回 NFTMintRequestResult。

#### GET `/api/economy/nft/assets`

- 认证：`economy.gameplay`；`X-Account-Id` 必填。
- 成功：`items: NFTAsset[]`。NFTAsset 主要字段为 `id`、`accountId`、`sourceAssetType`、`sourceAssetId`、`mintAddress?`、`metadataUri?`、`status`、`equipmentUid?`。

### 4.5 副本与掉落

#### POST `/api/economy/dungeon/enter`

```json
{"opId":"dungeon-enter-01","characterId":10,"chapterId":0,"floorId":1}
```

- 认证：`economy.gameplay`。
- 签名游戏服必须与玩家 Online Presence 的 `serverId`、`characterId` 一致。
- 同一角色只能有一个 `STARTED` run；扣除配置中的进入成本并记录 `originServerId`。
- Boss 层的 `enterCost.items` 包含门票时，系统在创建 Run 时从背包原子扣除；缺少门票不会创建 Run。
- 成功：`dungeonRunId`、章节/层、`status=IN_PROGRESS`、成本、空奖励、snapshot。
- 在线归属不一致：HTTP `403` / code `3100`；其他错误 code `3101`。

#### POST `/api/economy/dungeon/finish`

```json
{
  "opId":"dungeon-finish-01",
  "characterId":10,
  "dungeonRunId":"uuid",
  "chapterId":0,
  "floorId":1,
  "result":"victory",
  "exp":120,
  "kills":[{"enemyId":1,"enemyName":"Slime","quantity":3}],
  "progress":{"reachedWave":5}
}
```

- 认证：`economy.gameplay`；只有 `originServerId` 对应身份能结束。
- `result` 为 `victory|defeat|timeout`；`exp >= 0` 且不能超过配置上限。
- 仅 `STARTED` run 可结束；账号、角色、章节、层必须一致。
- 胜利响应状态为 `REWARDED`，失败/超时为 `FAILED`。
- 相同 `opId` 重试返回原结果；不同 `opId` 不能二次结算。错误 code `3102`。

#### Loot Tray

共用 Body：

```json
{"opId":"loot-op-01","characterId":10,"lootId":100,"slotIndex":2,"quantity":1}
```

| 方法与路径 | 说明 | 特殊字段 |
| --- | --- | --- |
| POST `/api/economy/loot/claim-player` | 领取单项奖励 | `lootId`；普通物品可传目标 `slotIndex`/`quantity` |
| POST `/api/economy/loot/claim-all` | 批量领取可放入背包的奖励 | `lootId` 可省略 |
| POST `/api/economy/loot/discard` | 丢弃奖励 | `lootId` |

认证均为 `economy.gameplay`；成功返回 `snapshot`；错误 code `3201`。

### 4.6 采集、农场和 Boss

#### POST `/api/economy/gathering/settle`

Body：`{"opId":"gather-01","characterId":10,"nodeId":"iron-node"}`。

- 根据 `nodeId` 配置生成奖励；物品/装备进入背包，AEB 进入 Locked AEB。
- 成功：`activityId`、`activityType`、`rewards`、`snapshot`；错误 code `3301`。

#### POST `/api/economy/farming/harvest`

Body：`{"opId":"harvest-01","characterId":10,"cropId":"wheat"}`。

- 与采集相同，但读取 Crop 配置；错误 code `3302`。

#### POST `/api/economy/boss/contribute`

Body：`{"opId":"boss-contrib-01","characterId":10,"bossEventId":99,"contribution":50}`。

- 成功：`bossEventId`、`bossKey`、累计 `contribution`；错误 code `3401`。

#### POST `/api/economy/boss/settle`

Body：`{"opId":"boss-settle-01","characterId":10,"bossEventId":99,"bossKey":"world-boss-1"}`。

- 根据参与门槛、贡献和配置池结算。
- 成功：`bossEventId`、`bossKey`、`contribution`、`rewards`、`snapshot`；错误 code `3402`。

以上四个玩家活动接口认证均为 `economy.gameplay`。

#### POST `/api/economy/internal/boss/events/open`

```json
{"opId":"boss-open-01","bossKey":"world-boss-1","startsAt":"2026-07-13T10:00:00Z","endsAt":"2026-07-13T11:00:00Z","metadata":{}}
```

- 认证：`economy.boss_ops`；时间可省略，提供时必须为 RFC3339。
- 成功：HTTP `201`，返回 BossEvent；错误 code `3403`。

#### POST `/api/economy/internal/boss/events/close`

Body：`{"opId":"boss-close-01","bossEventId":99}`。

- 认证：`economy.boss_ops`；成功返回关闭后的 BossEvent；错误 code `3404`。

#### POST `/api/economy/internal/boss/events/settle`

Body：`{"opId":"boss-mark-settled-01","bossEventId":99}`。

- 认证：`economy.boss_ops`；成功返回已标记 settled 的 BossEvent；错误 code `3405`。

#### GET `/api/economy/internal/boss/events/active`

- 认证：`economy.boss_ops`；成功：`events: BossEvent[]`。

### 4.7 背包整理、丢弃与合成

#### POST `/api/economy/inventory/organize`

Body：`{"opId":"bag-organize-01","characterId":10}`。成功返回 `snapshot`；错误 code `3501`。

#### POST `/api/economy/warehouse/organize`

Body：`{"opId":"warehouse-organize-01","characterId":10}`。成功返回 `snapshot`；错误 code `3504`。

#### POST `/api/economy/inventory/discard`

- 普通物品：`{"opId":"discard-01","characterId":10,"slotIndex":3,"quantity":2}`。
- 装备：`{"opId":"discard-02","characterId":10,"equipmentUid":"eq_..."}`。
- 成功返回 `snapshot`；错误 code `3502`。

#### POST `/api/economy/inventory/synthesize`

Body：`{"opId":"synth-01","characterId":10,"recipeId":"iron-sword","batchCount":1}`。

- `recipeId` 必须存在；批次数受材料和 Recipe 约束。
- 成功返回 `snapshot`；错误 code `3503`。

以上四个接口认证均为 `economy.gameplay`。

### 4.8 背包扩容与交易许可证

#### POST `/api/economy/inventory/bag/expand`

Body：`{"opId":"bag-expand-01","characterId":10}`。

- 认证：`economy.gameplay`。
- 创建 `BAG_EXPAND` Payment Order，不会在未验证付款时直接扩容。
- 成功：`order`、`bagExpandCount`、`bagSlots`；错误 code `3720`。

#### POST `/api/economy/license/purchase`

Body：`{"opId":"license-01","characterId":10}`。

- 认证：`economy.gameplay`。
- 创建 `TRADING_LICENSE` Payment Order。
- 成功：`order`、`hasLicense`；错误 code `3721`。
- `/api/economy/license/buy` 是同一处理器的别名。

### 4.9 Marketplace

#### GET `/api/economy/marketplace/listings`

- 认证：`economy.gameplay`。
- Query：`status?`（默认 `LISTED`）、`assetType?`、`itemId?`、`limit?`、`offset?`。
- 成功：`items`、`limit`、`offset`；错误 code `3701`。

#### GET `/api/economy/marketplace/listings/mine`

- 认证：`economy.gameplay`；`X-Account-Id` 必填。
- Query：`status?`、`limit?`、`offset?`。
- 成功：`items`、`limit`、`offset`；错误 code `3702`。

#### GET `/api/economy/marketplace/slots`

- 认证：`economy.gameplay`。
- 成功：`accountId`、`baseSlots`、`materialExpandCount`、`walletExpandCount`、`capacity`、`used`、`available`。
- 错误 code `3703`。

#### POST `/api/economy/marketplace/list`

装备示例：

```json
{"opId":"list-eq-01","characterId":10,"assetType":"EQUIPMENT","equipmentUid":"eq_...","sourceLocation":"BAG","quantity":1,"priceToken":100}
```

普通物品使用 `assetType=ITEM`、`slotIndex`、`quantity`。

- 认证：`economy.gameplay`；成功：`listing`、`snapshot`；错误 code `3704`。

#### POST `/api/economy/marketplace/listings/{listingId}/buy`

Body：`{"opId":"buy-01","characterId":10}`。

- 认证：`economy.gameplay`；`listingId` 必须为正整数。
- 成功：`listing`、`order`、`snapshot`；错误 code `3705`/`3706`。

#### POST `/api/economy/marketplace/listings/{listingId}/cancel`

Body：`{"opId":"cancel-listing-01"}`。

- 认证：`economy.gameplay`；只能取消自己的可取消挂单。
- 成功：`listing`、`snapshot`；错误 code `3705`/`3707`。

#### POST `/api/economy/marketplace/slots/expand-material`

Body：`{"opId":"slot-material-01","characterId":10}`。

- 认证：`economy.gameplay`；按配置消耗材料。
- 成功：`slots`、`snapshot`；错误 code `3708`。

#### POST `/api/economy/marketplace/slots/expand-wallet`

Body：`{"opId":"slot-wallet-create-01","characterId":10}`。

- 认证：`economy.gameplay`；创建 `MARKET_SLOT_WALLET_EXPAND` Payment Order。
- 成功：`order`、`slots`；错误 code `3709`。

#### POST `/api/economy/marketplace/slots/expand-wallet/submit`

```json
{"opId":"slot-wallet-submit-01","orderId":"uuid","txSignature":"<solana-signature>"}
```

- 认证：`economy.gameplay`。
- 通过 Solana RPC 校验付款、收款钱包、Mint、金额和唯一回执。
- 成功：PaymentOrder；错误 code `3710`。

#### POST `/api/economy/internal/payments/submit`

- 认证：`economy.payments`。
- Body 与上一接口相同，用于独立支付操作器；校验和幂等规则相同。

MarketplaceListing 主要字段：`id`、卖家账号/角色、`assetType`、`assetId`、`itemId`、`quantity`、`priceToken`、`listingDepositToken`、`feeBps`、`status` 和时间字段。

### 4.10 AEB 与账本

#### POST `/api/economy/rewards/grant-locked`

```json
{"amount":25,"source":"quest","ref":"quest-100","cooldownHours":24}
```

- 认证：`economy.rewards`；`X-Account-Id` 必填；`amount > 0`。
- 成功：HTTP `201`，返回 LockedGame；错误 code `3000`。

#### POST `/api/chain/token/claim`

Body：`{"amount":100,"wallet":"<solana-wallet>"}`。

- 认证：`economy.gameplay`；`amount > 0` 且有足够 withdrawable balance。
- 根据限额进入 `QUEUED` 或 `MANUAL_REVIEW`。
- 成功：HTTP `201`，返回 Withdrawal；错误 code `3600`。

#### GET `/api/chain/token/ledger`

- 认证：`economy.gameplay`；`X-Account-Id` 必填。
- 成功：`entries: LedgerEntry[]`。

### 4.11 Worker 与链操作命令

以下 Worker 命令通常发送 `{}`；`npc-recycle/purge` 可额外传 `limit`：

| 方法与路径 | Capability | 成功响应 | 错误码 |
| --- | --- | --- | ---: |
| POST `/api/economy/internal/unlocks/settle` | `economy.worker` | `settled: LockedGame[]` | 400 |
| POST `/api/economy/internal/withdrawals/process` | `economy.worker` | `processed: Withdrawal[]` | 400 |
| POST `/api/economy/internal/chain/deposits/scan` | `economy.worker` | DepositScanResult | 3801 |
| POST `/api/economy/internal/chain/payouts/submit` | `economy.worker` | PayoutJobResult | 3802 |
| POST `/api/economy/internal/chain/payouts/confirm` | `economy.worker` | PayoutJobResult | 3803 |
| POST `/api/economy/internal/equipment/npc-recycle/purge` | `economy.worker` | `purged` | 5601 |

DepositScanResult：`scanned`、`credited`、`ignored`、`paymentsFulfilled`、`cursorSlot`、`signatures?`、`disabled?`、`message?`。

PayoutJobResult：`processed`、`submitted`、`confirmed`、`failed`、`ids?`、`disabled?`、`message?`。

#### POST `/api/economy/internal/equipment/npc-recycle/purge`

Body：`{"limit":100}`；无参数时也必须传 `{}`。

- 认证：`economy.worker`。
- 仅删除 `location=NPC_RECYCLED` 且 `npcRecycleExpiresAt <= now` 的记录；默认/最大批量为 `1000`。
- 成功：`{"purged": <count>}`。该操作只应由可信 worker 定时调用，删除后装备本体无法恢复。

#### POST `/api/economy/internal/payments/confirm`

Body：`{"orderId":"uuid","reason":"chain confirmation"}`。

- 认证：`economy.payments`。
- 只允许已提交/已确认且具有交易签名的订单；不能直接确认 `PENDING_PAYMENT`。
- 成功：PaymentOrder；错误 code `3804`。

## 5. 请求与响应示例

本节按调用场景给出常用入参和出参示例。完整接口清单见 [接口列表](interface-list.md)。除 `/ready` 外，HTTP 接口均返回统一信封：

```json
{"ok":true,"data":{}}
```

`/ready` 由 readiness probe 直接返回 `ready/service/checks`，不包在 `ok/data` 信封内。示例中的时间、ID、Token、签名和钱包地址均为占位值；`X-Account-Id`、`X-Character-Id` 表示由可信游戏服或内部服务传入的上下文，不应由玩家客户端自行填写。表格中的 `{...EconomySnapshot}`、`{...PaymentOrder}`、`{...BountyBoard}` 表示嵌入上方同名复用模型示例。

### 5.1 复用响应模型示例

#### EconomySnapshot 示例

```json
{
  "accountId": 1,
  "characterId": 10,
  "gold": 1200,
  "gems": 0,
  "stamina": 100,
  "bagSlots": 25,
  "bagExpandCount": 0,
  "hasLicense": true,
  "level": 3,
  "exp": 240,
  "accountToken": {
    "accountId": 1,
    "tokenBalance": 0,
    "withdrawableBalance": 80,
    "lockedBalance": 25,
    "externalBalance": 0,
    "unlockCredit": 0
  },
  "inventory": [{"id": 101, "itemId": "iron_ore", "quantity": 3, "slot": 0, "durability": 0, "enhanceLevel": 0}],
  "warehouse": [],
  "lootTray": [],
  "equipmentItems": [{
    "id": 501,
    "equipmentUid": "eq_abc",
    "itemId": "ashbound_sword_t1",
    "rarity": 3,
    "enhanceLevel": 1,
    "durability": 95,
    "maxDurability": 100,
    "status": "ACTIVE",
    "equipSlot": -1,
    "slot": 4,
    "weaponType": 1,
    "weaponTypeKey": "sword",
    "nftContract": null,
    "nftTokenId": null,
    "affixes": [{"affixId": "atk_flat", "instanceId": "atk_flat#1", "enhanceHits": 1, "stat": "attack", "value": 12}]
  }]
}
```

#### PaymentOrder 示例

```json
{
  "id": "pay_00000000-0000-4000-8000-000000000001",
  "accountId": 1,
  "characterId": 10,
  "purpose": "MARKET_SLOT_WALLET_EXPAND",
  "payAsset": "AEB",
  "amount": 100,
  "receiverWallet": "DepositWalletBase58",
  "status": "PENDING_PAYMENT",
  "createdAt": "2026-07-14T10:00:00Z",
  "expiresAt": "2026-07-14T10:10:00Z"
}
```

#### BountyBoard 示例

```json
{
  "slots": [
    {"slotIndex": 1, "unlocked": true, "task": {"id": 301, "slotIndex": 1, "templateId": "gather_ore", "type": "gather", "difficulty": "normal", "itemId": "iron_ore", "requiredQuantity": 10, "progressQuantity": 3, "status": "ACTIVE", "rewardItemId": "bounty_badge_common", "rewardQuantity": 1}},
    {"slotIndex": 2, "unlocked": false}
  ],
  "freeRefreshAvailableAt": "2026-07-14T18:00:00Z"
}
```

### 5.2 account-api 示例

| 接口 | 入参示例 | 出参示例 |
| --- | --- | --- |
| GET `/health` | 无 | `{"ok":true,"data":{"service":"account-api","status":"ok"}}` |
| GET `/ready` | 无 | `{"ready":true,"service":"account-api","checks":[{"name":"postgres","status":"ok","required":true}]}` |
| GET `/api/auth/wallet/nonce` | Query：`walletAddress=<solana-wallet>` | `{"ok":true,"data":{"nonce":"nonce_x","message":"Sign in to Aeonblight\\nWallet: ...","expiresAt":"2026-07-14T10:05:00Z"}}` |
| POST `/api/auth/wallet` | Body：`{"walletAddress":"<solana-wallet>","walletPlugin":"phantom","nonce":"nonce_x","signature":"<base58-signature>","deviceId":"device-01"}` | `{"ok":true,"data":{"account":{"id":1,"username":"player_1","walletAddress":"<solana-wallet>","isBanned":false,"createdAt":"2026-07-14T10:00:00Z","lastLoginAt":"2026-07-14T10:00:00Z"},"accessToken":"jwt...","refreshToken":"refresh_x","sessionId":"session_x","walletPlugin":"phantom","expiresAt":"2026-07-21T10:00:00Z"}}` |
| POST `/api/auth/refresh` | Body：`{"refreshToken":"refresh_x"}` | `{"ok":true,"data":{"accessToken":"jwt...","refreshToken":"refresh_y","sessionId":"session_x","expiresIn":7200}}` |
| POST `/api/auth/logout` | Header：`Authorization: Bearer <accessToken>`；Body：`{"sessionId":"session_x","refreshToken":"refresh_y"}` | `{"ok":true,"data":{"status":"revoked","sessionId":"session_x"}}` |
| GET `/api/auth/verify` | Header：`Authorization: Bearer <accessToken>` | `{"ok":true,"data":{"id":1,"username":"player_1","walletAddress":"<solana-wallet>","isBanned":false,"createdAt":"2026-07-14T10:00:00Z","lastLoginAt":"2026-07-14T10:00:00Z"}}` |
| GET `/api/auth/session/redis` | Service Identity：`account.ops` | `{"ok":true,"data":{"enabled":true,"ok":true,"mode":"redis+postgres"}}` |
| GET `/api/public/servers` | 无 | `{"ok":true,"data":{"servers":[{"serverId":"world-a","name":"World A","curPlayers":120,"maxPlayers":500,"status":"online","queueLength":0,"region":"ap-east"}]}}` |
| GET `/api/public/servers/online` | 无 | `{"ok":true,"data":{"servers":[{"serverId":"world-a","name":"World A","curPlayers":120,"maxPlayers":500,"status":"online","queueLength":0,"region":"ap-east"}]}}` |
| GET `/api/public/home/stats` | 无 | `{"ok":true,"data":{"onlinePlayers":125,"monthlyActivePlayers":263,"updatedAt":"2026-07-15T10:00:00Z"}}` |
| GET `/api/public/home/config` | 无 | `{"ok":true,"data":{"contractAddress":"solana_token_mint_or_ca","tokenSymbol":"AEB","gameClientBaseUrl":"https://game.example.com","supportWallets":["phantom","solflare","backpack","okx"],"updatedAt":"2026-07-15T10:00:00Z"}}` |
| GET `/api/public/leaderboards/clear-progress` | Query：`limit=10` | `{"ok":true,"data":{"items":[],"updatedAt":"2026-07-14T10:00:00Z"}}` |
| GET `/api/public/leaderboards/weekly-score` | Query：`limit=10` | `{"ok":true,"data":{"period":{"periodId":"weekly-score-20260701-0","startsAt":"2026-07-01T00:00:00Z","endsAt":"2026-07-08T00:00:00Z"},"items":[],"updatedAt":"2026-07-14T10:00:00Z"}}` |
| GET `/api/character/list` | Service Identity：`account.gameplay`；Header：`X-Account-Id: 1` | `{"ok":true,"data":{"characters":[{"id":10,"accountId":1,"name":"Knight","slotIndex":0,"level":1,"appearance":{},"equipmentItems":[],"createdAt":"2026-07-14T10:00:00Z","lastPlayed":"1970-01-01T00:00:00Z","hasLastPlayed":false,"deleted":false}]}}` |
| POST `/api/character/create` | Service Identity：`account.gameplay`；Header：`X-Account-Id: 1`；Body：`{"name":"Knight","appearance":{}}` | `{"ok":true,"data":{"id":10,"accountId":1,"name":"Knight","slotIndex":0,"level":1,"appearance":{},"equipmentItems":[],"createdAt":"2026-07-14T10:00:00Z","lastPlayed":"1970-01-01T00:00:00Z","hasLastPlayed":false,"deleted":false}}` |
| POST `/api/character/delete` | Service Identity：`account.gameplay`；Header：`X-Account-Id: 1`；Body：`{"characterId":10}` | `{"ok":true,"data":{}}` |
| GET `/api/player/profile` | Service Identity：`account.gameplay`；Header：`X-Account-Id: 1`；Query：`characterId=10` | `{"ok":true,"data":{"player":{"characterId":10,"currentMap":"town","posX":10.5,"posY":20.25},"economy":{...},"appearanceJson":{},"characterName":"Knight"}}` |
| POST `/api/player/save` | Service Identity：`account.gameplay`；Header：`X-Account-Id: 1`；Body：`{"characterId":10,"posX":10.5,"posY":20.25,"currentMap":"town","playTimeSec":360,"hunger":92.5}` | `{"ok":true,"data":{}}` |
| POST `/api/game/launch` | Header：`Authorization: Bearer <accessToken>`；Body：`{"sessionId":"session_x","serverId":"world-a"}` | `{"ok":true,"data":{"status":"ready","ticket":"ticket_x","expiresIn":90,"expiresAt":"2026-07-14T10:01:30Z","serverId":"world-a","host":"10.0.0.10","port":7001,"publicEndpoint":"wss://world-a.example.com","walletAddress":"<solana-wallet>","walletPlugin":"phantom","gameUrl":"https://client.example/play?ticket=ticket_x&serverId=world-a&host=10.0.0.10&port=7001&walletAddress=<solana-wallet>&walletPlugin=phantom"}}` |
| GET `/api/game/dungeon/recovery` | Header：`Authorization: Bearer <accessToken>`；Query：`characterId=10` | 无恢复：`{"ok":true,"data":{"required":false}}`；有恢复：`{"ok":true,"data":{"required":true,"status":"STARTED","dungeonRunId":"run_uuid","accountId":1,"characterId":10,"serverId":"world-a","chapterId":1,"floorId":3,"startedAt":"2026-07-14T09:55:00Z"}}` |
| POST `/api/game/dungeon/recovery` | Header：`Authorization: Bearer <accessToken>`；Body：`{"characterId":10,"dungeonRunId":"run_uuid","action":"resume","sessionId":"session_x"}` | Resume：`{"ok":true,"data":{"action":"resume","status":"RESUME_READY","dungeonRunId":"run_uuid","serverId":"world-a","ticket":"ticket_y","expiresAt":"2026-07-14T10:01:30Z"}}`；Abandon：`{"ok":true,"data":{"action":"abandon","status":"CANCELLED","dungeonRunId":"run_uuid"}}` |
| POST `/api/game/launch/consume` | Service Identity：`account.gameplay`，subject=`world-a`；Body：`{"ticket":"ticket_x","serverId":"world-a"}` | `{"ok":true,"data":{"accountId":1,"walletAddress":"<solana-wallet>","walletPlugin":"phantom","sessionId":"session_x","serverId":"world-a"}}` |
| POST `/api/game/servers/register` | Service Identity：`account.gameplay`，subject=`world-a`；Body：`{"serverId":"world-a","displayName":"World A","region":"ap-east","host":"10.0.0.10","port":7001,"publicEndpoint":"wss://world-a.example.com","maxPlayers":500,"onlinePlayers":0,"status":"ONLINE"}` | `{"ok":true,"data":{"serverId":"world-a","displayName":"World A","region":"ap-east","host":"10.0.0.10","port":7001,"publicEndpoint":"wss://world-a.example.com","maxPlayers":500,"onlinePlayers":0,"status":"ONLINE","registeredAt":"2026-07-14T10:00:00Z","lastHeartbeatAt":"2026-07-14T10:00:00Z"}}` |
| POST `/api/game/servers/heartbeat` | Service Identity：`account.gameplay`，subject=`world-a`；Body：`{"serverId":"world-a","onlinePlayers":120}` | `{"ok":true,"data":{"serverId":"world-a","displayName":"World A","host":"10.0.0.10","port":7001,"maxPlayers":500,"onlinePlayers":120,"status":"ONLINE","registeredAt":"2026-07-14T10:00:00Z","lastHeartbeatAt":"2026-07-14T10:01:00Z"}}` |
| GET `/api/game/servers` | Service Identity：`account.gameplay`；Query：`status=ONLINE` | `{"ok":true,"data":{"items":[{"serverId":"world-a","displayName":"World A","host":"10.0.0.10","port":7001,"maxPlayers":500,"onlinePlayers":120,"status":"ONLINE","registeredAt":"2026-07-14T10:00:00Z","lastHeartbeatAt":"2026-07-14T10:01:00Z"}]}}` |
| POST `/api/game/online/enter` | Service Identity：`account.gameplay`，subject=`world-a`；Body：`{"accountId":1,"characterId":10,"sessionId":"session_x","serverId":"world-a","connectionId":"conn-01"}` | `{"ok":true,"data":{"accountId":1,"characterId":10,"sessionId":"session_x","serverId":"world-a","connectionId":"conn-01","enteredAt":"2026-07-14T10:00:10Z","lastSeenAt":"2026-07-14T10:00:10Z"}}` |
| POST `/api/game/online/heartbeat` | Service Identity：`account.gameplay`；Body：`{"accountId":1,"connectionId":"conn-01"}` | `{"ok":true,"data":{"accountId":1,"characterId":10,"sessionId":"session_x","serverId":"world-a","connectionId":"conn-01","enteredAt":"2026-07-14T10:00:10Z","lastSeenAt":"2026-07-14T10:01:00Z"}}` |
| POST `/api/game/online/leave` | Service Identity：`account.gameplay`；Body：`{"accountId":1,"connectionId":"conn-01"}` | `{"ok":true,"data":{"accountId":1,"characterId":10,"sessionId":"session_x","serverId":"world-a","connectionId":"conn-01","enteredAt":"2026-07-14T10:00:10Z","lastSeenAt":"2026-07-14T10:01:00Z"}}` |
| GET `/api/game/online` | Service Identity：`account.gameplay`；Query/Header：`accountId=1` | `{"ok":true,"data":{"accountId":1,"characterId":10,"sessionId":"session_x","serverId":"world-a","connectionId":"conn-01","enteredAt":"2026-07-14T10:00:10Z","lastSeenAt":"2026-07-14T10:01:00Z"}}` |
| GET `/api/game/online/server` | Service Identity：`account.gameplay`，subject=`world-a`；Query：`serverId=world-a` | `{"ok":true,"data":{"items":[{"accountId":1,"characterId":10,"sessionId":"session_x","serverId":"world-a","connectionId":"conn-01","enteredAt":"2026-07-14T10:00:10Z","lastSeenAt":"2026-07-14T10:01:00Z"}],"count":1}}` |
| POST `/api/game/online/sweep` | Service Identity：`account.ops`；Body：`{}` | `{"ok":true,"data":{"swept":3}}` |

### 5.3 economy-api 示例

| 接口 | 入参示例 | 出参示例 |
| --- | --- | --- |
| GET `/health` | 无 | `{"ok":true,"data":{"service":"economy-api","status":"ok"}}` |
| GET `/ready` | 无 | `{"ready":true,"service":"economy-api","checks":[{"name":"postgres","status":"ok","required":true},{"name":"economy-rules","status":"ok","required":true}]}` |
| GET `/api/announcements/active` | 公开；Query：`afterId=0&limit=50` | `{"ok":true,"data":{"items":[{"id":1001,"kind":"OPS_NOTICE","status":"ACTIVE","displayMode":"BANNER","title":"限时活动开启","body":"世界 Boss 活动将于 20:00 开启","priority":700,"scope":"GLOBAL","startsAt":"2026-07-15T12:00:00Z","createdAt":"2026-07-15T11:00:00Z"}],"count":1,"afterId":0}}` |
| GET `/api/economy/snapshot` | Service Identity：`economy.gameplay`；Header：`X-Account-Id: 1`、`X-Character-Id: 10` | `{"ok":true,"data":{...EconomySnapshot}}` |
| GET `/api/economy/announcements/active` | Service Identity：`economy.gameplay`；Query：`afterId=0&limit=50` | `{"ok":true,"data":{"items":[...Announcement],"count":2,"afterId":0}}` |
| POST `/api/economy/warehouse/deposit` | Header：`X-Account-Id: 1`；Body：`{"opId":"wh-deposit-01","characterId":10,"slotIndex":0,"quantity":2}`；装备用 `{"opId":"wh-deposit-eq-01","characterId":10,"equipmentUid":"eq_abc"}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/warehouse/withdraw` | Header：`X-Account-Id: 1`；Body：`{"opId":"wh-withdraw-01","characterId":10,"slotIndex":0,"quantity":2}` 或 `{"opId":"wh-withdraw-eq-01","characterId":10,"equipmentUid":"eq_abc"}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/equipment/equip` | Header：`X-Account-Id: 1`；Body：`{"opId":"equip-01","characterId":10,"equipmentUid":"eq_abc","equipSlot":0}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/equipment/unequip` | Header：`X-Account-Id: 1`；Body：`{"opId":"unequip-01","characterId":10,"equipmentUid":"eq_abc"}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/equipment/repair` | Header：`X-Account-Id: 1`；Body：`{"opId":"repair-01","characterId":10,"equipmentUid":"eq_abc"}` | `{"ok":true,"data":{"equipment":{"id":501,"equipmentUid":"eq_abc","itemId":"ashbound_sword_t1","rarity":3,"enhanceLevel":1,"durability":100,"maxDurability":100,"status":"ACTIVE","equipSlot":-1,"slot":4,"weaponType":1,"weaponTypeKey":"sword","nftContract":null,"nftTokenId":null},"costToken":5,"repairedPoints":5,"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/equipment/enhance` | Header：`X-Account-Id: 1`；Body：`{"opId":"enhance-01","characterId":10,"equipmentUid":"eq_abc"}` | `{"ok":true,"data":{"equipment":{"id":501,"equipmentUid":"eq_abc","itemId":"ashbound_sword_t1","rarity":3,"enhanceLevel":2,"durability":100,"maxDurability":100,"status":"ACTIVE","equipSlot":-1,"slot":4,"weaponType":1,"weaponTypeKey":"sword","nftContract":null,"nftTokenId":null},"goldCost":100,"stoneItemId":"enhance_stone_t1","stoneQuantity":1,"enhancedAffix":{"affixId":"atk_flat","instanceId":"atk_flat#1","enhanceHits":2,"stat":"attack","value":13.2},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/equipment/npc-recycle` | Header：`X-Account-Id: 1`；Body：`{"opId":"npc-recycle-01","characterId":10,"equipmentUid":"eq_abc"}` | `{"ok":true,"data":{"goldCredit":120,"expiresAt":"2026-07-21T10:00:00Z","snapshot":{...EconomySnapshot}}}` |
| GET `/api/economy/shop/catalog?shopId=general_merchant` | Header：`X-Account-Id: 1`, `X-Character-Id: 10` | `{"ok":true,"data":{"shopId":"general_merchant","businessDate":"2026-07-16","nextResetAt":"...","items":[{"slotIndex":1,"itemId":"shadow_iron","dailyLimit":200,"remainingToday":200,"available":true}]}}` |
| POST `/api/economy/shop/buy` | Header：`X-Account-Id: 1`；Body：`{"opId":"shop-buy-01","characterId":10,"shopId":"town-general","itemId":"health_potion","quantity":2}` | 金币商品：`{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}`；链上商品：`{"ok":true,"data":{"order":{...PaymentOrder,"purpose":"SHOP_BUY"}}}` |
| POST `/api/economy/shop/sell` | 普通物品：`{"opId":"shop-sell-01","characterId":10,"shopId":"town-general","slotIndex":3,"quantity":1}`；装备：`{"opId":"shop-sell-eq-01","characterId":10,"shopId":"town-general","equipmentUid":"eq_abc"}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/lottery/draw` | Header：`X-Account-Id: 1`；Body：`{"opId":"lottery-01","characterId":10,"count":10}` | `{"ok":true,"data":{"order":{...PaymentOrder,"purpose":"LOTTERY_DRAW","amount":300},"rewards":{"IsBoss":false,"Items":[{"RewardType":"item","ItemID":"iron_ore","Quantity":5}],"TokenReward":0}}}` |
| GET `/api/economy/bounty/board` | Header：`X-Account-Id: 1`、`X-Character-Id: 10` | `{"ok":true,"data":{...BountyBoard}}` |
| POST `/api/economy/bounty/slots/unlock-gold` | Header：`X-Account-Id: 1`；Body：`{"opId":"bounty-gold-01","characterId":10}` | `{"ok":true,"data":{...BountyBoard}}` |
| POST `/api/economy/bounty/slots/unlock-aeb` | Header：`X-Account-Id: 1`；Body：`{"opId":"bounty-aeb-01","characterId":10,"slotIndex":3}` | `{"ok":true,"data":{"order":{...PaymentOrder,"purpose":"BOUNTY_SLOT_UNLOCK"},"board":{...BountyBoard}}}` |
| POST `/api/economy/bounty/refresh` | Header：`X-Account-Id: 1`；Body：`{"opId":"bounty-refresh-01","characterId":10,"mode":"free"}` | 免费/Gold：`{"ok":true,"data":{...BountyBoard}}`；Premium：`{"ok":true,"data":{"order":{...PaymentOrder,"purpose":"BOUNTY_PREMIUM_REFRESH"}}}` |
| POST `/api/economy/bounty/progress/combat` | Service Identity：`economy.gameplay` 且为 `GAME_SERVER`；Body：`{"opId":"bounty-combat-01","characterId":10,"dungeonRunId":"run_uuid"}` | `{"ok":true,"data":{"tasks":[{"id":301,"slotIndex":1,"templateId":"kill_slimes","type":"combat","difficulty":"normal","requiredQuantity":10,"progressQuantity":10,"status":"COMPLETED","rewardItemId":"bounty_badge_common","rewardQuantity":1}]}}` |
| POST `/api/economy/bounty/submit-equipment` | Header：`X-Account-Id: 1`；Body：`{"opId":"bounty-submit-01","characterId":10,"slotIndex":1,"equipmentUid":"eq_rare"}` | `{"ok":true,"data":{"id":302,"slotIndex":1,"templateId":"submit_rare_equipment","type":"equipment","difficulty":"rare","minRarity":3,"requiredQuantity":1,"progressQuantity":1,"status":"COMPLETED","rewardItemId":"bounty_badge_rare","rewardQuantity":1}}` |
| POST `/api/economy/bounty/claim` | Header：`X-Account-Id: 1`；Body：`{"opId":"bounty-claim-01","characterId":10,"slotIndex":1}` | `{"ok":true,"data":{"id":301,"slotIndex":1,"templateId":"kill_slimes","type":"combat","difficulty":"normal","requiredQuantity":10,"progressQuantity":10,"status":"CLAIMED","rewardItemId":"bounty_badge_common","rewardQuantity":1}}` |
| POST `/api/economy/bounty/badges/draw` | Header：`X-Account-Id: 1`；Body：`{"opId":"bounty-badge-draw-01","characterId":10,"badge":"common"}` | `{"ok":true,"data":{"rewardType":"item","itemId":"iron_ore","amount":5}}` 或 `{"ok":true,"data":{"rewardType":"locked_aeb","amount":25,"unlockAt":"2026-07-15T10:00:00Z"}}` |
| POST `/api/economy/nft/mint/request` | Header：`X-Account-Id: 1`；Body：`{"opId":"mint-request-01","characterId":10,"equipmentUid":"eq_abc"}` | `{"ok":true,"data":{"request":{"id":1001,"accountId":1,"nftAssetId":2001,"sourceAssetType":"EQUIPMENT","sourceAssetId":501,"mintFeeToken":500,"status":"PENDING","createdAt":"2026-07-14T10:00:00Z","equipmentUid":"eq_abc"},"asset":{"id":2001,"accountId":1,"sourceAssetType":"EQUIPMENT","sourceAssetId":501,"status":"MINT_REQUESTED","createdAt":"2026-07-14T10:00:00Z","equipmentUid":"eq_abc"},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/nft/mint/cancel` | Header：`X-Account-Id: 1`；Body：`{"opId":"mint-cancel-01","requestId":1001}` | `{"ok":true,"data":{"request":{"id":1001,"accountId":1,"nftAssetId":2001,"sourceAssetType":"EQUIPMENT","sourceAssetId":501,"mintFeeToken":500,"status":"CANCELLED","createdAt":"2026-07-14T10:00:00Z","equipmentUid":"eq_abc"},"asset":{"id":2001,"accountId":1,"sourceAssetType":"EQUIPMENT","sourceAssetId":501,"status":"CANCELLED","createdAt":"2026-07-14T10:00:00Z","equipmentUid":"eq_abc"},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/internal/nft/mint/confirm` | Service Identity：`economy.mint`；Body：`{"opId":"mint-confirm-01","requestId":1001,"mintAddress":"CoreAssetAddress","txSignature":"solana_tx_sig","metadataUri":"https://metadata.example/eq_abc.json"}` | Stub 模式：`{"ok":true,"data":{"request":{"id":1001,"status":"CONFIRMED","txSignature":"solana_tx_sig"},"asset":{"id":2001,"mintAddress":"CoreAssetAddress","metadataUri":"https://metadata.example/eq_abc.json","status":"MINTED","mintedAt":"2026-07-14T10:02:00Z"},"snapshot":{...EconomySnapshot}}}`；生产未接适配器时返回 `503/code=3903` |
| GET `/api/economy/nft/assets` | Header：`X-Account-Id: 1` | `{"ok":true,"data":{"items":[{"id":2001,"accountId":1,"sourceAssetType":"EQUIPMENT","sourceAssetId":501,"mintAddress":"CoreAssetAddress","metadataUri":"https://metadata.example/eq_abc.json","status":"MINTED","createdAt":"2026-07-14T10:00:00Z","mintedAt":"2026-07-14T10:02:00Z","equipmentUid":"eq_abc"}]}}` |
| POST `/api/economy/dungeon/enter` | Header：`X-Account-Id: 1`、`X-Character-Id: 10`；Body：`{"opId":"dungeon-enter-01","chapterId":1,"floorId":3}` | `{"ok":true,"data":{"dungeonRunId":"run_uuid","chapterId":1,"floorId":3,"isBoss":false,"status":"IN_PROGRESS","cost":{"gold":10,"items":[]},"rewards":{"exp":0,"levelsGained":0,"level":3,"expToNextLevel":760,"tokenReward":"","items":[],"equipmentItems":[]},"discardedRewards":{"items":[]},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/dungeon/finish` | Body：`{"opId":"dungeon-finish-01","characterId":10,"dungeonRunId":"run_uuid","chapterId":1,"floorId":3,"result":"victory","exp":120,"kills":[{"enemyId":1,"enemyName":"Slime","quantity":3}],"progress":{"reachedWave":5}}` | `{"ok":true,"data":{"dungeonRunId":"run_uuid","chapterId":1,"floorId":3,"isBoss":false,"status":"REWARDED","result":"victory","cost":{"gold":10,"items":[]},"rewards":{"exp":120,"levelsGained":0,"level":3,"expToNextLevel":640,"tokenReward":"5","items":[{"id":901,"itemId":"iron_ore","quantity":2,"slot":-1,"durability":0,"enhanceLevel":0}],"equipmentItems":[]},"discardedRewards":{"items":[]},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/loot/claim-player` | Body：`{"opId":"loot-claim-01","characterId":10,"lootId":901,"slotIndex":2,"quantity":1}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/loot/claim-all` | Body：`{"opId":"loot-claim-all-01","characterId":10}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/loot/discard` | Body：`{"opId":"loot-discard-01","characterId":10,"lootId":901}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/gathering/settle` | Body：`{"opId":"gather-01","characterId":10,"nodeId":"iron-node"}` | `{"ok":true,"data":{"activityId":"iron-node","activityType":"gathering","rewards":{"exp":0,"levelsGained":0,"level":3,"expToNextLevel":760,"tokenReward":"1","items":[{"id":1101,"itemId":"iron_ore","quantity":3,"slot":5,"durability":0,"enhanceLevel":0}],"equipmentItems":[]},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/farming/harvest` | Body：`{"opId":"harvest-01","characterId":10,"cropId":"wheat"}` | `{"ok":true,"data":{"activityId":"wheat","activityType":"farming","rewards":{"exp":0,"levelsGained":0,"level":3,"expToNextLevel":760,"tokenReward":"","items":[{"id":1102,"itemId":"wheat","quantity":4,"slot":6,"durability":0,"enhanceLevel":0}],"equipmentItems":[]},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/boss/contribute` | Body：`{"opId":"boss-contrib-01","characterId":10,"bossEventId":99,"contribution":50}` | `{"ok":true,"data":{"bossEventId":99,"bossKey":"world-boss-1","contribution":50}}` |
| POST `/api/economy/boss/settle` | Body：`{"opId":"boss-settle-01","characterId":10,"bossEventId":99,"bossKey":"world-boss-1"}` | `{"ok":true,"data":{"bossEventId":99,"bossKey":"world-boss-1","contribution":50,"rewards":{"exp":0,"levelsGained":0,"level":3,"expToNextLevel":760,"tokenReward":"10","items":[],"equipmentItems":[]},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/internal/boss/events/open` | Service Identity：`economy.boss_ops`；Body：`{"opId":"boss-open-01","bossKey":"world-boss-1","startsAt":"2026-07-14T10:00:00Z","endsAt":"2026-07-14T11:00:00Z","metadata":{"region":"ap-east"}}` | `{"ok":true,"data":{"id":99,"bossKey":"world-boss-1","status":"OPEN","startsAt":"2026-07-14T10:00:00Z","endsAt":"2026-07-14T11:00:00Z","createdAt":"2026-07-14T10:00:00Z"}}` |
| POST `/api/economy/internal/boss/events/close` | Body：`{"opId":"boss-close-01","bossEventId":99}` | `{"ok":true,"data":{"id":99,"bossKey":"world-boss-1","status":"CLOSED","startsAt":"2026-07-14T10:00:00Z","endsAt":"2026-07-14T11:00:00Z","createdAt":"2026-07-14T10:00:00Z"}}` |
| POST `/api/economy/internal/boss/events/settle` | Body：`{"opId":"boss-mark-settled-01","bossEventId":99}` | `{"ok":true,"data":{"id":99,"bossKey":"world-boss-1","status":"SETTLED","startsAt":"2026-07-14T10:00:00Z","endsAt":"2026-07-14T11:00:00Z","createdAt":"2026-07-14T10:00:00Z"}}` |
| GET `/api/economy/internal/boss/events/active` | Service Identity：`economy.boss_ops` | `{"ok":true,"data":{"events":[{"id":99,"bossKey":"world-boss-1","status":"OPEN","startsAt":"2026-07-14T10:00:00Z","endsAt":"2026-07-14T11:00:00Z","createdAt":"2026-07-14T10:00:00Z"}]}}` |
| POST `/api/economy/inventory/organize` | Body：`{"opId":"bag-organize-01","characterId":10}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/warehouse/organize` | Body：`{"opId":"warehouse-organize-01","characterId":10}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/inventory/discard` | 普通物品：`{"opId":"discard-01","characterId":10,"slotIndex":3,"quantity":2}`；装备：`{"opId":"discard-eq-01","characterId":10,"equipmentUid":"eq_abc"}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/inventory/synthesize` | Body：`{"opId":"synth-01","characterId":10,"recipeId":"iron-sword","batchCount":1}` | `{"ok":true,"data":{"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/inventory/bag/expand` | Body：`{"opId":"bag-expand-01","characterId":10}` | `{"ok":true,"data":{"order":{...PaymentOrder,"purpose":"BAG_EXPAND"},"bagExpandCount":0,"bagSlots":25}}` |
| POST `/api/economy/license/purchase` | Body：`{"opId":"license-01","characterId":10}` | `{"ok":true,"data":{"order":{...PaymentOrder,"purpose":"TRADING_LICENSE"},"hasLicense":false}}` |
| POST `/api/economy/license/buy` | Body：`{"opId":"license-buy-01","characterId":10}` | 同 `/api/economy/license/purchase` |
| GET `/api/economy/marketplace/listings` | Query：`status=LISTED&assetType=ITEM&itemId=iron_ore&limit=20&offset=0` | `{"ok":true,"data":{"items":[{"id":7001,"sellerAccountId":2,"sellerCharacterId":20,"assetType":"ITEM","assetId":1101,"itemId":"iron_ore","quantity":5,"priceToken":100,"listingDepositToken":1,"feeBps":500,"status":"LISTED","createdAt":"2026-07-14T10:00:00Z","updatedAt":"2026-07-14T10:00:00Z"}],"limit":20,"offset":0}}` |
| GET `/api/economy/marketplace/listings/mine` | Header：`X-Account-Id: 1`；Query：`status=LISTED&limit=50&offset=0` | `{"ok":true,"data":{"items":[{"id":7001,"sellerAccountId":1,"sellerCharacterId":10,"assetType":"ITEM","assetId":1101,"itemId":"iron_ore","quantity":5,"priceToken":100,"listingDepositToken":1,"feeBps":500,"status":"LISTED","createdAt":"2026-07-14T10:00:00Z","updatedAt":"2026-07-14T10:00:00Z"}],"limit":50,"offset":0}}` |
| GET `/api/economy/marketplace/slots` | Header：`X-Account-Id: 1` | `{"ok":true,"data":{"accountId":1,"baseSlots":5,"materialExpandCount":0,"walletExpandCount":0,"capacity":5,"used":1,"available":4}}` |
| POST `/api/economy/marketplace/list` | ITEM：`{"opId":"list-item-01","characterId":10,"assetType":"ITEM","sourceLocation":"BAG","slotIndex":0,"quantity":5,"priceToken":100}`；EQUIPMENT：`{"opId":"list-eq-01","characterId":10,"assetType":"EQUIPMENT","equipmentUid":"eq_abc","priceToken":1000}` | `{"ok":true,"data":{"listing":{"id":7001,"sellerAccountId":1,"sellerCharacterId":10,"assetType":"ITEM","assetId":1101,"itemId":"iron_ore","quantity":5,"priceToken":100,"listingDepositToken":1,"feeBps":500,"status":"LISTED","createdAt":"2026-07-14T10:00:00Z","updatedAt":"2026-07-14T10:00:00Z"},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/marketplace/listings/{listingId}/buy` | Path：`listingId=7001`；Body：`{"opId":"buy-01","characterId":10}` | `{"ok":true,"data":{"listing":{"id":7001,"status":"SOLD","soldAt":"2026-07-14T10:05:00Z"},"order":{"id":"market_order_01","listingId":7001,"buyerAccountId":1,"buyerCharacterId":10,"amountToken":100,"feeToken":5,"burnToken":1,"treasuryToken":3,"rewardsToken":1,"sellerProceedsToken":95,"depositReturnedToken":1,"status":"COMPLETED","createdAt":"2026-07-14T10:05:00Z","completedAt":"2026-07-14T10:05:00Z"},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/marketplace/listings/{listingId}/cancel` | Path：`listingId=7001`；Body：`{"opId":"cancel-listing-01"}` | `{"ok":true,"data":{"listing":{"id":7001,"sellerAccountId":1,"assetType":"ITEM","itemId":"iron_ore","quantity":5,"priceToken":100,"status":"CANCELLED","cancelledAt":"2026-07-14T10:06:00Z"},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/marketplace/slots/expand-material` | Body：`{"opId":"slot-material-01","characterId":10}` | `{"ok":true,"data":{"slots":{"accountId":1,"baseSlots":5,"materialExpandCount":1,"walletExpandCount":0,"capacity":7,"used":1,"available":6},"snapshot":{...EconomySnapshot}}}` |
| POST `/api/economy/marketplace/slots/expand-wallet` | Body：`{"opId":"slot-wallet-create-01","characterId":10}` | `{"ok":true,"data":{"order":{...PaymentOrder,"purpose":"MARKET_SLOT_WALLET_EXPAND"},"slots":{"accountId":1,"baseSlots":5,"materialExpandCount":0,"walletExpandCount":0,"capacity":5,"used":1,"available":4}}}` |
| POST `/api/economy/marketplace/slots/expand-wallet/submit` | Body：`{"opId":"slot-wallet-submit-01","orderId":"pay_00000000-0000-4000-8000-000000000001","txSignature":"solana_tx_sig"}` | `{"ok":true,"data":{...PaymentOrder,"status":"SUBMITTED","txSignature":"solana_tx_sig","submittedAt":"2026-07-14T10:03:00Z"}}` |
| POST `/api/economy/internal/payments/submit` | Service Identity：`economy.payments`；Body：`{"opId":"payment-submit-01","orderId":"pay_00000000-0000-4000-8000-000000000001","txSignature":"solana_tx_sig"}` | `{"ok":true,"data":{...PaymentOrder,"status":"SUBMITTED","txSignature":"solana_tx_sig","submittedAt":"2026-07-14T10:03:00Z"}}` |
| POST `/api/economy/rewards/grant-locked` | Service Identity：`economy.rewards`；Header：`X-Account-Id: 1`；Body：`{"amount":25,"source":"quest","ref":"quest-100","cooldownHours":24}` | `{"ok":true,"data":{"id":9001,"accountId":1,"amount":25,"source":"quest","status":"LOCKED","ref":"quest-100","createdAt":"2026-07-14T10:00:00Z","unlockAt":"2026-07-15T10:00:00Z"}}` |
| POST `/api/chain/token/claim` | Header：`X-Account-Id: 1`；Body：`{"amount":100,"wallet":"<solana-wallet>"}` | `{"ok":true,"data":{"id":8001,"accountId":1,"wallet":"<solana-wallet>","amount":100,"status":"QUEUED","createdAt":"2026-07-14T10:00:00Z"}}` |
| GET `/api/chain/token/ledger` | Header：`X-Account-Id: 1` | `{"ok":true,"data":{"entries":[{"id":1,"accountId":1,"kind":"WITHDRAW_REQUEST","amount":100,"ref":"withdrawal:8001","detail":"queued","createdAt":"2026-07-14T10:00:00Z"}]}}` |
| POST `/api/economy/internal/unlocks/settle` | Service Identity：`economy.worker`；Body：`{}` | `{"ok":true,"data":{"settled":[{"id":9001,"accountId":1,"amount":25,"source":"quest","status":"UNLOCKED","ref":"quest-100","createdAt":"2026-07-14T10:00:00Z","unlockAt":"2026-07-15T10:00:00Z"}]}}` |
| POST `/api/economy/internal/withdrawals/process` | Service Identity：`economy.worker`；Body：`{}` | `{"ok":true,"data":{"processed":[{"id":8001,"accountId":1,"wallet":"<solana-wallet>","amount":100,"status":"PROCESSING","createdAt":"2026-07-14T10:00:00Z","processedAt":"2026-07-14T10:01:00Z"}]}}` |
| POST `/api/economy/internal/chain/deposits/scan` | Service Identity：`economy.worker`；Body：`{}` | `{"ok":true,"data":{"scanned":20,"credited":2,"ignored":18,"paymentsFulfilled":1,"cursorSlot":345678,"signatures":["sig1","sig2"]}}`；未配置链时：`{"ok":true,"data":{"scanned":0,"credited":0,"ignored":0,"paymentsFulfilled":0,"cursorSlot":0,"disabled":true,"message":"solana deposit scan requires rpc, deposit wallet and token mint"}}` |
| POST `/api/economy/internal/chain/payouts/submit` | Service Identity：`economy.worker`；Body：`{}` | `{"ok":true,"data":{"processed":2,"submitted":2,"confirmed":0,"failed":0,"ids":[8001,8002]}}` |
| POST `/api/economy/internal/chain/payouts/confirm` | Service Identity：`economy.worker`；Body：`{}` | `{"ok":true,"data":{"processed":2,"submitted":0,"confirmed":2,"failed":0,"ids":[8001,8002]}}` |
| POST `/api/economy/internal/payments/confirm` | Service Identity：`economy.payments`；Body：`{"orderId":"pay_00000000-0000-4000-8000-000000000001","reason":"chain confirmation"}` | `{"ok":true,"data":{...PaymentOrder,"status":"FULFILLED","txSignature":"solana_tx_sig","submittedAt":"2026-07-14T10:03:00Z","confirmedAt":"2026-07-14T10:04:00Z","fulfilledAt":"2026-07-14T10:04:00Z"}}` |
| POST `/api/economy/internal/equipment/npc-recycle/purge` | Service Identity：`economy.worker`；Body：`{"limit":100}`；无参数也传 `{}` | `{"ok":true,"data":{"purged":12}}` |

## 6. 常见错误

| HTTP | code | 含义 |
| ---: | ---: | --- |
| 400 | 400 | JSON、Query、Path 参数不合法 |
| 400 | 2006 | `accountId` 缺失或非法 |
| 400 | 2007 | `characterId` 缺失或非法 |
| 401 | 401 | JWT 缺失、过期或签名错误 |
| 401 | 4010 | Service Identity、签名或 nonce 非法/已撤销 |
| 403 | 4030 | 缺少 capability 或游戏服 subject 不匹配 |
| 404 | 2008/4204 | 经济快照或在线状态不存在 |
| 409 | 4015 | 副本恢复决策与当前状态冲突 |
| 503 | 3903 | 生产 NFT Core 确认适配器尚未配置 |

调用方应记录 HTTP 状态、`error.code`、`error.message`、`opId` 和 Service Request nonce，但不得记录私钥、Refresh Token 或完整 Bearer Token。
