# Economy Worker Context

## Purpose

Advance scheduled economy state by calling internal `economy-api` routes. The
worker does not own a second copy of economy state and does not write the
database directly.

## Canonical terms

- **Worker Tick** — one ordered pass through scheduled economy jobs.
- **Unlock Settlement** — promotion of mature Locked GAME into withdrawable value.
- **Withdrawal Processing** — queue transition according to limits and review rules.
- **Deposit Scan** — discovery and matching of inbound Solana token transfers.
- **Payout Submission** — signing/sending an eligible queued Withdrawal.
- **Payout Confirmation** — final chain-status reconciliation for a submitted payout.

## Runtime

`economy-worker` is independently restartable. It calls `economy-api` with the
internal key and currently runs five jobs on every tick.
