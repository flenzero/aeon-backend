# Admin API

`admin-api` (default `:8083`) is the privileged ops surface. All routes below
require:

```http
Authorization: Bearer <ADMIN_TOKEN>
```

Default local token: `dev-admin-token` (`ADMIN_TOKEN` env).

On-chain NFT mint signing remains stubbed; admin can still list pending mint
requests and confirm with a stub / operator-supplied mint address.

## Account

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/api/admin/accounts?accountId=` or `?wallet=` | Detail: status, risk, ban reason, license, active market restrictions |
| `POST` | `/api/admin/accounts/ban` | `{ accountId, banned, reason, adminId }` — writes `ban_reason` + risk event |
| `POST` | `/api/admin/accounts/risk-level` | `{ accountId, riskLevel, reason }` |
| `POST` | `/api/admin/accounts/license` | `{ accountId, granted, reason }` — grant/revoke trading license |
| `POST` | `/api/admin/accounts/sessions/revoke` | Force-revoke all ACTIVE sessions |

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

## Withdrawals / payments / NFT

| Method | Path |
| --- | --- |
| `GET` | `/api/admin/withdrawals?status=` |
| `POST` | `/api/admin/withdrawals/review` — `{ id, approve, reason }` |
| `GET` | `/api/admin/payments?accountId=&status=` |
| `GET` | `/api/admin/nft/requests?accountId=&status=` |
| `POST` | `/api/admin/nft/mint/confirm` — `{ requestId, mintAddress?, txSignature?, metadataUri?, opId? }` (stub mint OK) |

## Hot wallet

| Method | Path |
| --- | --- |
| `GET` | `/api/admin/hot-wallet?wallet=` — defaults to `SOLANA_PAYOUT_WALLET` |
| `POST` | `/api/admin/hot-wallet/pause` — `{ paused, wallet?, reason }` — sets `hot_wallet_status.payouts_paused` |

## Example

```bash
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://127.0.0.1:8083/api/admin/accounts?wallet=YourWallet"

curl -s -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"accountId":1,"restrictionType":"ALL","reason":"manual hold"}' \
  http://127.0.0.1:8083/api/admin/market/restrictions
```
