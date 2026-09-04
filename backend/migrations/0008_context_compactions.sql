PRAGMA foreign_keys = ON;

CREATE TABLE context_compactions (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    run_id TEXT NOT NULL DEFAULT '',
    trigger TEXT NOT NULL CHECK (trigger IN ('auto', 'manual')),
    summary TEXT NOT NULL,
    before_tokens INTEGER NOT NULL CHECK (before_tokens >= 0),
    after_tokens INTEGER NOT NULL CHECK (after_tokens >= 0),
    max_tokens INTEGER NOT NULL CHECK (max_tokens > 0),
    reclaimed_tokens INTEGER NOT NULL CHECK (reclaimed_tokens >= 0),
    duration_ms INTEGER NOT NULL CHECK (duration_ms >= 0),
    created_at INTEGER NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX idx_context_compactions_thread
ON context_compactions(thread_id, created_at, id);
