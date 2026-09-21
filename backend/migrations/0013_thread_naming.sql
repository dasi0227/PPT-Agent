ALTER TABLE threads ADD COLUMN auto_rename_enabled INTEGER NOT NULL DEFAULT 1 CHECK (auto_rename_enabled IN (0, 1));
ALTER TABLE threads ADD COLUMN naming_revision INTEGER NOT NULL DEFAULT 1 CHECK (naming_revision >= 1);
ALTER TABLE threads ADD COLUMN rename_operation_version INTEGER NOT NULL DEFAULT 1 CHECK (rename_operation_version >= 1);
ALTER TABLE threads ADD COLUMN rename_input_count INTEGER NOT NULL DEFAULT 0 CHECK (rename_input_count >= 0);
ALTER TABLE threads ADD COLUMN rename_first_input_seen INTEGER NOT NULL DEFAULT 0 CHECK (rename_first_input_seen IN (0, 1));

CREATE TABLE thread_naming_inputs (
    thread_id   TEXT NOT NULL,
    input_id    TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    accepted_at INTEGER NOT NULL,
    PRIMARY KEY (thread_id, input_id),
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);
CREATE INDEX idx_thread_naming_inputs_order ON thread_naming_inputs(thread_id, accepted_at, input_id);

CREATE TABLE thread_naming_operations (
    thread_id    TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    action       TEXT NOT NULL CHECK (action IN ('generate','manual','enable','disable')),
    status       TEXT NOT NULL CHECK (status IN ('in_progress','accepted','completed','failed')),
    request_id   TEXT NOT NULL DEFAULT '',
    result_json  TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (thread_id, operation_id),
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);
