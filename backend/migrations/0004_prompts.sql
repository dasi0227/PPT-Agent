CREATE TABLE IF NOT EXISTS prompts (
    id                TEXT PRIMARY KEY,
    key_zh            TEXT NOT NULL UNIQUE,
    key_en            TEXT NOT NULL,
    normalized_key_en TEXT NOT NULL UNIQUE,
    value             TEXT NOT NULL,
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_prompts_updated_at
ON prompts(updated_at DESC, id);
