PRAGMA foreign_keys = ON;

CREATE TABLE briefing_versions (
    briefing_id TEXT NOT NULL,
    thread_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('kickoff', 'handoff')),
    version_no INTEGER NOT NULL CHECK (version_no > 0),
    content TEXT NOT NULL,
    feedback TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    PRIMARY KEY (briefing_id, version_no),
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX idx_briefing_versions_thread
ON briefing_versions(thread_id, created_at, briefing_id, version_no);

CREATE INDEX idx_briefing_versions_project
ON briefing_versions(project_id, created_at);
