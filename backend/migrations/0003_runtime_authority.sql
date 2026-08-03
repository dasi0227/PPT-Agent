ALTER TABLE runs ADD COLUMN client_request_id TEXT;
ALTER TABLE runs ADD COLUMN cancel_requested_at INTEGER;

CREATE UNIQUE INDEX IF NOT EXISTS idx_runs_thread_client_request
ON runs(thread_id, client_request_id)
WHERE client_request_id IS NOT NULL AND client_request_id <> '';

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

CREATE UNIQUE INDEX IF NOT EXISTS idx_versions_run_target
ON versions(run_id, target_type, target_id)
WHERE run_id IS NOT NULL;
