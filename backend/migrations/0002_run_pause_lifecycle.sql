-- Rebuild runs so existing development databases accept durable pause states.
PRAGMA foreign_keys = OFF;

CREATE TABLE runs_pause_v1 (
    id                  TEXT PRIMARY KEY,
    thread_id           TEXT NOT NULL,
    project_id          TEXT NOT NULL,
    scope_artifact      TEXT NOT NULL CHECK (scope_artifact IN ('spec','ppt')),
    scope_level         TEXT NOT NULL CHECK (scope_level IN ('slide','deck')),
    scope_slide_id      TEXT,
    mode                TEXT NOT NULL CHECK (mode IN ('talk','ask','plan','execute')),
    run_command_json    TEXT NOT NULL,
    client_request_id   TEXT DEFAULT '',
    model_profile_name  TEXT,
    model_provider      TEXT,
    model_name          TEXT,
    model_url           TEXT,
    cancel_requested_at INTEGER,
    owner_instance_id   TEXT NOT NULL DEFAULT '',
    pause_reason        TEXT NOT NULL DEFAULT '',
    paused_at           INTEGER,
    status              TEXT NOT NULL DEFAULT 'pending'
                           CHECK (status IN ('pending','running','waiting','paused','recovering','done','failed','canceled')),
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

INSERT INTO runs_pause_v1 (
    id, thread_id, project_id, scope_artifact, scope_level, scope_slide_id,
    mode, run_command_json, client_request_id, model_profile_name, model_provider,
    model_name, model_url, cancel_requested_at, owner_instance_id, pause_reason,
    paused_at, status, created_at, updated_at
)
SELECT
    id, thread_id, project_id, scope_artifact, scope_level, scope_slide_id,
    mode, run_command_json, client_request_id, model_profile_name, model_provider,
    model_name, model_url, cancel_requested_at, '', '', NULL,
    status, created_at, updated_at
FROM runs;

DROP TABLE runs;
ALTER TABLE runs_pause_v1 RENAME TO runs;

CREATE INDEX idx_runs_thread ON runs(thread_id);
CREATE INDEX idx_runs_project ON runs(project_id);
CREATE INDEX idx_runs_status ON runs(status);
CREATE UNIQUE INDEX idx_runs_thread_client_request
ON runs(thread_id, client_request_id)
WHERE client_request_id IS NOT NULL AND client_request_id <> '';

PRAGMA foreign_keys = ON;
