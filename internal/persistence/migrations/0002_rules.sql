-- Organize rules: extension -> destination folder. These are definitions
-- only. Nuvio never moves a file automatically from a rule — see
-- internal/rules' package doc for why applying rules is deferred until a
-- Safe Operation Layer (journal, trash, undo) exists.
CREATE TABLE rules (
    id                  TEXT PRIMARY KEY,
    extension           TEXT NOT NULL,
    destination_folder  TEXT NOT NULL,
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_rules_extension ON rules (extension);
