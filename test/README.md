# One-command local verification

Run from the repository root. The default is the fast AI contract profile:

```bash
./test/run.sh
```

Available completeness levels:

```bash
./test/run.sh --unit
./test/run.sh --contract
./test/run.sh --integration
./test/run.sh --full
```

- `unit`: internal Go module behavior only.
- `contract`: unit tests, all 116 HTTP route/auth contracts, wallet workflow,
  the 61 internal-route capability matrix, all 78 JSON write-route size
  boundaries, strict JSON/query/signature tests, Solana JSON-RPC transport, NFT
  backend lifecycle, and four binary builds.
- `integration`: contract coverage plus explicit database migration and all
  19 PostgreSQL integration groups; Docker is required.
- `full`: integration coverage plus local startup of all four modules,
  `/health`, `/ready`, Redis connectivity, and a worker tick.

Configuration can also be provided through `TEST_SCOPE`. `STUB_MODE` accepts
`enabled` or `disabled`; a full run with stub disabled requires complete real
Solana configuration and intentionally refuses to start otherwise.

The integration/full profiles call the only migration entry explicitly,
choosing `scripts/db-migrate.sh bootstrap` for a new database or `up` for an
existing database. Docker and application startup never change the schema.

## NFT coverage

The suite verifies rarity-based AEB fee snapshots, idempotent backend request /
confirm / list state, and cancellation refunds to the original locked,
withdrawable, and external AEB balance categories.

Metaplex Core is the selected NFT standard. The repository does not yet submit
real Core Asset creation/freeze/unfreeze transactions; that adapter and its
validator-backed tests remain the next NFT milestone.
