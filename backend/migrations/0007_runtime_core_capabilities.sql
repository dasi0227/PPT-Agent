CREATE TABLE IF NOT EXISTS run_checkpoints (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    loop_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    phase TEXT NOT NULL,
    checkpoint_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE(run_id, seq),
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_run_checkpoints_run_seq
ON run_checkpoints(run_id, seq DESC);

CREATE TABLE IF NOT EXISTS context_index_snapshots (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    pack_hash TEXT NOT NULL,
    index_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_context_index_snapshots_run_created
ON context_index_snapshots(run_id, created_at DESC);

CREATE TABLE IF NOT EXISTS semantic_reviews (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    finish_call_id TEXT NOT NULL,
    accepted INTEGER NOT NULL,
    confidence REAL NOT NULL,
    input_hash TEXT NOT NULL,
    output_json TEXT NOT NULL,
    prompt_manifest_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_semantic_reviews_run_created
ON semantic_reviews(run_id, created_at DESC);
