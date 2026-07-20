# Admin API

> 当前、完整的管理员接口文档见
> [`docs/api/admin-interface-reference.md`](../api/admin-interface-reference.md)。本页仅保留内部速查信息。

`admin-api` (default `:8083`) is the privileged ops surface. Ordinary-admin
routes use a short-lived signed-login JWT:

```http
Authorization: Bearer <admin-login-jwt>
```

Super administrators first create an ordinary admin with an Ed25519 public key:

```http
POST /api/admin/admin-users
X-Super-Admin-Key: <SUPER_ADMIN_OPS_KEY>
```

The ordinary admin then calls `GET /api/admin/auth/nonce?adminId=...`, signs the
returned message, and posts it to `POST /api/admin/auth/login`. The returned JWT
uses `ADMIN_SESSION_TTL_MINUTES` (default 30). Expiry or disablement causes later
requests to return `401`, so clients should clear local admin state and request a
new signature.

`ADMIN_TOKEN=dev-admin-token` remains only a development/test compatibility
fallback.

Super-admin operations use a separate header:

```http
X-Super-Admin-Key: <SUPER_ADMIN_OPS_KEY>
```

With `SUPER_ADMIN_OPS_KEY` configured, an `ADMIN_TOKEN` cannot call
super-admin-only operations. The local development compatibility fallback is
documented in the public admin API reference.

On-chain NFT mint signing remains stubbed. Development and test may confirm with
the stub / operator-supplied mint address; staging and production fail closed
until the Core adapter is configured.

## Admin users

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/api/admin/auth/nonce?adminId=` | Public challenge for configured admin public key |
| `POST` | `/api/admin/auth/login` | `{ adminId, nonce, signature }` returns short-lived JWT |
| `GET` | `/api/admin/admin-users?status=` | Super-admin list |
| `POST` | `/api/admin/admin-users` | Super-admin create ordinary admin public key |
| `DELETE` | `/api/admin/admin-users/{adminId}` | Super-admin soft-disable |

## Account

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/api/admin/accounts/selector?keyword=&status=` | Selector list: account option with comma-separated character names in `roles` |
| `GET` | `/api/admin/accounts?accountId=` or `?wallet=` | Detail: status, risk, ban reason, license, active market restrictions |
| `POST` | `/api/admin/accounts/ban` | `{ accountId, banned, reason, adminId }` — writes `ban_reason` + risk event |
| `POST` | `/api/admin/accounts/risk-level` | `{ accountId, riskLevel, reason }` |
| `POST` | `/api/admin/accounts/license` | `{ accountId, granted, reason }` — grant/revoke trading license |
| `POST` | `/api/admin/accounts/sessions/revoke` | Force-revoke all ACTIVE sessions |

## Characters and catalog

Ordinary administrators can use these read-only routes for the admin character
console. They do not require player-side Service Identity headers.

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/api/admin/characters?keyword=&accountId=&wallet=&serverId=` | Paginated character search with account status, license and online server |
| `GET` | `/api/admin/characters/{characterId}?include=account,snapshot,online` | Character detail plus read-only Economy Snapshot fields |
| `GET` | `/api/admin/characters/{characterId}/ledger?kind=` | Character-filtered economy ledger |
| `GET` | `/api/admin/characters/{characterId}/audits` | Character/account-related admin audit rows |
| `GET` | `/api/admin/characters/{characterId}/timeline?types=ledger,audit,risk` | Merged ledger/audit/risk timeline |
| `GET` | `/api/admin/equipment/{equipmentUid}` | Equipment owner, NFT and marketplace status by UID |
| `GET` | `/api/admin/catalog/items?grouped=true` | Current Economy config item picker catalog; template equipment is listed per rarity and equipment grant quantity is `1` |

## Market restrictions

Enforced by marketplace list/buy (`BUY` / `SELL` / `ALL`).

| Method | Path |
| --- | --- |
| `GET` | `/api/admin/market/restrictions?accountId=&activeOnly=true` |
| `POST` | `/api/admin/market/restrictions` — `{ accountId, restrictionType, reason, expiresAt? }` |
| `POST` | `/api/admin/market/restrictions/revoke` — `{ id, reason }` |

## Risk & audit

| Method | Path |
| --- | --- |
| `GET` | `/api/admin/risk/events?accountId=` |
| `POST` | `/api/admin/risk/events` — `{ accountId, eventType, severity, deviceId?, ipAddress?, wallet?, detail? }` |
| `GET` | `/api/admin/audits` |
| `GET` | `/api/admin/ledger?accountId=` |

## Announcements

Rare reward announcements are generated only by trusted reward settlement
paths. Admins can update the format templates and revoke generated rows, but
cannot manually create rare reward announcements.

| Method | Path |
| --- | --- |
| `GET` | `/api/admin/announcements?kind=&status=&limit=&offset=` |
| `GET` | `/api/admin/announcements/templates?kind=` |
| `PUT` | `/api/admin/announcements/templates/{code}` — `{ titleTemplate, bodyTemplate, displayMode, priority, durationSeconds, enabled?, reason }` |
| `POST` | `/api/admin/announcements/notices` — `{ title, body, displayMode?, priority?, startsAt?, endsAt?, reason }` |
| `PUT` | `/api/admin/announcements/notices/{announcementId}` — replaces an ops notice |
| `POST` | `/api/admin/announcements/{announcementId}/revoke` — `{ reason }` |

Default rare templates are `rare_equipment` and `rare_mount`; supported
placeholders are `{characterName}`, `{source}`, `{itemName}`, `{itemId}`,
`{rarity}`, and `{equipmentUid}`.

## Withdrawals / payments / NFT

| Method | Path |
| --- | --- |
| `GET` | `/api/admin/withdrawals?status=` |
| `POST` | `/api/admin/withdrawals/review` — `{ id, approve, reason }` |
| `GET` | `/api/admin/payments?accountId=&status=` |
| `GET` | `/api/admin/nft/requests?accountId=&status=` |
| `POST` | `/api/admin/nft/mint/confirm` — `{ requestId, mintAddress?, txSignature?, metadataUri?, opId? }` (stub only in development/test) |

## Hot wallet

| Method | Path |
| --- | --- |
| `GET` | `/api/admin/hot-wallet?wallet=` — defaults to `SOLANA_PAYOUT_WALLET` |
| `POST` | `/api/admin/hot-wallet/pause` — `{ paused, wallet?, reason }` — sets `hot_wallet_status.payouts_paused` |

## Super-admin server ops

All write requests require `opId` and `reason`; a repeated `opId` returns its
original result rather than performing the write again.

| Method | Path |
| --- | --- |
| `GET` | `/api/admin/ops/servers?status=&region=` |
| `GET` | `/api/admin/ops/servers/online?region=` |
| `GET` | `/api/admin/ops/servers/{serverId}` |
| `PUT` | `/api/admin/ops/servers/{serverId}` |
| `POST` | `/api/admin/ops/servers/{serverId}/status` |
| `GET` | `/api/admin/ops/servers/online-players?serverId=` |
| `POST` | `/api/admin/ops/servers/online-players/{accountId}/kick` |
| `POST` | `/api/admin/ops/characters/{characterId}/grants/rewards` |
| `POST` | `/api/admin/ops/characters/{characterId}/lottery/draw` |
| `POST` | `/api/admin/ops/characters/{characterId}/lottery/commit-preview` |
| `POST` | `/api/admin/ops/compensation/preview` |
| `GET` | `/api/admin/ops/compensation/previews/{previewId}` |
| `GET` | `/api/admin/ops/compensation/previews/{previewId}/targets?limit=&offset=&keyword=` |
| `POST` | `/api/admin/ops/compensation/commit` |
| `GET` | `/api/admin/ops/payments/economy-orders/{orderId}/trace` |
| `POST` | `/api/admin/ops/payments/economy-orders/{orderId}/recover` |

奖励使用 Aeonblight 的 Gold 和 AEB 余额类别：`gold`、`withdrawableAeb`、`lockedAeb`、`items`。`items[]` 支持 `itemId`、`quantity`，装备还可带 `rarity` 指定星级；Catalog 会把装备模板按当前品质配置展开为 `itemId + rarity` 选项，`grouped=true` 返回完整分组。物品和装备始终进入 Loot Tray；发放、经济账本、审计和幂等结果是同一 PostgreSQL 事务。直接发奖默认不生成稀有奖励公告；如果请求带 `announceRare:true`，则必须同时提供 `announcementSource`，用于公告中的奖励途径。

补偿先以联合筛选条件创建 30 分钟有效的 Preview，再以该 `previewId` 提交。实际执行在一笔事务内按 250 名角色分批；其中任意角色无法发放时全部回滚。Payment Recovery 只处理已有链上收据的 `SUBMITTED`/`CONFIRMED` 订单，不能伪造交易签名，也不会重放已完成订单。

## Example

```bash
curl -s -H "Authorization: Bearer $ADMIN_JWT" \
  "http://127.0.0.1:8083/api/admin/accounts?wallet=YourWallet"

curl -s -X POST -H "Authorization: Bearer $ADMIN_JWT" \
  -H "Content-Type: application/json" \
  -d '{"accountId":1,"restrictionType":"ALL","reason":"manual hold"}' \
  http://127.0.0.1:8083/api/admin/market/restrictions
```
