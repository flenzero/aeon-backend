-- Phase 4 Solana chain helpers for existing databases.
-- Folded into aeonblight_full_schema.sql for fresh one-shot deploys.

CREATE UNIQUE INDEX IF NOT EXISTS uq_economy_payment_orders_tx_signature
  ON economy_payment_orders(tx_signature)
  WHERE tx_signature IS NOT NULL;

INSERT INTO chain_cursors (name, network, cursor_slot, status)
VALUES ('solana_deposits', 'solana-devnet', 0, 'OK')
ON CONFLICT (name) DO NOTHING;

INSERT INTO schema_migrations (version)
VALUES ('20260710_solana_chain_v1')
ON CONFLICT DO NOTHING;
