# One-command local verification

Run from the repository root:

```bash
./test/run.sh
```

The command verifies:

- all Go unit tests;
- registration and authentication guards for all 89 HTTP routes;
- a successful wallet-login → character → economy snapshot → admin lookup flow;
- the backend NFT request → confirm → asset-list HTTP lifecycle;
- all Solana JSON-RPC methods used by the backend against a local in-memory transport;
- builds for `account-api`, `economy-api`, `admin-api`, and `economy-worker`;
- local startup/health for all three HTTP processes and a successful worker tick;
- all PostgreSQL integration tests when Docker is running.

Go build artifacts stay under `work/`; the module download cache uses the
`/tmp/aeon-backend-go-mod` so `go test ./...` never scans dependency sources.

Use strict full mode when PostgreSQL/Redis coverage must not be skipped:

```bash
./test/run.sh --full
```

`--full` fails when the Docker daemon is unavailable. It starts only the
`postgres` and `redis` Compose services and leaves their named volumes intact.

## Scope of the NFT test

The local NFT contract test verifies the backend lifecycle and the durable
PostgreSQL lifecycle (`TestPostgresNFTMintRequestConfirm`). It does not claim to
mint an NFT on Solana. The production on-chain mint adapter/program is still
absent from this repository and requires a selected Solana NFT standard plus a
Solana/Anchor or compatible Rust toolchain.
