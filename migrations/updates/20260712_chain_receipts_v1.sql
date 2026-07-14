BEGIN;

ALTER TABLE solana_deposits
  DROP CONSTRAINT IF EXISTS chk_solana_deposits_status;

ALTER TABLE solana_deposits
  ADD CONSTRAINT chk_solana_deposits_status
  CHECK (status IN ('PENDING', 'CREDITED', 'IGNORED', 'REORGED', 'PAYMENT_MATCHED'));

INSERT INTO schema_migrations (version)
VALUES ('20260712_chain_receipts_v1')
ON CONFLICT DO NOTHING;

COMMIT;
