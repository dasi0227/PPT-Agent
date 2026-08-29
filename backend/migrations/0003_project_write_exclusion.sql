PRAGMA foreign_keys = ON;

CREATE TRIGGER IF NOT EXISTS trg_runs_reject_active_git_commit_insert
BEFORE INSERT ON runs
WHEN NEW.status NOT IN ('done','failed','canceled')
  AND EXISTS (
    SELECT 1 FROM git_commit_operations
    WHERE project_id = NEW.project_id AND status IN ('accepted','running')
  )
BEGIN
  SELECT RAISE(ABORT, 'GIT_COMMIT_ACTIVE');
END;

CREATE TRIGGER IF NOT EXISTS trg_runs_reject_active_git_commit_update
BEFORE UPDATE OF status ON runs
WHEN NEW.status NOT IN ('done','failed','canceled')
  AND EXISTS (
    SELECT 1 FROM git_commit_operations
    WHERE project_id = NEW.project_id AND status IN ('accepted','running')
  )
BEGIN
  SELECT RAISE(ABORT, 'GIT_COMMIT_ACTIVE');
END;

CREATE TRIGGER IF NOT EXISTS trg_git_commits_reject_active_run_insert
BEFORE INSERT ON git_commit_operations
WHEN NEW.status IN ('accepted','running')
  AND EXISTS (
    SELECT 1 FROM runs
    WHERE project_id = NEW.project_id AND status NOT IN ('done','failed','canceled')
  )
BEGIN
  SELECT RAISE(ABORT, 'RUN_ACTIVE');
END;

CREATE TRIGGER IF NOT EXISTS trg_git_commits_reject_active_run_update
BEFORE UPDATE OF status ON git_commit_operations
WHEN NEW.status IN ('accepted','running')
  AND EXISTS (
    SELECT 1 FROM runs
    WHERE project_id = NEW.project_id AND status NOT IN ('done','failed','canceled')
  )
BEGIN
  SELECT RAISE(ABORT, 'RUN_ACTIVE');
END;
