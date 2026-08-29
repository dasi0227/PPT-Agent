PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS git_commit_operations (
    id                TEXT PRIMARY KEY,
    project_id        TEXT NOT NULL,
    thread_id         TEXT NOT NULL,
    client_request_id TEXT NOT NULL,
    model_profile     TEXT NOT NULL,
    status            TEXT NOT NULL CHECK (status IN ('accepted','running','empty','completed','failed')),
    phase             TEXT NOT NULL DEFAULT '' CHECK (phase IN ('','staging','analyzing','committing')),
    result_json       TEXT NOT NULL DEFAULT '',
    error_json        TEXT NOT NULL DEFAULT '',
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL,
    UNIQUE (thread_id, client_request_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_git_commit_operations_project
ON git_commit_operations(project_id, created_at);

CREATE INDEX IF NOT EXISTS idx_git_commit_operations_thread
ON git_commit_operations(thread_id, created_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_git_commit_operations_active_project
ON git_commit_operations(project_id)
WHERE status IN ('accepted','running');

CREATE TABLE IF NOT EXISTS git_commit_events (
    operation_id TEXT NOT NULL,
    seq          INTEGER NOT NULL,
    type         TEXT NOT NULL,
    payload      TEXT NOT NULL,
    created_at   INTEGER NOT NULL,
    PRIMARY KEY (operation_id, seq),
    FOREIGN KEY (operation_id) REFERENCES git_commit_operations(id) ON DELETE CASCADE
);
