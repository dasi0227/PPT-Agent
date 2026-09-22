-- Checkpoints own content history. No legacy file or metadata backfill.
DROP TABLE versions;

CREATE TABLE deleted_slides (
    project_id TEXT NOT NULL,
    slide_id TEXT NOT NULL,
    PRIMARY KEY (project_id, slide_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
