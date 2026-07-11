# Account and Game Admission Context

## Purpose

Own wallet identity, sessions, characters, game-server registration, launch
tickets, and online presence. After login, the WebGL client receives a
short-lived launch ticket; game servers consume it through an internal route.

## Canonical terms

- **Account** — identity anchored by a case-sensitive Solana wallet address.
- **Session** — revocable login state with access and refresh tokens.
- **Character** — a playable identity owned by an Account.
- **Game Server** — a gameplay partition, not a separate economy.
- **Launch Ticket** — short-lived, one-time admission proof for a Character.
- **Online Presence** — the current Game Server connection for an Account.

## Runtime

`account-api` is independently restartable. PostgreSQL is the durable source of
truth; Redis is the hot adapter for sessions and online presence.
