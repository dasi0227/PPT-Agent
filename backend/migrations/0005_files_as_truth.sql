-- Phase B · 文件为真相：DB 退出内容维度。
-- slides 删除 position/layout/title/spec_path/html_path（顺序来自 outline.json 的
-- slide_order，title/layout 来自 spec.json，路径由 slide_id 机械推导）。
-- projects 删除 design_path/outline_path（路径由协议固定推导）。
-- DB 仅保留编排与版本/修订游标。SQLite 无 DROP COLUMN，采用表重建。

PRAGMA foreign_keys = OFF;

CREATE TABLE slides_v3 (
    id                      TEXT PRIMARY KEY,
    project_id              TEXT NOT NULL,
    current_version         INTEGER NOT NULL DEFAULT 0,
    spec_revision           INTEGER NOT NULL DEFAULT 1,
    html_revision           INTEGER NOT NULL DEFAULT 0,
    source_outline_revision INTEGER NOT NULL DEFAULT 0,
    source_spec_revision    INTEGER NOT NULL DEFAULT 0,
    source_design_revision  INTEGER NOT NULL DEFAULT 0,
    last_export_at          INTEGER,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
INSERT INTO slides_v3 (
    id,project_id,current_version,spec_revision,html_revision,
    source_outline_revision,source_spec_revision,source_design_revision,last_export_at
)
SELECT
    id,project_id,current_version,spec_revision,html_revision,
    source_outline_revision,source_spec_revision,source_design_revision,last_export_at
FROM slides;

CREATE TABLE projects_v3 (
    id               TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    work_dir         TEXT NOT NULL,
    theme            TEXT NOT NULL DEFAULT 'default',
    status           TEXT NOT NULL DEFAULT 'draft'
                             CHECK (status IN ('draft','generating','ready')),
    outline_revision INTEGER NOT NULL DEFAULT 1,
    design_revision  INTEGER NOT NULL DEFAULT 1,
    layout_version   INTEGER NOT NULL DEFAULT 1
                             CHECK (layout_version IN (1,2)),
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL
);
INSERT INTO projects_v3 (
    id,title,work_dir,theme,status,outline_revision,design_revision,
    layout_version,created_at,updated_at
)
SELECT
    id,title,work_dir,theme,status,outline_revision,design_revision,
    layout_version,created_at,updated_at
FROM projects;

DROP TABLE slides;
DROP TABLE projects;

ALTER TABLE slides_v3 RENAME TO slides;
ALTER TABLE projects_v3 RENAME TO projects;

CREATE INDEX idx_slides_project ON slides(project_id);

PRAGMA foreign_keys = ON;
