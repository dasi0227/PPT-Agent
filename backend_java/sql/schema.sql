PRAGMA foreign_keys = ON;

CREATE TABLE assets (
    id TEXT NOT NULL PRIMARY KEY,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    content TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    updated_at INTEGER NOT NULL,
    CHECK (length(trim(name)) BETWEEN 1 AND 80),
    CHECK (length(trim(description)) BETWEEN 1 AND 500),
    CHECK (enabled IN (0, 1))
);

CREATE TABLE tags (
    id TEXT NOT NULL PRIMARY KEY,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    CHECK (length(trim(name)) > 0)
);

CREATE TABLE projects (
    id TEXT NOT NULL PRIMARY KEY,
    title TEXT NOT NULL,
    theme TEXT DEFAULT NULL REFERENCES assets(id) ON DELETE RESTRICT,
    logo BLOB DEFAULT NULL,
    slide_pagination_position TEXT NOT NULL DEFAULT 'bottom-right',
    deck_title_position TEXT NOT NULL DEFAULT 'none',
    chapter_title_position TEXT NOT NULL DEFAULT 'top-left',
    project_logo_position TEXT NOT NULL DEFAULT 'none',
    updated_at INTEGER NOT NULL,
    CHECK (slide_pagination_position = 'none' OR deck_title_position = 'none' OR slide_pagination_position <> deck_title_position),
    CHECK (slide_pagination_position = 'none' OR chapter_title_position = 'none' OR slide_pagination_position <> chapter_title_position),
    CHECK (slide_pagination_position = 'none' OR project_logo_position = 'none' OR slide_pagination_position <> project_logo_position),
    CHECK (deck_title_position = 'none' OR chapter_title_position = 'none' OR deck_title_position <> chapter_title_position),
    CHECK (deck_title_position = 'none' OR project_logo_position = 'none' OR deck_title_position <> project_logo_position),
    CHECK (chapter_title_position = 'none' OR project_logo_position = 'none' OR chapter_title_position <> project_logo_position)
);

CREATE TABLE sessions (
    id TEXT NOT NULL PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL DEFAULT 0,
    title TEXT NOT NULL DEFAULT '新会话',
    plan TEXT DEFAULT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (revision >= 0),
    CHECK (plan IS NULL OR (json_valid(plan) AND json_type(plan) = 'object'))
);

CREATE TABLE runs (
    id TEXT NOT NULL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    mode TEXT NOT NULL,
    model TEXT NOT NULL,
    scope TEXT DEFAULT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    changes TEXT DEFAULT NULL,
    error TEXT DEFAULT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    call_count INTEGER NOT NULL DEFAULT 0,
    active_duration INTEGER NOT NULL DEFAULT 0,
    active_since INTEGER DEFAULT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (input_tokens >= 0),
    CHECK (output_tokens >= 0),
    CHECK (total_tokens >= 0),
    CHECK (call_count >= 0),
    CHECK (active_duration >= 0),
    UNIQUE (id, session_id),
    CHECK (scope IS NULL OR (json_valid(scope) AND json_type(scope) = 'array')),
    CHECK (changes IS NULL OR (json_valid(changes) AND json_type(changes) = 'array')),
    CHECK (error IS NULL OR (json_valid(error) AND json_type(error) = 'object'))
);

CREATE TABLE messages (
    id TEXT NOT NULL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    run_id TEXT DEFAULT NULL,
    seq INTEGER NOT NULL,
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    raw TEXT DEFAULT NULL,
    tool_call_id TEXT DEFAULT NULL,
    instruction_decision TEXT DEFAULT NULL,
    instruction_status TEXT DEFAULT NULL,
    context_position INTEGER DEFAULT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (run_id, session_id) REFERENCES runs(id, session_id),
    CHECK (run_id IS NOT NULL OR type = 'summary'),
    CHECK (((type = 'tool_call' OR type = 'tool_result') AND tool_call_id IS NOT NULL) OR ((type <> 'tool_call' AND type <> 'tool_result') AND tool_call_id IS NULL)),
    CHECK ((type = 'instruction' AND instruction_status IS NOT NULL) OR (type <> 'instruction' AND instruction_status IS NULL AND instruction_decision IS NULL)),
    CHECK (json_valid(content)),
    CHECK (raw IS NULL OR (json_valid(raw)))
);

CREATE TABLE commands (
    id TEXT NOT NULL PRIMARY KEY,
    operation_id TEXT NOT NULL,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    run_id TEXT DEFAULT NULL,
    seq INTEGER NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    input TEXT NOT NULL,
    result TEXT DEFAULT NULL,
    error TEXT DEFAULT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (run_id, session_id) REFERENCES runs(id, session_id),
    CHECK (json_valid(input) AND json_type(input) = 'object'),
    CHECK (result IS NULL OR (json_valid(result))),
    CHECK (error IS NULL OR (json_valid(error) AND json_type(error) = 'object'))
);

CREATE TABLE checkpoints (
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    run_id TEXT DEFAULT NULL,
    snapshot TEXT NOT NULL,
    CHECK (json_valid(snapshot) AND json_type(snapshot) = 'object')
);

CREATE TABLE receipts (
    type TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    PRIMARY KEY (type, owner_id, request_id)
);

CREATE TABLE asset_tags (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, tag_id)
);

CREATE TABLE settings_fileopen (
    id INTEGER NOT NULL PRIMARY KEY DEFAULT 1,
    default_open TEXT NOT NULL DEFAULT 'system',
    json_open TEXT NOT NULL DEFAULT 'inherit',
    html_open TEXT NOT NULL DEFAULT 'inherit',
    custom_apps TEXT NOT NULL DEFAULT '[]',
    CHECK (id = 1),
    CHECK (json_valid(custom_apps) AND json_type(custom_apps) = 'array')
);

CREATE TABLE settings_shortcut (
    id INTEGER NOT NULL PRIMARY KEY DEFAULT 1,
    command_trigger TEXT NOT NULL DEFAULT '/',
    reference_trigger TEXT NOT NULL DEFAULT '@',
    component_trigger TEXT NOT NULL DEFAULT '$',
    slide_trigger TEXT NOT NULL DEFAULT '#',
    snippet_trigger TEXT NOT NULL DEFAULT '%',
    previous_slide TEXT NOT NULL DEFAULT 'Primary+ArrowLeft',
    next_slide TEXT NOT NULL DEFAULT 'Primary+ArrowRight',
    zoom_in TEXT NOT NULL DEFAULT 'Primary+Equal',
    zoom_out TEXT NOT NULL DEFAULT 'Primary+Minus',
    toggle_overview TEXT NOT NULL DEFAULT 'Primary+KeyO',
    export_deck TEXT NOT NULL DEFAULT 'Primary+KeyP',
    toggle_view TEXT NOT NULL DEFAULT 'Primary+KeyU',
    toggle_source TEXT NOT NULL DEFAULT 'Primary+KeyI',
    switch_theme TEXT NOT NULL DEFAULT 'Primary+KeyT',
    submit_request TEXT NOT NULL DEFAULT 'Primary+Enter',
    select_suggestion TEXT NOT NULL DEFAULT 'Option',
    CHECK (id = 1),
    CHECK (length(command_trigger) = 1),
    CHECK (length(reference_trigger) = 1),
    CHECK (length(component_trigger) = 1),
    CHECK (length(slide_trigger) = 1),
    CHECK (length(snippet_trigger) = 1),
    CHECK (command_trigger <> reference_trigger),
    CHECK (command_trigger <> component_trigger),
    CHECK (command_trigger <> slide_trigger),
    CHECK (command_trigger <> snippet_trigger),
    CHECK (reference_trigger <> component_trigger),
    CHECK (reference_trigger <> slide_trigger),
    CHECK (reference_trigger <> snippet_trigger),
    CHECK (component_trigger <> slide_trigger),
    CHECK (component_trigger <> snippet_trigger),
    CHECK (slide_trigger <> snippet_trigger)
);

CREATE TABLE settings_models (
    id TEXT NOT NULL PRIMARY KEY CHECK (length(trim(id)) > 0),
    name TEXT NOT NULL UNIQUE,
    protocol TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('deepseek', 'kimi', 'mimo', 'gpt', 'claude', 'gemini', 'qwen', 'minimax', 'glm', 'grok')),
    base_url TEXT NOT NULL,
    model TEXT NOT NULL,
    key TEXT NOT NULL,
    CHECK (length(trim(name)) BETWEEN 1 AND 80),
    CHECK (length(trim(base_url)) > 0),
    CHECK (length(trim(model)) > 0),
    CHECK (length(trim(key)) > 0)
);

CREATE TABLE settings_route (
    id INTEGER NOT NULL PRIMARY KEY DEFAULT 1,
    main_default TEXT NOT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    main_fallback TEXT DEFAULT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    side_default TEXT NOT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    side_fallback TEXT DEFAULT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    rename TEXT DEFAULT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    compact TEXT DEFAULT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    "commit" TEXT DEFAULT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    polish TEXT DEFAULT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    handoff TEXT DEFAULT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    suggest TEXT DEFAULT NULL REFERENCES settings_models(id) DEFERRABLE INITIALLY DEFERRED,
    CHECK (id = 1)
);

CREATE TABLE settings_behavior (
    id INTEGER NOT NULL PRIMARY KEY DEFAULT 1,
    auto_rename_enabled INTEGER NOT NULL DEFAULT 1,
    auto_suggest_enabled INTEGER NOT NULL DEFAULT 1,
    show_tool_failures INTEGER NOT NULL DEFAULT 0,
    require_resource_approval INTEGER NOT NULL DEFAULT 0,
    require_high_risk_approval INTEGER NOT NULL DEFAULT 1,
    CHECK (id = 1),
    CHECK (auto_rename_enabled IN (0, 1)),
    CHECK (auto_suggest_enabled IN (0, 1)),
    CHECK (show_tool_failures IN (0, 1)),
    CHECK (require_resource_approval IN (0, 1)),
    CHECK (require_high_risk_approval IN (0, 1))
);

CREATE INDEX idx_projects_theme ON projects(theme);
CREATE INDEX idx_sessions_project ON sessions(project_id);
CREATE INDEX idx_runs_session ON runs(session_id);
CREATE UNIQUE INDEX idx_runs_active_session ON runs(session_id)
    WHERE status IN ('pending', 'running', 'waiting', 'cancelling');
CREATE UNIQUE INDEX idx_messages_session_seq ON messages(session_id, seq);
CREATE INDEX idx_messages_run ON messages(run_id, seq);
CREATE UNIQUE INDEX idx_messages_response ON messages(run_id)
    WHERE run_id IS NOT NULL AND type = 'response';
CREATE UNIQUE INDEX idx_messages_tool ON messages(run_id, tool_call_id, type)
    WHERE type IN ('tool_call', 'tool_result');
CREATE UNIQUE INDEX idx_messages_context ON messages(session_id, context_position)
    WHERE context_position IS NOT NULL;
CREATE UNIQUE INDEX idx_commands_session_seq ON commands(session_id, seq);
CREATE INDEX idx_commands_run ON commands(run_id);
CREATE INDEX idx_commands_operation ON commands(operation_id, seq);
CREATE UNIQUE INDEX idx_commands_active_operation ON commands(operation_id)
    WHERE status IN ('pending', 'running', 'cancelling');
CREATE UNIQUE INDEX idx_commands_manual_compact ON commands(session_id)
    WHERE type = 'compact' AND run_id IS NULL AND status IN ('pending', 'running', 'cancelling');
CREATE UNIQUE INDEX idx_checkpoints_run ON checkpoints(session_id, run_id)
    WHERE run_id IS NOT NULL;
CREATE UNIQUE INDEX idx_checkpoints_return ON checkpoints(session_id)
    WHERE run_id IS NULL;
CREATE INDEX idx_asset_tags_tag ON asset_tags(tag_id);
