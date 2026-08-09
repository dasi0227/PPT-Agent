PRAGMA foreign_keys = OFF;

CREATE TABLE projects_v6 (
    id             TEXT PRIMARY KEY,
    title          TEXT NOT NULL,
    work_dir       TEXT NOT NULL,
    theme          TEXT NOT NULL DEFAULT 'default',
    status         TEXT NOT NULL DEFAULT 'draft'
                           CHECK (status IN ('draft','generating','ready')),
    layout_version INTEGER NOT NULL DEFAULT 1
                           CHECK (layout_version IN (1,2,3,4,5)),
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

INSERT INTO projects_v6 (
    id,title,work_dir,theme,status,layout_version,created_at,updated_at
)
SELECT
    id,title,work_dir,theme,status,layout_version,created_at,updated_at
FROM projects;

DROP TABLE projects;

ALTER TABLE projects_v6 RENAME TO projects;

PRAGMA foreign_keys = ON;
