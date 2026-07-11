# Solana Chain Boundary

The new game defaults to Solana, not EVM.

## Address Rules

Solana wallet addresses are base58 public keys and are case-sensitive. Backend
code must not lowercase wallet addresses.

Current development validation lives in `internal/chain`:

- accepts Solana base58 addresses that decode to 32-byte public keys
- rejects EVM-style `0x...` addresses
- preserves address casing
- verifies wallet login signatures with Ed25519 over the exact nonce message
- accepts base58, base64 or hex encoded 64-byte signatures

Wallet login now uses one-time `wallet_login_nonces` rows:

```text
GET  /api/auth/wallet/nonce?walletAddress=<solana address>
POST /api/auth/wallet
```

The login request must include `walletAddress`, `nonce` and `signature`.

## First Integration Targets

The chain adapter should be added inside the economy domain first, not as a new
runtime service:

```text
economy-api
  internal/chain
economy-worker
  -> economy-api internal chain/withdrawal endpoints
```

This keeps the deployment small while preserving a clear code boundary for a
future `chain-service` split.

## Payout Signer (`SOLANA_PAYOUT_MODE=rpc`)

`economy-worker` / internal submit builds a real SPL `TransferChecked` with
`SOLANA_PAYOUT_PRIVATE_KEY` (base58 64-byte secret or JSON byte array) and
`sendTransaction`.

Recipient ATA must already exist. Missing ATA fails that payout only (no
auto-create). Product rule: real trading requires holding AEB (or a trading
license path later).

Hot wallet ATA balance of zero (or below `SOLANA_PAYOUT_LOW_BALANCE_RAW`) pauses
further payouts via `hot_wallet_status.payouts_paused`.

## Phase 4 Runtime

Environment:

- `SOLANA_RPC_ENABLED` (default `false`)
- `SOLANA_RPC_URL`, `SOLANA_NETWORK`
- `SOLANA_TOKEN_MINT`, `SOLANA_TOKEN_DECIMALS`
- `SOLANA_DEPOSIT_WALLET` (treasury owner that receives inbound AEB)
- `SOLANA_PAYOUT_WALLET`, `SOLANA_PAYOUT_PRIVATE_KEY`, `SOLANA_PAYOUT_MODE`
  (`stub` | `record` | `rpc`)
- `SOLANA_PAYOUT_LOW_BALANCE_RAW` (optional pause threshold in raw units)

Worker ticks:

1. settle unlocks
2. process withdrawals (creates `solana_payouts`; stub confirms immediately)
3. scan deposits → credit `external_balance`, or match open payment orders
4. submit / confirm payouts when not in stub mode

## Wallet expand / on-chain payment orders

Supported purposes:

- `MARKET_SLOT_WALLET_EXPAND` — marketplace listing slot expand
- `BAG_EXPAND` — character bag capacity expand
- `TRADING_LICENSE` — account trading license (required for marketplace list/buy)

Flow:

1. Game server creates order:
   - `POST /api/economy/marketplace/slots/expand-wallet`
   - `POST /api/economy/inventory/bag/expand`
   - `POST /api/economy/license/purchase`
2. Player pays AEB on-chain to `SOLANA_DEPOSIT_WALLET`
3. Game server submits tx hash (backend verifies on-chain when RPC enabled):
   - `POST /api/economy/marketplace/slots/expand-wallet/submit`
   - or `POST /api/economy/internal/payments/submit`
4. On success the order is fulfilled immediately after verification

When `SOLANA_RPC_ENABLED=false`, submit only records the signature
(`SUBMITTED`); use `POST /api/economy/internal/payments/confirm` for local
fulfillment.

Deposit scan can also auto-fulfill a matching open order (same account + amount)
without crediting `external_balance` (`solana_deposits.status=PAYMENT_MATCHED`).

Trading license does **not** bypass withdrawal recipient ATA checks.