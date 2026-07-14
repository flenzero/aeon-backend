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
- **Service Identity Approval** — super-admin creation of a machine identity's
  public key, kind, subject and least-privilege capabilities.
- **Service Identity Revocation** — soft disable that immediately blocks signed
  requests while retaining the identity and audit history.

## Runtime

`admin-api` is independently restartable and requires the shared PostgreSQL
database. Admin mutations are expected to produce audit records.
Service Identity listing is an administrator read operation. Approval/creation
and revocation are super-administrator operations. Ordinary administrators are
created by a super administrator with an Ed25519 public key, sign a one-time
login challenge, and receive a short-lived JWT controlled by
`ADMIN_SESSION_TTL_MINUTES` (default 30). Disabled admins are rejected even if a
previous JWT has not expired. `SUPER_ADMIN_OPS_KEY` authenticates a super
administrator for high-risk operations and ordinary-admin configuration.
Development/test retain an explicit `ADMIN_TOKEN` Bootstrap Super Admin
compatibility mode only while the latter is unset; staging and production
require the separate super-admin key.
