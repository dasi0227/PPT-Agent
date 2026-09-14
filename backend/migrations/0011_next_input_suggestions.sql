ALTER TABLE runs ADD COLUMN project_history_revision INTEGER NOT NULL DEFAULT 1 CHECK (project_history_revision >= 1);
