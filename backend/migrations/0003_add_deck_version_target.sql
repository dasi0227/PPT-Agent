PRAGMA foreign_keys = OFF;

ALTER TABLE versions RENAME TO versions_without_deck;

CREATE TABLE versions (
    id            TEXT PRIMARY KEY,
    target_type   TEXT NOT NULL
                      CHECK (target_type IN ('deck','outline','slide_spec','slide_html','design','asset')),
    target_id     TEXT NOT NULL,
    version_no    INTEGER NOT NULL,
    snapshot_path TEXT NOT NULL,
    run_id        TEXT,
    created_at    INTEGER NOT NULL,
    UNIQUE (target_type, target_id, version_no)
);

INSERT INTO versions(id, target_type, target_id, version_no, snapshot_path, run_id, created_at)
SELECT id, target_type, target_id, version_no, snapshot_path, run_id, created_at
FROM versions_without_deck;

DROP TABLE versions_without_deck;

CREATE INDEX idx_versions_target ON versions(target_type, target_id);
CREATE UNIQUE INDEX idx_versions_run_target
ON versions(run_id, target_type, target_id) WHERE run_id IS NOT NULL;

PRAGMA foreign_keys = ON;
