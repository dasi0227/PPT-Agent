ALTER TABLE briefing_versions ADD COLUMN title TEXT NOT NULL DEFAULT '';
ALTER TABLE context_compactions RENAME COLUMN summary TO content;
