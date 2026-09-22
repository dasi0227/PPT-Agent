CREATE TABLE command_activities (
    id TEXT PRIMARY KEY,
    attempt_id TEXT NOT NULL,
    thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('rename','polish','kickoff','handoff','compact')),
    method TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('loading','completed','failed','canceled')),
    phase INTEGER NOT NULL DEFAULT -1,
    previous_title TEXT NOT NULL DEFAULT '',
    request TEXT NOT NULL,
    result TEXT NOT NULL DEFAULT 'null',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX idx_command_activities_thread ON command_activities(thread_id, created_at, id);
