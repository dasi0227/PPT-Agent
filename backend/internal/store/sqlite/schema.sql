-- The complete current development schema. There is no migration chain.
CREATE TABLE projects (
 id TEXT PRIMARY KEY, title TEXT NOT NULL, theme TEXT NOT NULL,
 decorations_json TEXT NOT NULL DEFAULT '{"slide_pagination":"bottom-right","deck_title":"none","section_title":"top-left","logo":"none"}' CHECK(json_valid(decorations_json)),
 logo_json TEXT CHECK(logo_json IS NULL OR json_valid(logo_json)),
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE slides (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX idx_slides_project ON slides(project_id);
CREATE TABLE threads (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 auto_suggest_enabled INTEGER NOT NULL DEFAULT 1 CHECK(auto_suggest_enabled IN (0,1)),
 suggest_version INTEGER NOT NULL DEFAULT 1,
 title TEXT NOT NULL DEFAULT '', auto_rename_enabled INTEGER NOT NULL DEFAULT 1 CHECK(auto_rename_enabled IN (0,1)),
 naming_revision INTEGER NOT NULL DEFAULT 1, rename_operation_version INTEGER NOT NULL DEFAULT 1,
 rename_input_count INTEGER NOT NULL DEFAULT 0, rename_first_input_seen INTEGER NOT NULL DEFAULT 0,
 last_seq INTEGER NOT NULL DEFAULT 0 CHECK(last_seq >= 0),
 retained_after_seq INTEGER NOT NULL DEFAULT 0 CHECK(retained_after_seq >= 0 AND retained_after_seq <= last_seq),
 context_revision INTEGER NOT NULL DEFAULT 0 CHECK(context_revision >= 0),
 active_run_id TEXT, last_context_run_id TEXT,
 plan_json TEXT CHECK(plan_json IS NULL OR json_valid(plan_json)), plan_run_id TEXT,
 plan_revision INTEGER NOT NULL DEFAULT 0, next_input_suggestions_json TEXT CHECK(next_input_suggestions_json IS NULL OR json_valid(next_input_suggestions_json)),
 entity_revision INTEGER NOT NULL DEFAULT 1, updated_seq INTEGER NOT NULL DEFAULT 0,
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE INDEX idx_threads_project ON threads(project_id);
CREATE TABLE runs (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 scene_revision INTEGER NOT NULL CHECK(scene_revision >= 1),
 input_id TEXT NOT NULL, accepted_seq INTEGER NOT NULL, client_request_id TEXT NOT NULL DEFAULT '',
 command_json TEXT NOT NULL CHECK(json_valid(command_json)),
 context_revision INTEGER NOT NULL DEFAULT 0,
 usage_json TEXT CHECK(usage_json IS NULL OR json_valid(usage_json)),
 terminal_json TEXT CHECK(terminal_json IS NULL OR json_valid(terminal_json)),
 terminal_ref TEXT,
 progress_json TEXT CHECK(progress_json IS NULL OR json_valid(progress_json)),
 context_window_json TEXT CHECK(context_window_json IS NULL OR json_valid(context_window_json)),
 final_message_id TEXT,
 active_duration_ms INTEGER NOT NULL DEFAULT 0, active_since INTEGER, cancel_request_id TEXT,
 cancel_reason TEXT CHECK(cancel_reason IS NULL OR cancel_reason IN ('user_requested','superseded','input_rejected','input_detection_failed')),
 continuation_allowed INTEGER NOT NULL DEFAULT 0 CHECK(continuation_allowed IN (0,1)),
 entity_revision INTEGER NOT NULL DEFAULT 1, updated_seq INTEGER NOT NULL DEFAULT 0,
 scope_json TEXT NOT NULL CHECK(json_valid(scope_json)), scope_revision INTEGER NOT NULL CHECK(scope_revision >= 1),
 mode TEXT NOT NULL CHECK(mode IN ('chat','ask','plan','create')),
 model_profile_name TEXT, model_provider TEXT, model_name TEXT, model_url TEXT,
 cancel_requested_at INTEGER, owner_instance_id TEXT NOT NULL DEFAULT '', execution_revision INTEGER NOT NULL DEFAULT 1,
 pause_reason TEXT NOT NULL DEFAULT '', paused_at INTEGER,
 status TEXT NOT NULL CHECK(status IN ('pending','running','waiting','paused','recovering','done','failed','canceled')),
 checkpoint_json TEXT CHECK(checkpoint_json IS NULL OR json_valid(checkpoint_json)),
 checkpoint_revision INTEGER NOT NULL DEFAULT 0 CHECK(checkpoint_revision >= 0),
 checkpoint_execution_revision INTEGER NOT NULL DEFAULT 0,
 checkpoint_boundary TEXT NOT NULL DEFAULT '', checkpoint_phase TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE INDEX idx_runs_thread ON runs(thread_id,accepted_seq);
CREATE INDEX idx_runs_project ON runs(project_id);
CREATE INDEX idx_runs_status ON runs(status);
CREATE UNIQUE INDEX idx_runs_active_project ON runs(project_id) WHERE status NOT IN ('done','failed','canceled');
CREATE TABLE steering_inbox (
 run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
 thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 client_message_id TEXT NOT NULL, request_hash TEXT NOT NULL,
 input_id TEXT NOT NULL REFERENCES thread_inputs(id), accepted_seq INTEGER NOT NULL,
 rejection_code TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL CHECK(status IN ('accepted','injected','rejected')),
 PRIMARY KEY(thread_id,client_message_id)
);
CREATE INDEX idx_steering_pending ON steering_inbox(run_id,status,accepted_seq);
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
 kind TEXT NOT NULL CHECK(kind IN ('rename','polish','handoff','compact','commit','suggest')),
 source TEXT NOT NULL CHECK(source IN ('user','automatic')),
 status TEXT NOT NULL CHECK(status IN ('accepted','running','cancel_requested','completed','failed','canceled','interrupted')),
 phase TEXT NOT NULL DEFAULT '', owner_instance_id TEXT NOT NULL DEFAULT '',
 execution_revision INTEGER NOT NULL DEFAULT 1, scene_revision INTEGER NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
 error_json TEXT CHECK(error_json IS NULL OR json_valid(error_json)),
 previous_title TEXT NOT NULL DEFAULT '', run_id TEXT,
 model_json TEXT CHECK(model_json IS NULL OR json_valid(model_json)),
 operation_json TEXT CHECK(operation_json IS NULL OR json_valid(operation_json)),
 entity_revision INTEGER NOT NULL DEFAULT 1, updated_seq INTEGER NOT NULL DEFAULT 0,
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 UNIQUE(command_id,attempt_no)
);
CREATE UNIQUE INDEX idx_command_active ON command_executions(command_id)
 WHERE status IN ('accepted','running','cancel_requested');
CREATE UNIQUE INDEX idx_commit_active_project ON command_executions(project_id)
 WHERE kind='commit' AND status IN ('accepted','running','cancel_requested');
CREATE INDEX idx_commands_thread ON command_executions(thread_id,created_at);
CREATE INDEX idx_commands_thread_active ON command_executions(thread_id,created_at,id)
 WHERE status IN ('accepted','running','cancel_requested');
CREATE INDEX idx_commands_success ON command_executions(command_id,status,attempt_no);
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

CREATE TABLE file_settings (
 id INTEGER PRIMARY KEY CHECK(id = 1),
 default_open TEXT NOT NULL DEFAULT '{"open_with":"system","custom_app_path":""}' CHECK(json_valid(default_open) AND json_type(default_open) = 'object'),
 json_open TEXT NOT NULL DEFAULT '{"open_with":"inherit","custom_app_path":""}' CHECK(json_valid(json_open) AND json_type(json_open) = 'object'),
 html_open TEXT NOT NULL DEFAULT '{"open_with":"inherit","custom_app_path":""}' CHECK(json_valid(html_open) AND json_type(html_open) = 'object'),
 custom_apps TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(custom_apps) AND json_type(custom_apps) = 'array'),
 revision INTEGER NOT NULL DEFAULT 0 CHECK(revision >= 0)
);
INSERT INTO file_settings(id) VALUES (1);


-- Current project scene control is never restored from an old business snapshot.
CREATE TABLE project_history_state (
 project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
 scene_revision INTEGER NOT NULL DEFAULT 1 CHECK(scene_revision >= 1),
 history_revision INTEGER NOT NULL DEFAULT 1 CHECK(history_revision >= 1),
 switch_state TEXT NOT NULL DEFAULT 'ready' CHECK(switch_state IN ('ready','restoring')),
 switch_operation_id TEXT, last_operation_id TEXT, last_command TEXT,
	 pending_operation_json TEXT CHECK(pending_operation_json IS NULL OR json_valid(pending_operation_json)),
 checkpoints_json TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(checkpoints_json)),
 latest_snapshot_ref TEXT, latest_time INTEGER,
 ui_scene_json TEXT CHECK(ui_scene_json IS NULL OR json_valid(ui_scene_json))
);
CREATE TABLE conversation_rounds (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
 opened_seq INTEGER NOT NULL, closed_seq INTEGER, last_message_seq INTEGER NOT NULL,
 public_message_count INTEGER NOT NULL DEFAULT 0 CHECK(public_message_count >= 0),
 has_task_input INTEGER NOT NULL DEFAULT 0 CHECK(has_task_input IN (0,1))
);
CREATE INDEX idx_rounds_recent ON conversation_rounds(thread_id,opened_seq,id);
CREATE INDEX idx_rounds_task ON conversation_rounds(thread_id,opened_seq,id) WHERE has_task_input = 1;
CREATE UNIQUE INDEX idx_rounds_open_run ON conversation_rounds(run_id) WHERE closed_seq IS NULL;
CREATE TABLE conversation_messages (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 run_id TEXT, input_id TEXT, round_id TEXT NOT NULL,
 created_seq INTEGER NOT NULL, role TEXT NOT NULL CHECK(role IN ('user','assistant','tool')),
 kind TEXT NOT NULL CHECK(kind IN ('instruction','assistant','tool_result','summary','direct_reply')),
 native_json TEXT NOT NULL CHECK(json_valid(native_json)),
 public_json TEXT CHECK(public_json IS NULL OR json_valid(public_json)),
 source_message_ids_json TEXT CHECK(source_message_ids_json IS NULL OR json_valid(source_message_ids_json)),
 created_at INTEGER NOT NULL
);
CREATE INDEX idx_messages_thread_seq ON conversation_messages(thread_id,created_seq,id);
CREATE INDEX idx_messages_round ON conversation_messages(thread_id,round_id,created_seq,id);
CREATE INDEX idx_messages_run ON conversation_messages(run_id,id);
CREATE TABLE thread_context_versions (
 thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 revision INTEGER NOT NULL CHECK(revision >= 0),
 message_ids_json TEXT NOT NULL CHECK(json_valid(message_ids_json) AND json_type(message_ids_json) = 'array'),
 created_seq INTEGER NOT NULL, created_at INTEGER NOT NULL,
 reason TEXT NOT NULL CHECK(reason IN ('append','compact','replace')),
 PRIMARY KEY(thread_id,revision)
);
CREATE TABLE thread_inputs (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 client_message_id TEXT NOT NULL, message_id TEXT NOT NULL REFERENCES conversation_messages(id),
 request_hash TEXT NOT NULL, run_id TEXT, source_run_id TEXT,
 accepted_seq INTEGER NOT NULL, history_before_seq INTEGER NOT NULL,
 disposition TEXT NOT NULL DEFAULT 'pending' CHECK(disposition IN ('pending','task','chat','disclosure','bypass','irrelevant','detection_failed')),
 decision_json TEXT CHECK(decision_json IS NULL OR json_valid(decision_json)),
 delivery_state TEXT NOT NULL CHECK(delivery_state IN ('accepted','injected','rejected','transferred')),
 reply_message_id TEXT REFERENCES conversation_messages(id),
 reference_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(reference_json)),
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 UNIQUE(thread_id,client_message_id)
);
CREATE INDEX idx_inputs_run ON thread_inputs(run_id,accepted_seq);
CREATE INDEX idx_inputs_history ON thread_inputs(thread_id,accepted_seq);
CREATE TABLE model_request_snapshots (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
 scene_revision INTEGER NOT NULL,
 context_revision INTEGER NOT NULL, provider TEXT NOT NULL, model TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 response_json TEXT CHECK(response_json IS NULL OR json_valid(response_json)),
 created_seq INTEGER NOT NULL, created_at INTEGER NOT NULL
);
CREATE INDEX idx_model_requests_run ON model_request_snapshots(run_id);
CREATE TABLE tool_executions (
 id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
 call_id TEXT NOT NULL, attempt_no INTEGER NOT NULL CHECK(attempt_no >= 1),
 thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 scene_revision INTEGER NOT NULL, execution_revision INTEGER NOT NULL,
 tool_name TEXT NOT NULL, arguments_json TEXT NOT NULL CHECK(json_valid(arguments_json)), arguments_hash TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('prepared','executing','succeeded','failed','canceled','outcome_unknown')),
 recovery_policy TEXT NOT NULL CHECK(recovery_policy IN ('read_only','idempotent','reconcile','unknown')),
 operation_id TEXT, result_json TEXT CHECK(result_json IS NULL OR json_valid(result_json)),
 result_message_id TEXT REFERENCES conversation_messages(id), error_json TEXT CHECK(error_json IS NULL OR json_valid(error_json)),
 started_at INTEGER, finished_at INTEGER,
 entity_revision INTEGER NOT NULL DEFAULT 1, updated_seq INTEGER NOT NULL,
 UNIQUE(run_id,call_id,attempt_no)
);
CREATE INDEX idx_tools_pending ON tool_executions(run_id,status);
CREATE INDEX idx_tools_operation ON tool_executions(run_id,operation_id,attempt_no) WHERE operation_id IS NOT NULL;
CREATE TABLE run_interactions (
 id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
 thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('question','plan','resource','command','scope')),
 call_id TEXT NOT NULL, request_json TEXT NOT NULL CHECK(json_valid(request_json)), request_revision INTEGER NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('pending','answered','applied','canceled')),
 answer_json TEXT CHECK(answer_json IS NULL OR json_valid(answer_json)), answer_hash TEXT,
 candidate_json TEXT CHECK(candidate_json IS NULL OR json_valid(candidate_json)),
 answered_seq INTEGER, applied_seq INTEGER,
 entity_revision INTEGER NOT NULL DEFAULT 1, updated_seq INTEGER NOT NULL,
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 CHECK((kind = 'resource' AND candidate_json IS NOT NULL) OR (kind <> 'resource' AND candidate_json IS NULL))
);
CREATE INDEX idx_interactions_run_state ON run_interactions(run_id,state);
CREATE INDEX idx_interactions_thread_state ON run_interactions(thread_id,state);
CREATE TABLE briefing_versions (
 briefing_id TEXT NOT NULL, version_no INTEGER NOT NULL,
 thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 title TEXT NOT NULL, content TEXT NOT NULL, feedback TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL, PRIMARY KEY(briefing_id,version_no)
);
CREATE INDEX idx_briefings_thread ON briefing_versions(thread_id,created_at);
CREATE TABLE context_compactions (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 run_id TEXT, before_revision INTEGER NOT NULL, after_revision INTEGER NOT NULL,
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), created_at INTEGER NOT NULL
);
CREATE INDEX idx_compactions_thread ON context_compactions(thread_id,created_at);
CREATE TABLE timeline_groups (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 order_seq INTEGER NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('run','command','exchange')),
 run_id TEXT, command_id TEXT, input_id TEXT,
 status TEXT NOT NULL, summary_json TEXT NOT NULL CHECK(json_valid(summary_json)),
 item_count INTEGER NOT NULL DEFAULT 0, entity_revision INTEGER NOT NULL DEFAULT 1,
 created_seq INTEGER NOT NULL, updated_seq INTEGER NOT NULL
);
CREATE INDEX idx_timeline_groups_order ON timeline_groups(thread_id,order_seq,id);
CREATE UNIQUE INDEX idx_timeline_run_group ON timeline_groups(thread_id,run_id) WHERE kind='run';
CREATE UNIQUE INDEX idx_timeline_command_group ON timeline_groups(thread_id,command_id) WHERE kind='command';
CREATE TABLE timeline_details (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 public_json TEXT NOT NULL CHECK(json_valid(public_json)), created_seq INTEGER NOT NULL
);
CREATE TABLE timeline_items (
 id TEXT PRIMARY KEY, thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 group_id TEXT NOT NULL REFERENCES timeline_groups(id) ON DELETE CASCADE,
 position INTEGER NOT NULL, kind TEXT NOT NULL, entity_revision INTEGER NOT NULL DEFAULT 1,
 message_id TEXT REFERENCES conversation_messages(id), tool_call_id TEXT, interaction_id TEXT, attempt_id TEXT,
 summary_json TEXT NOT NULL CHECK(json_valid(summary_json)), body_ref TEXT, detail_ref TEXT,
 created_seq INTEGER NOT NULL, updated_seq INTEGER NOT NULL, created_at INTEGER NOT NULL
);
CREATE INDEX idx_timeline_items_group ON timeline_items(group_id,position,id);
CREATE INDEX idx_timeline_items_kind ON timeline_items(group_id,kind,position,id);
CREATE INDEX idx_timeline_items_updated ON timeline_items(thread_id,updated_seq);
CREATE UNIQUE INDEX idx_timeline_message ON timeline_items(thread_id,message_id) WHERE message_id IS NOT NULL;
CREATE UNIQUE INDEX idx_timeline_interaction ON timeline_items(thread_id,interaction_id) WHERE interaction_id IS NOT NULL;
CREATE TABLE thread_changes (
 thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
 scene_revision INTEGER NOT NULL, seq INTEGER NOT NULL CHECK(seq >= 1), commit_id TEXT NOT NULL UNIQUE,
 payload_json TEXT NOT NULL CHECK(json_valid(payload_json)), created_at INTEGER NOT NULL,
 PRIMARY KEY(thread_id,scene_revision,seq)
);
CREATE INDEX idx_changes_retention ON thread_changes(created_at);
