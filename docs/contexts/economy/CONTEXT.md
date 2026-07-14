# Economy and Chain Settlement Context

## Purpose

Own durable assets and economy state: balances, inventory, equipment, dungeon
and activity settlement, marketplace, payments, withdrawals, Solana accounting,
and the backend NFT lifecycle.

## Canonical terms

- **Economy Snapshot** — one Character's durable inventory/state plus the owning Account's token balance.
- **Locked AEB** — earned token value waiting for its unlock time.
- **Payment Order** — expected on-chain payment matched to one backend purpose.
- **Withdrawal** — request to move withdrawable value to a Solana wallet.
- **NFT Mint Request** — paid backend request that locks an Equipment item pending mint.
- **NFT Asset** — backend representation of an off-chain, mint-pending, or minted Equipment item.
- **Chain Settlement** — deposit discovery, payment matching, payout submission, and confirmation.
- **Capability** — a narrow permission on a signed Service Identity, such as
  `economy.gameplay`, `economy.worker`, `economy.payments`, or `economy.mint`.
- **Dungeon Origin Server** — the Game Server that created a Dungeon Run and is
  the only server allowed to finish it.

## Runtime

`economy-api` is the authority for durable economy writes. Game servers submit
facts and intent; they do not write economy tables directly. Solana is the
default chain. The current repository implements chain RPC/payment/payout logic,
but does not implement a real on-chain NFT mint adapter or program. Production
NFT confirmation therefore fails closed instead of manufacturing a stub asset.
