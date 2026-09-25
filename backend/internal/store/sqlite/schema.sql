-- The complete current development schema. There is no migration chain.
CREATE TABLE projects (
 id TEXT PRIMARY KEY, title TEXT NOT NULL, theme TEXT NOT NULL,
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE slides (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 generation_inputs_json TEXT
);
CREATE INDEX idx_slides_project ON slides(project_id);
CREATE TABLE threads (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 title TEXT NOT NULL DEFAULT '', auto_rename_enabled INTEGER NOT NULL DEFAULT 1 CHECK(auto_rename_enabled IN (0,1)),
 naming_revision INTEGER NOT NULL DEFAULT 1, rename_operation_version INTEGER NOT NULL DEFAULT 1,
 rename_input_count INTEGER NOT NULL DEFAULT 0, rename_first_input_seen INTEGER NOT NULL DEFAULT 0,
 next_event_seq INTEGER NOT NULL DEFAULT 1 CHECK(next_event_seq >= 1),
 delivered_event_seq INTEGER NOT NULL DEFAULT 0 CHECK(delivered_event_seq >= 0 AND delivered_event_seq < next_event_seq),
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE INDEX idx_threads_project ON threads(project_id);
CREATE TABLE runs (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 start_event_seq INTEGER NOT NULL CHECK(start_event_seq >= 1), scope_json TEXT NOT NULL CHECK(json_valid(scope_json)), scope_revision INTEGER NOT NULL CHECK(scope_revision >= 1),
 mode TEXT NOT NULL CHECK(mode IN ('chat','grill','plan','execute')),
 model_profile_name TEXT, model_provider TEXT, model_name TEXT, model_url TEXT,
 cancel_requested_at INTEGER, owner_instance_id TEXT NOT NULL DEFAULT '', execution_revision INTEGER NOT NULL DEFAULT 1,
 pause_reason TEXT NOT NULL DEFAULT '', paused_at INTEGER,
 status TEXT NOT NULL CHECK(status IN ('pending','running','waiting','paused','recovering','done','failed','canceled')),
 checkpoint_json TEXT CHECK(checkpoint_json IS NULL OR json_valid(checkpoint_json)),
 checkpoint_revision INTEGER NOT NULL DEFAULT 0 CHECK(checkpoint_revision >= 0),
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE INDEX idx_runs_thread ON runs(thread_id);
CREATE INDEX idx_runs_project ON runs(project_id);
CREATE INDEX idx_runs_status ON runs(status);
CREATE UNIQUE INDEX idx_runs_active_project ON runs(project_id) WHERE status NOT IN ('done','failed','canceled');
CREATE TABLE steering_inbox (
 run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
 thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 client_message_id TEXT NOT NULL, request_hash TEXT NOT NULL,
 input_event_seq INTEGER NOT NULL, result_event_seq INTEGER,
 status TEXT NOT NULL CHECK(status IN ('accepted','injected','rejected')),
 PRIMARY KEY(thread_id,client_message_id)
);
CREATE INDEX idx_steering_pending ON steering_inbox(run_id,status,input_event_seq);
CREATE TABLE idempotency_records (
 scope TEXT NOT NULL, owner_id TEXT NOT NULL, key TEXT NOT NULL, request_hash TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('in_progress','completed','failed')),
 result_json TEXT NOT NULL DEFAULT '', PRIMARY KEY(scope,owner_id,key)
);
CREATE TABLE resources (
 type TEXT NOT NULL CHECK(type IN ('theme','component','skill','snippet')), id TEXT NOT NULL,
 name TEXT NOT NULL, normalized_name TEXT NOT NULL, description TEXT NOT NULL,
 disabled INTEGER NOT NULL DEFAULT 0 CHECK(disabled IN (0,1)),
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, PRIMARY KEY(type,id)
);
CREATE UNIQUE INDEX idx_snippet_name ON resources(normalized_name) WHERE type='snippet';
CREATE TABLE tags (
 id TEXT PRIMARY KEY, scope TEXT NOT NULL CHECK(scope IN ('theme','component','skill','snippet')),
 key TEXT NOT NULL, name TEXT NOT NULL, normalized_name TEXT NOT NULL,
 is_system INTEGER NOT NULL CHECK(is_system IN (0,1)), sort_order INTEGER NOT NULL,
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 UNIQUE(scope,key), UNIQUE(scope,normalized_name)
);
CREATE TABLE resource_tags (
 resource_type TEXT NOT NULL, resource_id TEXT NOT NULL, tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
 PRIMARY KEY(resource_type,resource_id,tag_id),
 FOREIGN KEY(resource_type,resource_id) REFERENCES resources(type,id) ON DELETE CASCADE
);
CREATE INDEX idx_resource_tags_tag ON resource_tags(tag_id);
CREATE TRIGGER resource_tag_scope BEFORE INSERT ON resource_tags
 WHEN NOT EXISTS(SELECT 1 FROM tags WHERE id=NEW.tag_id AND scope=NEW.resource_type)
 BEGIN SELECT RAISE(ABORT,'RESOURCE_TAG_SCOPE'); END;
CREATE TABLE shortcut_settings (
 id INTEGER PRIMARY KEY CHECK(id=1), revision INTEGER NOT NULL DEFAULT 0,
 overrides_json TEXT NOT NULL DEFAULT '{}'
);
INSERT INTO shortcut_settings(id) VALUES(1);
CREATE TABLE command_executions (
 id TEXT PRIMARY KEY, command_id TEXT NOT NULL, attempt_no INTEGER NOT NULL CHECK(attempt_no >= 1),
 thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('rename','polish','kickoff','handoff','compact','commit')),
 source TEXT NOT NULL CHECK(source IN ('user','automatic')),
 status TEXT NOT NULL CHECK(status IN ('accepted','running','cancel_requested','completed','failed','canceled','interrupted')),
 phase TEXT NOT NULL DEFAULT '', owner_instance_id TEXT NOT NULL DEFAULT '',
 execution_revision INTEGER NOT NULL DEFAULT 1, scene_revision INTEGER NOT NULL,
 start_event_seq INTEGER NOT NULL, terminal_event_seq INTEGER,
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 UNIQUE(command_id,attempt_no)
);
CREATE UNIQUE INDEX idx_command_active ON command_executions(command_id)
 WHERE status IN ('accepted','running','cancel_requested');
CREATE UNIQUE INDEX idx_commit_active_project ON command_executions(project_id)
 WHERE kind='commit' AND status IN ('accepted','running','cancel_requested');
CREATE INDEX idx_commands_thread ON command_executions(thread_id,created_at);
CREATE TABLE thread_event_outbox (
 thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 seq INTEGER NOT NULL CHECK(seq >= 1), event_id TEXT NOT NULL UNIQUE, event_json TEXT NOT NULL CHECK(json_valid(event_json)),
 PRIMARY KEY(thread_id,seq)
);
CREATE TRIGGER run_reject_commit_insert BEFORE INSERT ON runs
 WHEN NEW.status NOT IN ('done','failed','canceled') AND EXISTS(
 SELECT 1 FROM command_executions WHERE project_id=NEW.project_id AND kind='commit' AND status IN ('accepted','running','cancel_requested'))
 BEGIN SELECT RAISE(ABORT,'GIT_COMMIT_ACTIVE'); END;
CREATE TRIGGER run_reject_commit_update BEFORE UPDATE OF status ON runs
 WHEN NEW.status NOT IN ('done','failed','canceled') AND EXISTS(
 SELECT 1 FROM command_executions WHERE project_id=NEW.project_id AND kind='commit' AND status IN ('accepted','running','cancel_requested'))
 BEGIN SELECT RAISE(ABORT,'GIT_COMMIT_ACTIVE'); END;
CREATE TRIGGER commit_reject_run_insert BEFORE INSERT ON command_executions
 WHEN NEW.kind='commit' AND NEW.status IN ('accepted','running','cancel_requested') AND EXISTS(
 SELECT 1 FROM runs WHERE project_id=NEW.project_id AND status NOT IN ('done','failed','canceled'))
 BEGIN SELECT RAISE(ABORT,'RUN_ACTIVE'); END;
CREATE TRIGGER commit_reject_run_update BEFORE UPDATE OF status ON command_executions
 WHEN NEW.kind='commit' AND NEW.status IN ('accepted','running','cancel_requested') AND EXISTS(
 SELECT 1 FROM runs WHERE project_id=NEW.project_id AND status NOT IN ('done','failed','canceled'))
 BEGIN SELECT RAISE(ABORT,'RUN_ACTIVE'); END;
