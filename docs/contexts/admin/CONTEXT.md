# Admin Operations Context

## Purpose

Own privileged account controls, marketplace restrictions, risk events,
withdrawal review, queue visibility, hot-wallet pause, and audit records.

## Canonical terms

- **Admin Action** — privileged mutation performed with an admin identity and reason.
- **Risk Event** — durable signal attached to an Account for review or enforcement.
- **Market Restriction** — BUY, SELL, or ALL prohibition applied to an Account.
- **Audit Record** — durable record of an Admin Action and its target.
- **Hot-wallet Pause** — switch preventing payout submission for a configured wallet.

## Runtime

`admin-api` is independently restartable and requires the shared PostgreSQL
database. Admin mutations are expected to produce audit records.
