# Aeonblight 管理员接口文档

版本日期：2026-07-14  
服务：`admin-api`  
本地默认地址：`http://127.0.0.1:8083`  
路由数量：52

本文独立描述管理端接口。玩家、游戏服和 Economy Worker 接口见 [接口列表](interface-list.md) 与 [接口详解](interface-reference.md)。

## 1. 认证与安全约定

除 `/health`、`/ready`、管理员登录挑战和登录提交外，全部接口要求普通管理员或超级管理员认证。

普通管理员先由超级管理员登记 Ed25519 公钥，然后通过签名登录换取短期 JWT。普通管理员请求使用：

```http
Authorization: Bearer <admin-login-jwt>
Content-Type: application/json
```

超级管理员接口（包括全部 `/api/admin/ops/*` 和普通管理员配置接口）使用：

```http
X-Super-Admin-Key: <SUPER_ADMIN_OPS_KEY>
```

development/test 仍保留旧兼容方式：

```http
Authorization: Bearer <ADMIN_TOKEN>
```

注意：

- 管理员 JWT 的认证上下文来自 `admin_users` 当前行；禁用管理员后，未过期 JWT 也会立即被拒绝。
- `ADMIN_SESSION_TTL_MINUTES` 控制管理员 JWT 有效期，默认 30 分钟；过期后客户端应清理登录态并重新签名。
- `SUPER_ADMIN_OPS_KEY` 的认证上下文为 `{"id":"super-admin-ops","role":"SUPER_ADMIN"}`。
- 配置了 `SUPER_ADMIN_OPS_KEY` 后，普通 `ADMIN_TOKEN` 访问超级管理员接口会得到 HTTP `403`；不需要同时提交 `Authorization`。
- 在 development/test 尚未配置 `SUPER_ADMIN_OPS_KEY` 时，`ADMIN_TOKEN` 暂按 Bootstrap Super Admin 兼容旧环境；staging/production 不接受 `ADMIN_TOKEN` 作为普通管理员登录方式，并会拒绝缺失超级管理员密钥的启动配置。
- Service Identity 创建和撤销明确要求 `SUPER_ADMIN`；列表为普通管理员可读设计。
- 许可证人工授予/撤销、提现审核、NFT 人工确认和 Hot Wallet 暂停也要求 `SUPER_ADMIN`；账号风控、市场限制和查询仍可由普通管理员处理。
- 部分旧请求 Body 仍接受 `adminId`，但服务端忽略该值，审计操作者只取认证上下文，不能由请求体伪造。
- JSON Body 最大 `1 MiB`；未知字段、尾随 JSON、空 Body 均拒绝。
- 通用成功/失败信封与非管理接口相同。

成功：

```json
{"ok":true,"data":{}}
```

失败：

```json
{"ok":false,"error":{"code":4001,"message":"..."}}
```

## 2. 健康检查

### GET `/health`

- 认证：公开。
- 仅表示 `admin-api` 进程存活。

### GET `/ready`

- 认证：公开。
- 检查 PostgreSQL 和 `REQUIRED_SCHEMA_VERSION`；Required 检查失败返回 HTTP `503`。

## 3. 管理员签名登录与配置

### GET `/api/admin/auth/nonce`

- 认证：公开。
- Query：`adminId`。
- 成功返回：`adminId`、`nonce`、完整待签 `message`、`expiresAt`。
- `message` 必须原样用对应 Ed25519 私钥签名；nonce 有效期 5 分钟且只能成功消费一次。

### POST `/api/admin/auth/login`

```json
{"adminId":"ops-01","nonce":"admin_nonce_...","signature":"<base58-ed25519-signature>"}
```

- 认证：公开。
- `signature` 必须是上一步 `message` 的 Ed25519 签名。
- 成功返回：`admin`、`accessToken`、`expiresAt`。
- JWT 有效期由 `ADMIN_SESSION_TTL_MINUTES` 控制，默认 30 分钟。
- 错误签名不会消费 nonce；成功登录会消费 nonce，重放返回 HTTP `401`。

### POST `/api/admin/admin-users`

- 权限：`SUPER_ADMIN`。

```json
{
  "adminId":"ops-01",
  "username":"Ops One",
  "role":"OPERATOR",
  "publicKey":"<base58-ed25519-public-key>",
  "reason":"initial operator"
}
```

约束：

- `adminId`：3–64 个小写字母、数字、`.`、`_`、`-`；
- `username`：最多 100 字符，省略时使用 `adminId`；
- `publicKey`：base58 编码的 32-byte Ed25519 公钥；私钥绝不能上传；
- `role`：`OPERATOR`、`FINANCE`、`SUPPORT`、`VIEWER`，省略默认为 `OPERATOR`；
- 该接口不能创建 `SUPER_ADMIN`；超级管理员仍由 `SUPER_ADMIN_OPS_KEY` 代表。

成功：HTTP `201`，状态为 `ACTIVE`，同时写入审计。

### GET `/api/admin/admin-users`

- 权限：`SUPER_ADMIN`。
- Query：`status?=ACTIVE|DISABLED`、`limit?`、`offset?`。
- 成功：`items: AdminUser[]`、`count`。

### DELETE `/api/admin/admin-users/{adminId}`

- 权限：`SUPER_ADMIN`。
- Body：`{"reason":"operator retired"}`。
- 软禁用管理员；后续登录会拒绝，已有未过期 JWT 也会在普通管理接口处被拒绝。

## 4. 账号管理

### GET `/api/admin/accounts/selector`

- 权限：普通管理员。
- Query：`keyword?`、`status?=ACTIVE|BANNED|FROZEN|DELETED`、`limit?`、`offset?`。
- 用于后台账号选择器；`keyword` 会匹配账号 ID、用户名、钱包地址和账号下角色名。
- 成功：`items: AdminAccountSelectorItem[]`、`count`、`limit`、`offset`。
- 每个 item 返回：`accountId`、`username`、`walletAddress`、`status`、`roles`、`createdAt`、`lastLoginAt`；其中 `roles` 是该账号未删除角色名按 slot/id 排序后的英文逗号分隔字符串，例如 `"Knight,Mage"`。

### GET `/api/admin/accounts`

- Query：`accountId` 或 `id`，也可使用 `wallet`；至少提供一种。
- 成功返回：
  - `id`、`username`、`walletAddress`、`status`、`isBanned`；
  - `riskLevel`、`banReason?`；
  - `hasTradingLicense`、`tradingLicenseAt?`；
  - `activeRestrictions`、`createdAt`、`lastLoginAt`。
- 不存在：HTTP `404` / code `404`。

### POST `/api/admin/accounts/ban`

```json
{"accountId":1,"banned":true,"reason":"fraud review","adminId":"ignored"}
```

- `accountId` 必填；也兼容 Query `accountId`。
- `banned=true` 设置账号 `BANNED`、保存原因，并尝试写入 severity `80` 的 `ADMIN_BAN` Risk Event。
- `banned=false` 恢复 `ACTIVE` 并清除封禁原因。
- 成功：`ok=true`、Audit Record。

### POST `/api/admin/accounts/risk-level`

```json
{"accountId":1,"riskLevel":42,"reason":"manual review","adminId":"ignored"}
```

- `accountId` 必填；`riskLevel >= 0`。
- 写入 `ADMIN_RISK_LEVEL` Risk Event 和 Audit Record。
- 成功：`ok`、`riskLevel`、`audit`。

### POST `/api/admin/accounts/license`

```json
{"accountId":1,"granted":true,"reason":"approved","adminId":"ignored"}
```

- 授予或撤销交易许可证。
- 授予时首次写入 `tradingLicenseAt`；撤销时清空。
- 成功：`ok`、`granted`、`audit`。

### POST `/api/admin/accounts/sessions/revoke`

```json
{"accountId":1,"reason":"account secured","adminId":"ignored"}
```

- 原子撤销该账号所有 `ACTIVE` Session；关联 Refresh Token 因 Session 不再是 `ACTIVE` 而无法继续使用。
- API 的敏感 Session 校验以 PostgreSQL 为准，不会被旧 Redis ACTIVE 缓存绕过。
- 成功：`revoked` 数量、`audit`。

## 5. 角色操作台与物品目录

本节接口均为普通管理员可读，用于管理后台角色操作台；不需要 `X-Internal-Key`，也不依赖玩家侧 `account-api` / `economy-api`。

### GET `/api/admin/characters`

Query：

| 字段 | 说明 |
| --- | --- |
| `keyword?` | 模糊搜索角色名、角色 ID、账号 ID、用户名、钱包地址 |
| `accountId?` | 按账号过滤 |
| `wallet?` | 按钱包精确过滤 |
| `minLevel?` / `maxLevel?` | 等级范围；非负，且上限不能小于下限 |
| `hasTradingLicense?` | `true` / `false` |
| `status?` | `ACTIVE`、`BANNED`、`FROZEN`、`DELETED` |
| `onlineOnly?` | 只返回在线角色 |
| `serverId?` | 按 Online Presence 所在服务器过滤 |
| `limit?` / `offset?` | `limit` 默认 50，范围 1-200；`offset` 非负 |

成功返回：`items`、`count`、`limit`、`offset`。每项包含 `characterId`、`accountId`、`name`、`level`、`exp`、`walletAddress`、`accountStatus`、`riskLevel`、`hasTradingLicense`、`lastLoginAt`、`online`、`serverId?`、`createdAt`。

### GET `/api/admin/characters/{characterId}`

Query：

| 字段 | 说明 |
| --- | --- |
| `include?` | 逗号列表；默认 `account,snapshot,online`，也支持 `ledger`、`audits`、`all` |

成功返回：

- `character`：角色基础状态，包含等级、经验、体力、背包格、最高通关进度和创建时间。
- `account`：账号状态、风险等级、交易许可证和限制数量。
- `economy`：只读经济快照，字段复用 `EconomySnapshot`：`inventory`、`warehouse`、`lootTray`、`equipmentItems`、`accountToken` 等。
- `online`：`online`、`serverId?`、`connectionId?`、`sessionId?`、`lastSeenAt?`。

装备项包含 `equipmentUid`、`itemId`、`rarity`、`enhanceLevel`、`durability`、`maxDurability`、`status`、`equipSlot`、`slot`、`weaponType`、`weaponTypeKey`、`affixes`、`nftStatus?`。

### GET `/api/admin/characters/{characterId}/ledger`

Query：`kind?`、`limit?`、`offset?`。

成功返回：`ledger`、`count`、`limit`、`offset`。数据按 `economy_ledger.character_id` 过滤。

### GET `/api/admin/characters/{characterId}/audits`

Query：`limit?`、`offset?`。

成功返回：`audits`、`count`、`limit`、`offset`。数据包含 `target_type='character'` 且 `target_id=characterId` 的审计，也包含同账号相关账号审计，方便角色操作台查看踢人、封禁等关联操作。

### GET `/api/admin/characters/{characterId}/timeline`

Query：

| 字段 | 说明 |
| --- | --- |
| `types?` | 逗号列表或重复参数；支持 `ledger`、`audit`、`risk`，默认全部 |
| `limit?` / `offset?` | `limit` 默认 50，范围 1-200；`offset` 非负 |

成功返回：`items`、`count`、`total`、`limit`、`offset`。每项包含 `type`、`id`、`title`、`detail?`、`amount?`、`ref?`、`severity?`、`createdAt`、`raw?`。数据来源包括该角色 `economy_ledger`、角色/账号管理员审计，以及同账号 `account_risk_events`。

### GET `/api/admin/equipment/{equipmentUid}`

按装备 UID 查询装备归属、当前位置、NFT 和市场状态。成功返回：

- `equipment`：`EquipmentItem` 字段，包括 `equipmentUid`、`itemId`、`rarity`、`enhanceLevel`、`durability`、`maxDurability`、`status`、`equipSlot`、`slot`、`weaponType`、`weaponTypeKey`、`affixes`、`nftStatus?`。
- `owner`：`accountId`、`characterId?`、`characterName?`、`walletAddress`、`accountStatus`。
- `nft?`：最近 NFT Asset / Mint Request 状态，包含 `assetId?`、`status?`、`mintAddress?`、`metadataUri?`、`requestId?`、`requestStatus?`、时间戳等。
- `marketplace?`：该装备最近一条 Marketplace Listing，包含价格、状态、卖方和创建/成交/取消时间。

### GET `/api/admin/catalog/items`

Query：

| 字段 | 说明 |
| --- | --- |
| `keyword?` | 搜索 `itemId`、显示名、分类、稀有度、显示类型 |
| `category?` | 分类过滤 |
| `isEquipment?` | `true` / `false` |
| `rarity?` | 稀有度过滤 |
| `limit?` / `offset?` | 非 grouped 列表分页；`limit` 默认 200，范围 1-500；`offset` 非负 |
| `grouped?` | `true` 时按分类返回完整 `groups`，不按 `limit`/`offset` 截断 |

成功返回当前生效 Economy 配置生成的目录：`configVersion`、`items` 或 `groups`、`categories`、`count`、`total`、`limit`、`offset`。

目录项包含 `itemId`、`displayName`、`category`、`categoryLabel`、`rarity`、`rarityLabel`、`isEquipment`、`equipmentSlot`、`equipmentSlotLabel`、`weaponType?`、`weaponTypeKey?`、`seriesId?`、`stage?`、`displayType?`、`stackable`、`sellPrice`、`maxGrantQuantity`、`enabledForAdminGrant`。装备模板会按当前配置的装备品质展开为多条 `itemId + rarity` 可选项，并按 `sellPriceGoldByStage` 带出卖出价；装备类 `maxGrantQuantity=1`，用于和发奖/补偿接口的装备数量校验保持一致。

## 6. Marketplace 限制

限制类型会被 Marketplace list/buy 流程执行。

### GET `/api/admin/market/restrictions`

Query：

| 字段 | 说明 |
| --- | --- |
| `accountId?` | 正整数；省略表示全部账号 |
| `activeOnly?` | `true`/`false`；默认 false |
| `limit?` | 默认 50，范围 1–200 |
| `offset?` | 非负整数，默认 0 |

成功：`restrictions: MarketRestriction[]`。

MarketRestriction 字段：`id`、`accountId`、`restrictionType`、`reason?`、`createdBy?`、`createdAt`、`expiresAt?`、`revokedAt?`。

### POST `/api/admin/market/restrictions`

```json
{
  "accountId": 1,
  "restrictionType": "ALL",
  "reason": "manual hold",
  "expiresAt": "2026-07-20T00:00:00Z",
  "adminId": "ignored"
}
```

- `restrictionType`：`BUY`、`SELL`、`ALL`。
- `expiresAt` 可省略；提供时必须为 RFC3339。
- 同时尝试写入 severity `50` 的 `MARKET_RESTRICTION` Risk Event。
- 成功：`restriction`、`audit`。

### POST `/api/admin/market/restrictions/revoke`

```json
{"id":100,"reason":"review complete","adminId":"ignored"}
```

- 只能撤销尚未撤销的记录。
- 撤销原因追加到现有原因。
- 成功：`restriction`、`audit`。

## 7. Risk Event 与审计

### GET `/api/admin/risk/events`

- Query：`accountId?`、`limit?`（1–200，默认 50）、`offset?`。
- 成功：`events: RiskEvent[]`。

RiskEvent 字段：`id`、`accountId?`、`eventType`、`severity`、`deviceId?`、`ipAddress?`、`wallet?`、`detail?`、`createdAt`。

### POST `/api/admin/risk/events`

```json
{
  "accountId": 1,
  "eventType": "MANUAL_REVIEW",
  "severity": 50,
  "deviceId": "device-01",
  "ipAddress": "203.0.113.10",
  "wallet": "<solana-wallet>",
  "detail": {"source":"support"},
  "reason": "manual observation",
  "adminId": "ignored"
}
```

- `eventType` 必填；`severity >= 0`。
- `accountId` 可为 `0`，表示非账号特定事件。
- `ipAddress` 提供时必须是可存储的 IP 格式。
- 成功：`event`、`audit`。

### GET `/api/admin/audits`

- Query：`limit?`（1–200，默认 50）、`offset?`。
- 成功：`audits: AuditEntry[]`。
- AuditEntry：`id`、`adminId`、`action`、`target`、`reason`、`createdAt`。

### GET `/api/admin/ledger`

- Query：`accountId`。
- 成功：`ledger: LedgerEntry[]`。
- 当前非法/缺失 `accountId` 会解析为 `0` 并通常返回空结果；调用方应始终传正整数。

## 8. 提现、支付与 NFT 队列

### GET `/api/admin/withdrawals`

- Query：`status?`。
- 成功：`withdrawals: Withdrawal[]`。
- 常见状态：`QUEUED`、`MANUAL_REVIEW`、`REJECTED`、`PAYOUT_CREATED`、`SUBMITTED`、`CONFIRMED`、`FAILED`、`CANCELLED`。

### POST `/api/admin/withdrawals/review`

```json
{"id":1001,"approve":true,"reason":"KYC passed","adminId":"ignored"}
```

- 用于处理 `MANUAL_REVIEW` 提现。
- `approve=true` 将状态改为 `QUEUED`，进入后续处理；`false` 改为 `REJECTED` 并退回可提现余额。
- 成功：Withdrawal。

### GET `/api/admin/payments`

- Query：`accountId?`、`status?`、`limit?`（1–200）、`offset?`。
- 成功：`payments: PaymentOrder[]`。

### GET `/api/admin/nft/requests`

- Query：`accountId?`、`status?`、`limit?`（1–200）、`offset?`。
- 成功：`requests: NFTMintRequest[]`。

### POST `/api/admin/nft/mint/confirm`

```json
{
  "opId":"admin-mint-confirm-01",
  "requestId":1001,
  "mintAddress":"<asset-address>",
  "txSignature":"<solana-signature>",
  "metadataUri":"https://...",
  "reason":"operator confirmation",
  "adminId":"ignored"
}
```

- `opId` 省略时服务端生成，但建议调用方显式提供稳定幂等键。
- `STUB_MODE=disabled` 当前直接返回 HTTP `503` / code `4002`，不会在生产制造假 NFT。
- Development/test Stub 模式成功返回 `result` 和 `audit`。

## 9. Hot Wallet

### GET `/api/admin/hot-wallet`

- Query：`wallet?`；省略时使用 `SOLANA_PAYOUT_WALLET`。
- 已存在：`status`、`exists=true`。
- 未存在：返回 `wallet`、`network`、`payoutsPaused=false`、`exists=false`，HTTP 仍为 `200`。

HotWalletStatus：`wallet`、`network`、`tokenMint?`、`balance`、`lowBalanceThreshold`、`payoutsPaused`、`lastCheckedAt?`、`updatedAt`。

### POST `/api/admin/hot-wallet/pause`

```json
{"wallet":"<hot-wallet>","paused":true,"reason":"low SOL","adminId":"ignored"}
```

- `wallet` 省略时使用配置的 Payout Wallet。
- `paused=true` 阻止后续 Payout Submission；false 恢复。
- 成功：`status`、`audit`。

## 10. Service Identity

### GET `/api/admin/service-identities`

- 权限：管理员可读。
- Query：
  - `status?`：`ACTIVE` 或 `DISABLED`；
  - `limit?`：1–200，默认 50；
  - `offset?`：非负整数。
- 成功：`items: ServiceIdentity[]`、`count`。

ServiceIdentity 字段：`serviceId`、`name`、`kind`、`subjectId?`、`publicKey`、`capabilities`、`status`、创建/撤销审计字段。

### POST `/api/admin/service-identities`

- 权限：`SUPER_ADMIN`。

```json
{
  "serviceId":"game-server-shanghai-01",
  "name":"Shanghai Game Server 01",
  "kind":"GAME_SERVER",
  "subjectId":"shanghai-01",
  "publicKey":"<base58-ed25519-public-key>",
  "capabilities":["account.gameplay","economy.gameplay"],
  "reason":"initial production registration"
}
```

约束：

- `serviceId`：3–64 个小写字母、数字、`.`、`_`、`-`；
- `name`：1–100 Unicode 字符；`reason`：1–500 字符；
- `publicKey`：base58 编码的 32-byte Ed25519 公钥；私钥绝不能上传；
- `GAME_SERVER` 必须有 `subjectId`，且同一 subject 只能有一个 ACTIVE 身份；
- 公钥全局唯一。

Kind 与 Capability 白名单：

| Kind | 可授予 Capability |
| --- | --- |
| `GAME_SERVER` | `account.gameplay`、`economy.gameplay` |
| `WORKER` | `economy.worker` |
| `CHAIN_OPERATOR` | `economy.payments` |
| `MINT_OPERATOR` | `economy.mint` |
| `OPS` | `account.ops`、`economy.boss_ops`、`economy.rewards` |

成功：HTTP `201`，状态为 `ACTIVE`，同时写入审计。

### DELETE `/api/admin/service-identities/{serviceId}`

- 权限：`SUPER_ADMIN`。
- Body：`{"reason":"server retired"}`。
- 是软撤销，不物理删除；状态改为 `DISABLED`，后续签名请求立即拒绝。
- 重复撤销幂等，不覆盖首次撤销人、原因和时间。
- 成功：更新后的 ServiceIdentity。

## 11. 公告管理

公告分为两类：

- `OPS_NOTICE`：活动开启/关闭、维护、版本更新等运营通知。管理员可以创建、修改和撤销。
- `RARE_REWARD`：超稀有装备、坐骑等炫耀公告。只能由真实发奖事务自动生成，管理员不能手工创建，但可以修改模板和撤销已生成公告。

### GET `/api/admin/announcements`

- 权限：管理员可读。
- Query：
  - `kind?`：`OPS_NOTICE` 或 `RARE_REWARD`；
  - `status?`：`ACTIVE` 或 `REVOKED`；
  - `limit?`：1–200，默认 50；
  - `offset?`：非负整数。
- 成功：`items: Announcement[]`、`count`、`limit`、`offset`。

`Announcement` 字段：`id`、`kind`、`status`、`displayMode`、`title`、`body`、`priority`、`scope`、`metadata?`、`startsAt`、`endsAt?`、`createdAt`、`createdBy?`、`revokedAt?`、`revokedBy?`、`revokeReason?`。

### GET `/api/admin/announcements/templates`

- 权限：管理员可读。
- Query：`kind?`。
- 默认模板：`rare_equipment`、`rare_mount`、`ops_notice`。
- 模板用于控制自动稀有公告和默认运营通知的标题、正文、展示方式、优先级和持续时间。

### PUT `/api/admin/announcements/templates/{code}`

```json
{
  "titleTemplate":"{characterName} 获得了 {rarity} 星装备",
  "bodyTemplate":"通过 {source} 获得 {itemName}",
  "displayMode":"POPUP",
  "priority":900,
  "durationSeconds":300,
  "enabled":true,
  "reason":"tighten rare equipment copy"
}
```

- 权限：管理员可写。
- `code` 必须是现有模板编码；当前支持 `rare_equipment`、`rare_mount`、`ops_notice`。
- `displayMode`：`POPUP` 或 `BANNER`；稀有奖励公告通常使用 `POPUP`。
- 自动稀有公告模板可使用占位符：`{characterName}`、`{itemName}`、`{rarity}`、`{source}`、`{quantity}`。
- 成功返回更新后的 `template`，并写入管理员审计。

### POST `/api/admin/announcements/notices`

```json
{
  "title":"限时活动开启",
  "body":"世界 Boss 活动将于 20:00 开启",
  "displayMode":"BANNER",
  "priority":700,
  "startsAt":"2026-07-15T12:00:00Z",
  "endsAt":"2026-07-15T16:00:00Z",
  "reason":"publish event notice"
}
```

- 权限：管理员可写。
- 创建 `OPS_NOTICE`。不传 `startsAt` 时立即生效；不传 `endsAt` 时一直有效，直到管理员撤销。
- 成功 HTTP `201`，返回 `announcement` 并写入审计。
- 当前生效的运营通知会出现在公开 `GET /api/announcements/active`，也会出现在游戏服 `GET /api/economy/announcements/active`。

### PUT `/api/admin/announcements/notices/{announcementId}`

- 权限：管理员可写。
- Body 与创建接口相同。
- 只能修改 `OPS_NOTICE`；`RARE_REWARD` 不能通过该接口改写内容。
- 成功返回更新后的 `announcement` 并写入审计。

### POST `/api/admin/announcements/{announcementId}/revoke`

```json
{
  "reason":"notice expired early"
}
```

- 权限：管理员可写。
- 可撤销任意公告，包括自动生成的 `RARE_REWARD`。
- 撤销是软删除：公告状态变为 `REVOKED`，不会再出现在 active 公告流。
- 成功返回撤销后的 `announcement` 并写入审计。

## 12. Super Admin Ops

本节所有接口均要求 `X-Super-Admin-Key: <SUPER_ADMIN_OPS_KEY>`。所有写接口（包括 dry run/preview）都要求非空 `opId` 和 `reason`：dry run 不改变玩家资产，但仍会保存预览、管理员审计和可回放的操作结果。`opId` 是全局幂等键，一旦接受即绑定当前超级管理员和操作类型；重试返回首次成功响应，不会重复创建预览或重复发奖。

| Method | Path | 用途 |
| --- | --- | --- |
| `GET` | `/api/admin/ops/servers?status=&region=` | 查询服务器配置、容量和公开状态 |
| `GET` | `/api/admin/ops/servers/online?region=` | 只返回 `ONLINE` 且 30 秒内有心跳的服务器 |
| `GET` | `/api/admin/ops/servers/{serverId}` | 服务器和当前在线玩家 |
| `PUT` | `/api/admin/ops/servers/{serverId}` | 创建或覆盖服务器注册配置 |
| `POST` | `/api/admin/ops/servers/{serverId}/status` | 设置服务器状态 |
| `GET` | `/api/admin/ops/servers/online-players?serverId=` | 查询在线玩家 |
| `POST` | `/api/admin/ops/servers/online-players/{accountId}/kick` | 清除在线态，并可撤销登录 Session |
| `POST` | `/api/admin/ops/characters/{characterId}/grants/rewards` | 事务性发放 Gold、AEB、物品或装备 |
| `POST` | `/api/admin/ops/characters/{characterId}/lottery/draw` | 生成并保存管理员抽奖预览（仅允许 dry run） |
| `POST` | `/api/admin/ops/characters/{characterId}/lottery/commit-preview` | 提交并发放持久化预览 |
| `POST` | `/api/admin/ops/compensation/preview` | 冻结符合联合条件的补偿目标 |
| `GET` | `/api/admin/ops/compensation/previews/{previewId}` | 重新读取补偿预览和奖励明细 |
| `GET` | `/api/admin/ops/compensation/previews/{previewId}/targets?limit=&offset=&keyword=` | 分页读取补偿预览冻结目标 |
| `POST` | `/api/admin/ops/compensation/commit` | 以 `previewId` 单事务提交全量补偿 |
| `GET` | `/api/admin/ops/payments/economy-orders/{orderId}/trace` | 查询订单与经济账本追踪 |
| `POST` | `/api/admin/ops/payments/economy-orders/{orderId}/recover` | 恢复已有验证收据但未完成的订单 |

服务器状态：`STARTING`、`ONLINE`、`DRAINING`、`OFFLINE`、`MAINTENANCE`、`DISABLED`。公开状态仅在 `ONLINE` 且心跳新鲜时为 `online` 或 `full`。

创建/覆盖示例：

```json
{
  "opId":"ops-server-upsert-001",
  "reason":"register Asia 1",
  "host":"game.example.com",
  "port":7777,
  "maxPlayers":100,
  "status":"MAINTENANCE",
  "region":"asia",
  "name":"Asia 1"
}
```

服务器查询响应的 `items`/`server` 是 `opsServerView`：`serverId`、`name`、`host`、`port`、`region?`、`publicEndpoint?`、`maxPlayers`、`curPlayers`、`capacityLimit`、`hasSlot`、`status`、`publicStatus`、`live`、`lastPing?`。`status` 是小写的运维状态；`publicStatus` 为 `online`、`full` 或 `offline`。`live=true` 表示最近 30 秒内收到心跳。

状态更新和踢人分别使用 `{ "opId", "reason", "status" }` 与 `{ "opId", "reason", "revokeSession" }`。踢人只清除本系统的在线态；`revokeSession=true` 时还会撤销当前 Session。它不会向游戏服发送远程断线命令，下一次鉴权、重连或心跳会体现 Session 状态。

### 12.1 单角色奖励

#### POST `/api/admin/ops/characters/{characterId}/grants/rewards`

```json
{
  "opId":"ops-reward-001",
  "reason":"support compensation",
  "gold":10000,
  "withdrawableAeb":500,
  "lockedAeb":100,
  "items":[
    {"itemId":"wood","quantity":10},
    {"itemId":"aeonblight_sword_t30","quantity":1,"rarity":5}
  ],
  "announceRare":true,
  "announcementSource":"活动奖励"
}
```

- `characterId`、`opId`、`reason` 必填；至少提供一项正奖励，金额不得为负。
- `itemId` 必须存在于当前 Economy 配置；普通物品数量必须为正，装备数量只能为 `1`；装备可额外传 `rarity` 指定星级。
- Gold 写入角色钱包；AEB 写入所属账号的可提现或锁定余额；物品与装备进入 Loot Tray，不占用背包格。
- 直接发奖默认不生成稀有奖励公告；如果 `announceRare=true`，必须同时提供 `announcementSource`，用于公告中的奖励途径。
- 成功返回 `snapshot`（角色及账号经济快照）、`items`（本次写入 Loot Tray 的物品、装备）和 `announcements`（若本次生成稀有公告）。Gold、AEB、Loot Tray、稀有公告、`economy_ledger`、审计和幂等结果在同一数据库事务中完成。

### 12.2 管理员抽奖：Dry Run → Preview → Commit

#### POST `/api/admin/ops/characters/{characterId}/lottery/draw`

```json
{
  "opId":"ops-lottery-preview-001",
  "reason":"review live-event reward",
  "count":3,
  "dryRun":true
}
```

- `count` 必须在当前 Economy 配置的 `1..lottery.maxCount` 区间内。
- `dryRun` 必须为 `true`；传 `false` 会被拒绝，不能通过该接口直接发奖。
- 成功返回 `dryRun`、`preview` 和 `audit`。`preview` 包含 `previewId`、`characterId`、`rewards` 和 `expiresAt`；奖励计划已落库并绑定当前超级管理员和角色，有效期 30 分钟。
- 本管理员抽奖为免费运营发奖：不扣 AEB，也不创建支付订单；dry run 只生成预览，不生成公告。

#### POST `/api/admin/ops/characters/{characterId}/lottery/commit-preview`

```json
{
  "previewId":"preview_...",
  "opId":"ops-lottery-commit-001",
  "reason":"live-event reward approved"
}
```

- `previewId` 必须尚未过期、仍为 `PENDING`，且属于当前超级管理员及 URL 中的 `characterId`。
- 只会发放 preview 中保存的 `rewards`，不会重新随机；成功后预览变为 `COMMITTED`。
- 如果 preview 中包含超稀有装备或坐骑，提交发放成功后会自动生成 `RARE_REWARD` 公告，公告途径为 `管理员代抽`。
- 成功返回与单角色奖励相同的 `snapshot`、`items` 和 `announcements`。若提交因任何业务或数据库错误失败，事务会回滚，预览仍可在有效期内重试。

### 12.3 批量全服补偿

#### POST `/api/admin/ops/compensation/preview`

```json
{
  "opId": "ops-compensation-preview-001",
  "reason": "review maintenance compensation",
  "filters": {
    "minLevel": 20,
    "maxLevel": 60,
    "lastLoginFrom": "2026-07-01T00:00:00Z",
    "lastLoginTo": "2026-07-14T00:00:00Z",
    "minClearedChapter": 3,
    "minClearedFloor": 10,
    "minDungeonClearCount": 25,
    "hasTradingLicense": true
  },
  "gold": 10000,
  "withdrawableAeb": 500,
  "lockedAeb": 100,
  "items": [{"itemId":"wood","quantity":10}]
}
```

筛选字段均为可选，多个字段以 **AND** 联合：

| 字段 | 说明 |
| --- | --- |
| `minLevel` / `maxLevel` | 角色等级下限/上限；非负，且上限不能小于下限 |
| `lastLoginFrom` / `lastLoginTo` | 账号最近登录时间闭区间，RFC3339 格式 |
| `minClearedChapter` / `minClearedFloor` | 最高通关进度的章节/层数下限，按 `(chapter, floor)` 字典序比较 |
| `minDungeonClearCount` | 累计胜利通关次数下限 |
| `hasTradingLicense` | 是否持有交易许可证 |

- 默认只选择账号状态为 `ACTIVE` 且角色未删除的目标；封禁、冻结等非 `ACTIVE` 账号不会命中。
- 副本胜利会同步更新角色的最高通关章节、最高通关层数、累计胜利次数和最近胜利时间；补偿筛选使用前三项。
- 奖励字段与单角色奖励相同，至少一项奖励必填，物品仍受当前 Economy 配置校验。
- 成功返回 `preview` 和 `audit`；`preview` 包含 `previewId`、`targetCount`、`expiresAt`、`filters`、`rewards` 和 `items`。`items` 最多展示前 50 个目标；`targetCount` 是完整冻结名单的数量。
- 没有命中目标时返回 `targetCount: 0`、空 `items`，且不会生成可提交的 `previewId`。

#### GET `/api/admin/ops/compensation/previews/{previewId}`

- 仅创建该预览的超级管理员可读。
- 成功返回 `preview`：`previewId`、`status`、`targetCount`、`expiresAt`、`filters`、`rewards`、`createdAt`、`committedAt?`。
- `rewards.items` 会按当前 Economy 配置回显 `displayName`、`category`、`rarity`、`quantity`、`isEquipment`，用于提交前复核。

#### GET `/api/admin/ops/compensation/previews/{previewId}/targets`

Query：`limit?`、`offset?`、`keyword?`；`limit` 默认 50，范围 1-200。

成功返回：`items`、`count`、`total`、`limit`、`offset`。每个目标包含 `accountId`、`characterId`、`name`、`level`、`lastLoginAt`。`keyword` 可按角色名、角色 ID、账号 ID、钱包地址过滤冻结目标。

#### POST `/api/admin/ops/compensation/commit`

```json
{
  "previewId":"preview_...",
  "opId":"ops-compensation-commit-001",
  "reason":"maintenance compensation approved"
}
```

- 仅创建该预览的超级管理员可提交；预览必须为未过期的 `PENDING` 状态。
- 提交严格使用预览创建时已冻结的目标名单和奖励计划，不会重新运行筛选条件。
- 系统按每 250 名角色一个内部批次执行，但全量补偿仍处于一个数据库事务；任何角色发奖失败都会回滚所有目标，预览保持可在有效期内重试。
- 成功返回 `previewId` 和完整 `processed` 数量，并将预览设为 `COMMITTED`。

### 12.4 支付订单追踪与补发

#### GET `/api/admin/ops/payments/economy-orders/{orderId}/trace`

- `orderId` 为支付订单 UUID。
- 成功返回 `order` 和 `ledger`。`order` 包含账号/角色、用途、支付资产、金额、收款钱包、订单状态、链上签名及创建/提交/确认/履约时间；`ledger` 是所有 `ref` 为该订单 ID 的经济账本条目。
- 不存在返回 HTTP `404`。

#### POST `/api/admin/ops/payments/economy-orders/{orderId}/recover`

```json
{
  "opId":"ops-payment-recover-001",
  "reason":"verified receipt, fulfillment retried"
}
```

- 此接口只复用原有支付确认与履约流程，绝不接受、写入或替换 `txSignature`。
- 订单必须是 `SUBMITTED` 或 `CONFIRMED`，并已保存非空链上签名和提交时间；随后系统确认并履约原订单用途。
- `FULFILLED` 订单明确拒绝再次补发。若需要额外人工补偿，应使用单角色奖励或批量补偿接口。
- 成功返回 `order` 和 `audit`；`opId` 重试返回首次成功结果，不会重复履约。


## 13. 通用状态码

| HTTP | code | 说明 |
| ---: | ---: | --- |
| 400 | 400 | JSON、Query 或枚举格式错误 |
| 400 | 4001 | 存储层业务约束失败 |
| 401 | 401 | 管理员认证缺失或错误 |
| 403 | 403 | 需要 SUPER_ADMIN |
| 404 | 404 | 目标不存在 |
| 503 | 4002 | 生产 NFT Core 适配器未配置 |

## 14. 调用示例

```bash
curl -sS \
  -H "Authorization: Bearer $ADMIN_JWT" \
  "http://127.0.0.1:8083/api/admin/accounts?accountId=1"
```

```bash
curl -sS -X POST \
  -H "Authorization: Bearer $ADMIN_JWT" \
  -H "Content-Type: application/json" \
  -d '{"accountId":1,"restrictionType":"ALL","reason":"manual hold"}' \
  http://127.0.0.1:8083/api/admin/market/restrictions
```
