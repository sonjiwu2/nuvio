-- The Safe Operation Layer's journal. Every successfully executed move is
-- recorded here before Apply returns, which is what makes Undo possible.
-- Skipped and failed items are never journaled — they never touched the
-- filesystem, so there is nothing to reverse.
CREATE TABLE operation_items (
    id           TEXT PRIMARY KEY,
    batch_id     TEXT NOT NULL,
    source       TEXT NOT NULL,
    destination  TEXT NOT NULL,
    executed_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    undone_at    TEXT
);

CREATE INDEX idx_operation_items_batch ON operation_items (batch_id);
