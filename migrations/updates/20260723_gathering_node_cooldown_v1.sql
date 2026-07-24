CREATE INDEX IF NOT EXISTS idx_gathering_settlements_character_node
  ON gathering_settlements(character_id, node_key, created_at DESC);
