ALTER TABLE memories
  ADD COLUMN kind VARCHAR(32) NULL AFTER type;

ALTER TABLE memories
  ADD KEY idx_memories_kind (kind);
