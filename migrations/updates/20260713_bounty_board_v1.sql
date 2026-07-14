BEGIN;

CREATE TABLE IF NOT EXISTS bounty_account_slot_unlocks (
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  slot_index INTEGER NOT NULL CHECK (slot_index BETWEEN 3 AND 5),
  unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  payment_order_id UUID REFERENCES economy_payment_orders(id) ON DELETE SET NULL,
  PRIMARY KEY (account_id, slot_index)
);

CREATE TABLE IF NOT EXISTS bounty_character_slots (
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  slot_index INTEGER NOT NULL CHECK (slot_index BETWEEN 1 AND 2),
  unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (character_id, slot_index)
);

CREATE TABLE IF NOT EXISTS bounty_tasks (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
  slot_index INTEGER NOT NULL CHECK (slot_index BETWEEN 1 AND 5),
  template_id TEXT NOT NULL,
  task_type TEXT NOT NULL CHECK (task_type IN ('gather', 'combat', 'submit_equipment')),
  difficulty TEXT NOT NULL CHECK (difficulty IN ('normal', 'rare')),
  item_id TEXT,
  min_rarity INTEGER,
  required_quantity BIGINT NOT NULL CHECK (required_quantity > 0),
  progress_quantity BIGINT NOT NULL DEFAULT 0 CHECK (progress_quantity >= 0),
  status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'COMPLETED', 'CLAIMED', 'REPLACED')),
  reward_item_id TEXT NOT NULL,
  reward_quantity BIGINT NOT NULL CHECK (reward_quantity > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at TIMESTAMPTZ,
  claimed_at TIMESTAMPTZ,
  replaced_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_bounty_active_slot
  ON bounty_tasks(character_id, slot_index)
  WHERE status IN ('ACTIVE', 'COMPLETED');

CREATE INDEX IF NOT EXISTS idx_bounty_tasks_character_status
  ON bounty_tasks(character_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS bounty_refreshes (
  character_id BIGINT PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
  free_refresh_available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO schema_migrations (version)
VALUES ('20260713_bounty_board_v1')
ON CONFLICT DO NOTHING;

COMMIT;
