# Account Sessions & Online Coordination

`account-api` persists login sessions in Postgres and caches hot paths in Redis.

## Data plane

| Concern | Postgres | Redis |
|---------|----------|-------|
| Session row | `account_sessions` | `aeon:session:{id}` |
| Refresh token (hashed) | `refresh_tokens` | `aeon:refresh:{sha256}` |
| Online presence | `online_sessions` | `aeon:online:acct:{accountId}` + `aeon:online:server:{serverId}` SET |
| Game servers | `game_servers` | `aeon:server:{serverId}` (short TTL mirror) |

If Redis is down at startup, account-api falls back to an in-process memory cache
and still writes Postgres. Set `REDIS_ENABLED=false` for postgres-only mode.

Public online counts use Redis Online Presence, not the game-server heartbeat's
reported `onlinePlayers`. The server SET is only an index; `account-api` verifies
each member against `aeon:online:acct:{accountId}` and only counts entries whose
TTL-backed account key still exists and still points at that server. Stale SET
members are removed during the read. In postgres-only development/test mode, the
same public count falls back to `online_sessions.last_seen_at` within
`ONLINE_PRESENCE_TTL_SECONDS`.

## Client / game-server flow

```text
1. POST /api/auth/wallet          → accessToken + refreshToken + sessionId
2. POST /api/auth/refresh         → rotate refresh, new accessToken
3. POST /api/auth/logout          → revoke session + refresh
4. GET  /api/public/servers       → public server selection, no host/port
5. POST /api/game/servers/register (internal)
6. POST /api/game/launch          → account-level ticket, requires ACTIVE sessionId
7. POST /api/game/launch/consume  → consumes ticket and returns account admission
8. POST /api/game/online/enter    → binds selected character + connection
9. POST /api/game/online/heartbeat (internal, every < ONLINE_PRESENCE_TTL_SECONDS)
10. POST /api/game/online/leave
```

## Environment

- `REDIS_ENABLED` (default true)
- `REDIS_ADDR` (compose: `redis:6379`, local example: `127.0.0.1:56379`)
- `SESSION_TTL_HOURS` (default 168)
- `ONLINE_PRESENCE_TTL_SECONDS` (default 90)

## Ops

- `GET /api/auth/session/redis` (internal) — redis connectivity
- `POST /api/game/online/sweep` (internal) — delete stale `online_sessions`
