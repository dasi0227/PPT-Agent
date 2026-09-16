DROP INDEX IF EXISTS idx_versions_run_target;

CREATE INDEX idx_versions_run_target
ON versions(run_id, target_type, target_id) WHERE run_id IS NOT NULL;
