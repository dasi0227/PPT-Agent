-- Initial v6 development schema. Later schema changes are applied by the
-- subsequent numbered migrations.
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS projects (
    id             TEXT PRIMARY KEY,
    title          TEXT NOT NULL,
    work_dir       TEXT NOT NULL,
    theme          TEXT NOT NULL DEFAULT 'default',
    status         TEXT NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft','generating','ready')),
    layout_version INTEGER NOT NULL DEFAULT 6 CHECK (layout_version = 6),
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS slides (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL,
    current_version INTEGER NOT NULL DEFAULT 0,
    last_export_at  INTEGER,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_slides_project ON slides(project_id);

CREATE TABLE IF NOT EXISTS versions (
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
CREATE INDEX IF NOT EXISTS idx_versions_target ON versions(target_type, target_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_versions_run_target
ON versions(run_id, target_type, target_id) WHERE run_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS threads (
    id           TEXT PRIMARY KEY,
    project_id   TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    history_path TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_threads_project ON threads(project_id);

CREATE TABLE IF NOT EXISTS runs (
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
    status              TEXT NOT NULL DEFAULT 'pending'
                           CHECK (status IN ('pending','running','waiting','done','failed','canceled')),
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_runs_thread ON runs(thread_id);
CREATE INDEX IF NOT EXISTS idx_runs_project ON runs(project_id);
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_runs_thread_client_request
ON runs(thread_id, client_request_id)
WHERE client_request_id IS NOT NULL AND client_request_id <> '';

CREATE TABLE IF NOT EXISTS run_events (
    run_id     TEXT NOT NULL,
    seq        INTEGER NOT NULL,
    type       TEXT NOT NULL,
    payload    TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (run_id, seq),
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS run_contexts (
    run_id           TEXT PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE,
    context_id       TEXT UNIQUE NOT NULL,
    profile          TEXT NOT NULL,
    pack_hash        TEXT NOT NULL,
    estimated_tokens INTEGER NOT NULL,
    budget_tokens    INTEGER NOT NULL,
    manifest_json    TEXT NOT NULL,
    created_at       INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_run_contexts_context_id ON run_contexts(context_id);

CREATE TABLE IF NOT EXISTS assets (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    kind          TEXT NOT NULL CHECK (kind IN ('layout','component','theme','fx')),
    version       TEXT NOT NULL DEFAULT '1.0.0',
    source        TEXT NOT NULL DEFAULT 'user' CHECK (source IN ('preset','user')),
    description   TEXT NOT NULL DEFAULT '',
    tags          TEXT NOT NULL DEFAULT '[]',
    manifest_path TEXT NOT NULL,
    dir           TEXT NOT NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    UNIQUE (name, kind)
);
CREATE INDEX IF NOT EXISTS idx_assets_kind ON assets(kind);
CREATE INDEX IF NOT EXISTS idx_assets_source ON assets(source);

CREATE TABLE IF NOT EXISTS idempotency_records (
    scope        TEXT NOT NULL,
    owner_id     TEXT NOT NULL,
    key          TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status       TEXT NOT NULL CHECK (status IN ('in_progress','completed','failed')),
    result_json  TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (scope, owner_id, key)
);
CREATE INDEX IF NOT EXISTS idx_idempotency_updated
ON idempotency_records(status, updated_at);

CREATE TABLE IF NOT EXISTS steering_inbox (
    run_id            TEXT NOT NULL,
    thread_id         TEXT NOT NULL,
    client_message_id TEXT NOT NULL,
    request_hash      TEXT NOT NULL,
    content           TEXT NOT NULL,
    status            TEXT NOT NULL CHECK (status IN ('accepted','injected','rejected')),
    accepted_at       INTEGER NOT NULL,
    injected_at       INTEGER,
    rejection_code    TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (thread_id, client_message_id),
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_steering_run_delivery
ON steering_inbox(run_id, status, accepted_at, client_message_id);

CREATE TABLE IF NOT EXISTS run_checkpoints (
    id              TEXT PRIMARY KEY,
    run_id          TEXT NOT NULL,
    loop_id         TEXT NOT NULL,
    seq             INTEGER NOT NULL,
    phase           TEXT NOT NULL,
    checkpoint_json TEXT NOT NULL,
    created_at      INTEGER NOT NULL,
    UNIQUE(run_id, seq),
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_run_checkpoints_run_seq
ON run_checkpoints(run_id, seq DESC);

CREATE TABLE IF NOT EXISTS context_index_snapshots (
    id         TEXT PRIMARY KEY,
    run_id     TEXT NOT NULL,
    pack_hash  TEXT NOT NULL,
    index_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_context_index_snapshots_run_created
ON context_index_snapshots(run_id, created_at DESC);

CREATE TABLE IF NOT EXISTS semantic_reviews (
    id                   TEXT PRIMARY KEY,
    run_id               TEXT NOT NULL,
    finish_call_id       TEXT NOT NULL,
    accepted             INTEGER NOT NULL,
    confidence           REAL NOT NULL,
    input_hash           TEXT NOT NULL,
    output_json          TEXT NOT NULL,
    prompt_manifest_json TEXT NOT NULL,
    created_at           INTEGER NOT NULL,
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_semantic_reviews_run_created
ON semantic_reviews(run_id, created_at DESC);
