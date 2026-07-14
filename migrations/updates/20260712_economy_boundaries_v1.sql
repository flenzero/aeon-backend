BEGIN;

ALTER TABLE withdrawals
  DROP CONSTRAINT IF EXISTS chk_withdrawals_amount_positive;

ALTER TABLE withdrawals
  ADD CONSTRAINT chk_withdrawals_amount_positive CHECK (amount > 0);

ALTER TABLE withdrawals
  DROP CONSTRAINT IF EXISTS chk_withdrawals_status;

ALTER TABLE withdrawals
  ADD CONSTRAINT chk_withdrawals_status
    CHECK (status IN ('QUEUED', 'MANUAL_REVIEW', 'REJECTED', 'PAYOUT_CREATED', 'SUBMITTED', 'CONFIRMED', 'FAILED', 'CANCELLED'));

CREATE UNIQUE INDEX IF NOT EXISTS uq_solana_payouts_withdrawal
  ON solana_payouts(withdrawal_id)
  WHERE withdrawal_id IS NOT NULL;

INSERT INTO schema_migrations (version)
VALUES ('20260712_economy_boundaries_v1')
ON CONFLICT DO NOTHING;

COMMIT;
