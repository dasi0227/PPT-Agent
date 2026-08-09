PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS legacy_slide_materializations (
    slide_id                TEXT PRIMARY KEY,
    project_id              TEXT NOT NULL,
    html_revision           INTEGER NOT NULL DEFAULT 0,
    source_outline_revision INTEGER NOT NULL DEFAULT 0,
    source_spec_revision    INTEGER NOT NULL DEFAULT 0,
    source_design_revision  INTEGER NOT NULL DEFAULT 0
);

INSERT OR REPLACE INTO legacy_slide_materializations (
    slide_id,project_id,html_revision,source_outline_revision,
    source_spec_revision,source_design_revision
)
SELECT
    id,project_id,html_revision,source_outline_revision,
    source_spec_revision,source_design_revision
FROM slides;

CREATE TABLE projects_v4 (
    id             TEXT PRIMARY KEY,
    title          TEXT NOT NULL,
    work_dir       TEXT NOT NULL,
    theme          TEXT NOT NULL DEFAULT 'default',
    status         TEXT NOT NULL DEFAULT 'draft'
                           CHECK (status IN ('draft','generating','ready')),
    layout_version INTEGER NOT NULL DEFAULT 1
                           CHECK (layout_version IN (1,2,3)),
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

INSERT INTO projects_v4 (
    id,title,work_dir,theme,status,layout_version,created_at,updated_at
)
SELECT
    id,title,work_dir,theme,status,layout_version,created_at,updated_at
FROM projects;

CREATE TABLE slides_v4 (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL,
    current_version INTEGER NOT NULL DEFAULT 0,
    last_export_at  INTEGER,
    FOREIGN KEY (project_id) REFERENCES projects_v4(id) ON DELETE CASCADE
);

INSERT INTO slides_v4 (
    id,project_id,current_version,last_export_at
)
SELECT
    id,project_id,current_version,last_export_at
FROM slides;

DROP TABLE slides;
DROP TABLE projects;

ALTER TABLE slides_v4 RENAME TO slides;
ALTER TABLE projects_v4 RENAME TO projects;

CREATE INDEX idx_slides_project ON slides(project_id);

PRAGMA foreign_keys = ON;
