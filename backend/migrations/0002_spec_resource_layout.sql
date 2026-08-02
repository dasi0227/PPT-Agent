PRAGMA foreign_keys = OFF;

CREATE TABLE projects_v2 (
    id               TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    work_dir         TEXT NOT NULL,
    theme            TEXT NOT NULL DEFAULT 'default',
    status           TEXT NOT NULL DEFAULT 'draft'
                             CHECK (status IN ('draft','generating','ready')),
    design_path      TEXT NOT NULL DEFAULT 'design.json',
    outline_path     TEXT NOT NULL DEFAULT 'outline.json',
    outline_revision INTEGER NOT NULL DEFAULT 1,
    design_revision  INTEGER NOT NULL DEFAULT 1,
    layout_version   INTEGER NOT NULL DEFAULT 1
                             CHECK (layout_version IN (1,2)),
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL
);
INSERT INTO projects_v2 (
    id,title,work_dir,theme,status,design_path,outline_path,
    outline_revision,design_revision,layout_version,created_at,updated_at
)
SELECT
    id,title,work_dir,theme,status,design_path,deck_path,
    deck_revision,design_revision,1,created_at,updated_at
FROM projects;

CREATE TABLE slides_v2 (
    id                      TEXT PRIMARY KEY,
    project_id              TEXT NOT NULL,
    position                INTEGER NOT NULL,
    layout                  TEXT NOT NULL,
    title                   TEXT NOT NULL DEFAULT '',
    spec_path               TEXT NOT NULL,
    html_path               TEXT NOT NULL,
    current_version         INTEGER NOT NULL DEFAULT 0,
    spec_revision           INTEGER NOT NULL DEFAULT 1,
    html_revision           INTEGER NOT NULL DEFAULT 0,
    source_outline_revision INTEGER NOT NULL DEFAULT 0,
    source_spec_revision    INTEGER NOT NULL DEFAULT 0,
    source_design_revision  INTEGER NOT NULL DEFAULT 0,
    last_export_at          INTEGER,
    FOREIGN KEY (project_id) REFERENCES projects_v2(id) ON DELETE CASCADE,
    UNIQUE (project_id, position)
);
INSERT INTO slides_v2 (
    id,project_id,position,layout,title,spec_path,html_path,current_version,
    spec_revision,html_revision,source_outline_revision,source_spec_revision,
    source_design_revision,last_export_at
)
SELECT
    id,project_id,position,layout,title,json_path,html_path,current_version,
    blueprint_revision,presentation_revision,source_deck_revision,
    source_blueprint_revision,source_design_revision,last_export_at
FROM slides;

CREATE TABLE versions_v2 (
    id            TEXT PRIMARY KEY,
    target_type   TEXT NOT NULL
                           CHECK (target_type IN ('outline','slide_spec','slide_html','design','asset')),
    target_id     TEXT NOT NULL,
    version_no    INTEGER NOT NULL,
    snapshot_path TEXT NOT NULL,
    run_id        TEXT,
    created_at    INTEGER NOT NULL,
    UNIQUE (target_type, target_id, version_no)
);
INSERT INTO versions_v2 (
    id,target_type,target_id,version_no,snapshot_path,run_id,created_at
)
SELECT
    id,
    CASE target_type
      WHEN 'blueprint_deck' THEN 'outline'
      WHEN 'blueprint_slide' THEN 'slide_spec'
      WHEN 'presentation_slide' THEN 'slide_html'
      WHEN 'design_spec' THEN 'design'
      ELSE target_type
    END,
    target_id,version_no,snapshot_path,run_id,created_at
FROM versions;

CREATE TABLE runs_v2 (
    id                 TEXT PRIMARY KEY,
    thread_id          TEXT NOT NULL,
    project_id         TEXT NOT NULL,
    target_artifact    TEXT NOT NULL CHECK (target_artifact IN ('spec','presentation')),
    target_level       TEXT NOT NULL CHECK (target_level IN ('slide','deck')),
    target_slide_id    TEXT,
    interaction_intent TEXT NOT NULL CHECK (interaction_intent IN ('talk','ask','execute')),
    work_spec_json     TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending','running','waiting','done','failed','canceled')),
    created_at         INTEGER NOT NULL,
    updated_at         INTEGER NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects_v2(id) ON DELETE CASCADE
);
INSERT INTO runs_v2 (
    id,thread_id,project_id,target_artifact,target_level,target_slide_id,
    interaction_intent,work_spec_json,status,created_at,updated_at
)
SELECT
    id,thread_id,project_id,
    CASE target_artifact WHEN 'blueprint' THEN 'spec' ELSE target_artifact END,
    target_level,target_slide_id,interaction_intent,
    json_set(work_spec_json, '$.target.artifact',
      CASE json_extract(work_spec_json, '$.target.artifact')
        WHEN 'blueprint' THEN 'spec'
        ELSE json_extract(work_spec_json, '$.target.artifact')
      END
    ),
    status,created_at,updated_at
FROM runs;

DROP TABLE runs;
DROP TABLE slides;
DROP TABLE versions;
DROP TABLE projects;

ALTER TABLE projects_v2 RENAME TO projects;
ALTER TABLE slides_v2 RENAME TO slides;
ALTER TABLE versions_v2 RENAME TO versions;
ALTER TABLE runs_v2 RENAME TO runs;

CREATE INDEX idx_slides_project ON slides(project_id);
CREATE INDEX idx_versions_target ON versions(target_type, target_id);
CREATE INDEX idx_runs_thread ON runs(thread_id);
CREATE INDEX idx_runs_project ON runs(project_id);
CREATE INDEX idx_runs_status ON runs(status);

PRAGMA foreign_keys = ON;
