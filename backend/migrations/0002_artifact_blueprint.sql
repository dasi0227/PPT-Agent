ALTER TABLE projects ADD COLUMN deck_path TEXT NOT NULL DEFAULT 'deck.json';
ALTER TABLE projects ADD COLUMN deck_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN design_revision INTEGER NOT NULL DEFAULT 0;

ALTER TABLE slides ADD COLUMN blueprint_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE slides ADD COLUMN presentation_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE slides ADD COLUMN source_deck_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE slides ADD COLUMN source_blueprint_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE slides ADD COLUMN source_design_revision INTEGER NOT NULL DEFAULT 0;

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
