PRAGMA foreign_keys = OFF;

CREATE TABLE runs_v3 (
    id                 TEXT PRIMARY KEY,
    thread_id          TEXT NOT NULL,
    project_id         TEXT NOT NULL,
    target_artifact    TEXT NOT NULL CHECK (target_artifact IN ('spec','presentation')),
    target_level       TEXT NOT NULL CHECK (target_level IN ('slide','deck')),
    target_slide_id    TEXT,
    interaction_intent TEXT NOT NULL CHECK (interaction_intent IN ('talk','ask','plan','execute')),
    work_spec_json     TEXT NOT NULL,
    client_request_id  TEXT DEFAULT '',
    model_profile_name TEXT,
    model_provider     TEXT,
    model_name         TEXT,
    model_url          TEXT,
    cancel_requested_at INTEGER,
    status             TEXT NOT NULL DEFAULT 'pending'
                               CHECK (status IN ('pending','running','waiting','done','failed','canceled')),
    created_at         INTEGER NOT NULL,
    updated_at         INTEGER NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

INSERT INTO runs_v3 (
    id,thread_id,project_id,target_artifact,target_level,target_slide_id,
    interaction_intent,work_spec_json,client_request_id,model_profile_name,
    model_provider,model_name,model_url,cancel_requested_at,status,created_at,updated_at
)
SELECT
    id,thread_id,project_id,target_artifact,target_level,target_slide_id,
    interaction_intent,work_spec_json,client_request_id,model_profile_name,
    model_provider,model_name,model_url,cancel_requested_at,status,created_at,updated_at
FROM runs;

DROP TABLE runs;
ALTER TABLE runs_v3 RENAME TO runs;

CREATE INDEX idx_runs_thread ON runs(thread_id);
CREATE INDEX idx_runs_project ON runs(project_id);
CREATE INDEX idx_runs_status ON runs(status);
CREATE UNIQUE INDEX idx_runs_thread_client_request
ON runs(thread_id, client_request_id)
WHERE client_request_id IS NOT NULL AND client_request_id <> '';

PRAGMA foreign_keys = ON;
