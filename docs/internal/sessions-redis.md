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

## Client / game-server flow

```text
1. POST /api/auth/wallet          → accessToken + refreshToken + sessionId
2. POST /api/auth/refresh         → rotate refresh, new accessToken
3. POST /api/auth/logout          → revoke session + refresh
4. POST /api/game/servers/register (internal)
5. POST /api/game/launch          → requires ACTIVE sessionId
6. POST /api/game/launch/consume  → consumes ticket AND enters online_sessions
7. POST /api/game/online/heartbeat (internal, every < ONLINE_PRESENCE_TTL_SECONDS)
8. POST /api/game/online/leave
```

## Environment

- `REDIS_ENABLED` (default true)
- `REDIS_ADDR` (compose: `redis:6379`, local example: `127.0.0.1:56379`)
- `SESSION_TTL_HOURS` (default 168)
- `ONLINE_PRESENCE_TTL_SECONDS` (default 90)

## Ops

- `GET /api/auth/session/redis` (internal) — redis connectivity
- `POST /api/game/online/sweep` (internal) — delete stale `online_sessions`
