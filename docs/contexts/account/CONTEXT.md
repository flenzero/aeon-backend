# Account and Game Admission Context

## Purpose

Own wallet identity, sessions, public server selection, characters, game-server
registration, launch tickets, and online presence. After wallet login, the home
page receives an account-level, server-bound launch ticket; game servers consume
it through an internal route before the client chooses a Character.

## Canonical terms

- **Account** — identity anchored by a case-sensitive Solana wallet address.
- **Session** — revocable login state with access and refresh tokens.
- **Character** — a playable identity owned by an Account.
- **Game Server** — a gameplay partition, not a separate economy.
- **Service Identity** — one super-admin-approved Ed25519 public identity for a
  machine caller, with explicit capabilities and revocable status.
- **Launch Ticket** — short-lived, one-time admission proof for an Account and
  one Game Server.
- **Online Presence** — the current Game Server connection for an Account.
- **Dungeon Recovery** — the post-selection decision that either issues an
  origin-server-only Launch Ticket or cancels an unfinished Dungeon Run without rewards.

## Runtime

`account-api` is independently restartable. PostgreSQL is the durable source of
truth; Redis is the hot adapter for sessions and online presence.
Each production Game Server uses a different Service Identity whose `subjectId`
must match its `serverId`; it cannot act on another Game Server's tickets,
heartbeat, or online presence.
Redis stores a hot Dungeon Recovery hint, but PostgreSQL remains authoritative
for whether the run is still `STARTED`.
