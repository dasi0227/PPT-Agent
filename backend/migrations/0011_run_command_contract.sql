PRAGMA foreign_keys = OFF;

CREATE TABLE runs_v4 (
    id                  TEXT PRIMARY KEY,
    thread_id           TEXT NOT NULL,
    project_id          TEXT NOT NULL,
    scope_artifact      TEXT NOT NULL CHECK (scope_artifact IN ('spec','ppt')),
    scope_level         TEXT NOT NULL CHECK (scope_level IN ('slide','deck')),
    scope_slide_id      TEXT,
    intent              TEXT NOT NULL CHECK (intent IN ('talk','ask','plan','execute')),
    run_command_json    TEXT NOT NULL,
    client_request_id   TEXT DEFAULT '',
    model_profile_name  TEXT,
    model_provider      TEXT,
    model_name          TEXT,
    model_url           TEXT,
    cancel_requested_at INTEGER,
    status              TEXT NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending','running','waiting','done','failed','canceled')),
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

INSERT INTO runs_v4 (
    id,thread_id,project_id,scope_artifact,scope_level,scope_slide_id,
    intent,run_command_json,client_request_id,model_profile_name,
    model_provider,model_name,model_url,cancel_requested_at,status,created_at,updated_at
)
SELECT
    id,
    thread_id,
    project_id,
    CASE target_artifact WHEN 'presentation' THEN 'ppt' ELSE target_artifact END,
    target_level,
    target_slide_id,
    interaction_intent,
    json_object(
        'scope',
        json(
            CASE
                WHEN target_slide_id IS NOT NULL AND target_slide_id <> ''
                THEN json_object(
                    'artifact', CASE target_artifact WHEN 'presentation' THEN 'ppt' ELSE target_artifact END,
                    'level', target_level,
                    'slide_id', target_slide_id
                )
                ELSE json_object(
                    'artifact', CASE target_artifact WHEN 'presentation' THEN 'ppt' ELSE target_artifact END,
                    'level', target_level
                )
            END
        ),
        'intent', interaction_intent,
        'instruction', COALESCE(json_extract(work_spec_json, '$.instruction'), ''),
        'options',
        json(
            json_patch(
                CASE
                    WHEN COALESCE(json_extract(work_spec_json, '$.options.language'), '') <> ''
                    THEN json_object('language', json_extract(work_spec_json, '$.options.language'))
                    ELSE json_object()
                END,
                CASE
                    WHEN COALESCE(json_extract(work_spec_json, '$.options.desired_slide_count'), 0) BETWEEN 1 AND 8
                    THEN json_object('range', '5-8')
                    WHEN COALESCE(json_extract(work_spec_json, '$.options.desired_slide_count'), 0) BETWEEN 9 AND 15
                    THEN json_object('range', '9-15')
                    WHEN COALESCE(json_extract(work_spec_json, '$.options.desired_slide_count'), 0) BETWEEN 16 AND 25
                    THEN json_object('range', '16-25')
                    WHEN COALESCE(json_extract(work_spec_json, '$.options.desired_slide_count'), 0) >= 26
                    THEN json_object('range', '26+')
                    ELSE json_object()
                END
            )
        )
    ),
    client_request_id,
    model_profile_name,
    model_provider,
    model_name,
    model_url,
    cancel_requested_at,
    status,
    created_at,
    updated_at
FROM runs;

DROP TABLE runs;
ALTER TABLE runs_v4 RENAME TO runs;

CREATE INDEX idx_runs_thread ON runs(thread_id);
CREATE INDEX idx_runs_project ON runs(project_id);
CREATE INDEX idx_runs_status ON runs(status);
CREATE UNIQUE INDEX idx_runs_thread_client_request
ON runs(thread_id, client_request_id)
WHERE client_request_id IS NOT NULL AND client_request_id <> '';

PRAGMA foreign_keys = ON;
