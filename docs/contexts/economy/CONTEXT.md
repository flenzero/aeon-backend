# Economy and Chain Settlement Context

## Purpose

Own durable assets and economy state: balances, inventory, equipment, dungeon
and activity settlement, marketplace, payments, withdrawals, Solana accounting,
and the backend NFT lifecycle.

## Canonical terms

- **Economy Snapshot** — one Character's durable inventory/state plus the owning Account's token balance.
- **Locked GAME** — earned token value waiting for its unlock time.
- **Payment Order** — expected on-chain payment matched to one backend purpose.
- **Withdrawal** — request to move withdrawable value to a Solana wallet.
- **NFT Mint Request** — paid backend request that locks an Equipment item pending mint.
- **NFT Asset** — backend representation of an off-chain, mint-pending, or minted Equipment item.
- **Chain Settlement** — deposit discovery, payment matching, payout submission, and confirmation.

## Runtime

`economy-api` is the authority for durable economy writes. Game servers submit
facts and intent; they do not write economy tables directly. Solana is the
default chain. The current repository implements chain RPC/payment/payout logic,
but does not implement a real on-chain NFT mint adapter or program.
