DELETE FROM resource_tags WHERE resource_type = 'prompt';
DELETE FROM resource_states WHERE resource_type = 'prompt';

DROP TABLE IF EXISTS prompts;

CREATE TABLE prompts (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    normalized_name TEXT NOT NULL UNIQUE,
    "desc"          TEXT NOT NULL,
    value           TEXT NOT NULL,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE INDEX idx_prompts_updated_at
ON prompts(updated_at DESC, id);
