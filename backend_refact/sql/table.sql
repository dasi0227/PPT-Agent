-- Schema source: docs/note/DATABASE.html (2026-11-10), including HTML comments.
-- Enable PRAGMA foreign_keys = ON on the connection BEFORE starting a transaction.
-- No transaction boundary here: table.sql and init.sql may share one caller transaction.
-- First creation: run table.sql + init.sql once. Normal startup does not run init.sql.
-- Explicit preset reset: run init.sql only; never clear business tables or recreate schema.
--
-- Backend write transactions must enforce cross-table/business rules:
-- * projects.theme refers to a theme; asset/tag kinds are stable and associations
--   have matching kinds, at most two tags per asset. Trim/Unicode-lowercase names
--   for snippet uniqueness and per-kind tag uniqueness (SQLite NOCASE is insufficient).
-- * Run/project/session ownership, command group session/kind and revision consistency;
--   allocate immutable seq jointly across messages/commands, including same-batch inserts.
-- * Match tool_result to its same-Run call; validate kind-specific JSON payloads,
--   plan steps (at most one processing), scope and original-request hash equality.
-- * Commit final response with done/rejected; execute tools while current output
--   contains calls. Chitchat is rejected; concurrent follow-up policy remains deferred.
-- * Validate checkpoint Run ownership on creation, preserve checkpoints when Runs are
--   removed, and coordinate history/file restoration, invalidation and revisions.
-- * receipts.owner_id is polymorphic: validate/clean session or Run owners and result
--   targets by kind in the same transaction; no static FK can express this relation.
-- * Validate custom-app membership/normalized paths, supported punctuation, shortcut
--   key/modifier/reserved-combination/conflict rules and atomic full-settings updates.
-- Run-linked messages/commands must be removed/restored in FK-safe order during history
-- switching; checkpoints.run_id deliberately has NO Run foreign key.


CREATE TABLE assets (
    id TEXT NOT NULL PRIMARY KEY,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    content TEXT NOT NULL,
    disabled INTEGER NOT NULL DEFAULT 0,
    CHECK (kind IN ('theme', 'component', 'skill', 'snippet')),
    CHECK (length(trim(name)) BETWEEN 1 AND 80),
    CHECK (length(trim(description)) BETWEEN 1 AND 500),
    CHECK (disabled IN (0, 1))
);

CREATE TABLE projects (
    id TEXT NOT NULL PRIMARY KEY,
    title TEXT NOT NULL,
    theme TEXT NOT NULL REFERENCES assets(id) ON DELETE RESTRICT,
    logo BLOB DEFAULT NULL,
    slide_pagination_position TEXT NOT NULL DEFAULT 'bottom-right',
    deck_title_position TEXT NOT NULL DEFAULT 'none',
    section_title_position TEXT NOT NULL DEFAULT 'top-left',
    project_logo_position TEXT NOT NULL DEFAULT 'none',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (slide_pagination_position IN ('none', 'top-left', 'top-center', 'top-right', 'bottom-left', 'bottom-center', 'bottom-right')),
    CHECK (deck_title_position IN ('none', 'top-left', 'top-center', 'top-right', 'bottom-left', 'bottom-center', 'bottom-right')),
    CHECK (section_title_position IN ('none', 'top-left', 'top-center', 'top-right', 'bottom-left', 'bottom-center', 'bottom-right')),
    CHECK (project_logo_position IN ('none', 'top-left', 'top-center', 'top-right', 'bottom-left', 'bottom-center', 'bottom-right')),
    CHECK (slide_pagination_position = 'none' OR deck_title_position = 'none' OR slide_pagination_position <> deck_title_position),
    CHECK (slide_pagination_position = 'none' OR section_title_position = 'none' OR slide_pagination_position <> section_title_position),
    CHECK (slide_pagination_position = 'none' OR project_logo_position = 'none' OR slide_pagination_position <> project_logo_position),
    CHECK (deck_title_position = 'none' OR section_title_position = 'none' OR deck_title_position <> section_title_position),
    CHECK (deck_title_position = 'none' OR project_logo_position = 'none' OR deck_title_position <> project_logo_position),
    CHECK (section_title_position = 'none' OR project_logo_position = 'none' OR section_title_position <> project_logo_position)
);

CREATE TABLE sessions (
    id TEXT NOT NULL PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL DEFAULT 0,
    title TEXT NOT NULL DEFAULT '新会话',
    plan TEXT DEFAULT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK (revision >= 0),
    CHECK (plan IS NULL OR (json_valid(plan) AND json_type(plan) = 'object'))
);

CREATE TABLE runs (
    id TEXT NOT NULL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
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
    CHECK (mode IN ('chat', 'ask', 'plan', 'create')),
    CHECK (status IN ('pending', 'running', 'waiting', 'cancelling', 'done', 'rejected', 'failed', 'interrupted', 'cancelled')),
    CHECK (revision >= 0),
    CHECK (input_tokens >= 0),
    CHECK (output_tokens >= 0),
    CHECK (total_tokens >= 0),
    CHECK (call_count >= 0),
    CHECK (active_duration >= 0),
    CHECK (scope IS NULL OR (json_valid(scope) AND json_type(scope) = 'array')),
    CHECK (changes IS NULL OR (json_valid(changes) AND json_type(changes) = 'array')),
    CHECK (error IS NULL OR (json_valid(error) AND json_type(error) = 'object'))
);

CREATE TABLE messages (
    id TEXT NOT NULL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    run_id TEXT DEFAULT NULL REFERENCES runs(id),
    revision INTEGER NOT NULL,
    seq INTEGER NOT NULL,
    kind TEXT NOT NULL,
    content TEXT NOT NULL,
    raw TEXT DEFAULT NULL,
    tool_call_id TEXT DEFAULT NULL,
    instruction_decision TEXT DEFAULT NULL,
    instruction_status TEXT DEFAULT NULL,
    context_position INTEGER DEFAULT NULL,
    created_at INTEGER NOT NULL,
    CHECK (kind IN ('instruction', 'response', 'thinking', 'commentary', 'tool_call', 'tool_result', 'summary')),
    CHECK (instruction_decision IN ('task', 'chat', 'disclosure', 'bypass', 'irrelevant')),
    CHECK (instruction_status IN ('queued', 'sent')),
    CHECK ((kind IN ('tool_call', 'tool_result') AND tool_call_id IS NOT NULL AND run_id IS NOT NULL) OR (kind NOT IN ('tool_call', 'tool_result') AND tool_call_id IS NULL)),
    CHECK ((kind = 'instruction' AND instruction_status IS NOT NULL) OR (kind <> 'instruction' AND instruction_status IS NULL AND instruction_decision IS NULL)),
    CHECK (revision >= 0),
    UNIQUE (session_id, seq),
    CHECK (json_valid(content)),
    CHECK (raw IS NULL OR (json_valid(raw)))
);

CREATE TABLE commands (
    id TEXT NOT NULL PRIMARY KEY,
    command_id TEXT NOT NULL,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    run_id TEXT DEFAULT NULL REFERENCES runs(id),
    revision INTEGER NOT NULL,
    seq INTEGER NOT NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    input TEXT NOT NULL,
    result TEXT DEFAULT NULL,
    error TEXT DEFAULT NULL,
    created_at INTEGER NOT NULL,
    CHECK (kind IN ('rename', 'polish', 'handoff', 'compact', 'commit', 'suggest')),
    CHECK (status IN ('pending', 'running', 'cancelling', 'done', 'failed', 'interrupted', 'cancelled')),
    CHECK (revision >= 0),
    UNIQUE (session_id, seq),
    CHECK (json_valid(input)),
    CHECK (result IS NULL OR (json_valid(result))),
    CHECK (error IS NULL OR (json_valid(error)))
);

CREATE TABLE checkpoints (
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    run_id TEXT DEFAULT NULL,
    snapshot TEXT NOT NULL,
    CHECK (json_valid(snapshot) AND json_type(snapshot) = 'object')
);

CREATE TABLE receipts (
    kind TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    result TEXT DEFAULT NULL,
    CHECK (kind IN ('create_run', 'create_command', 'commit_files')),
    PRIMARY KEY (kind, owner_id, request_id),
    CHECK (CASE kind
        WHEN 'commit_files' THEN result IS NULL
        ELSE CASE WHEN json_valid(result) THEN
            json_type(result) = 'object' AND CASE kind
                WHEN 'create_run' THEN COALESCE(json_type(result, '$.run_id') = 'text', 0)
                WHEN 'create_command' THEN COALESCE(json_type(result, '$.execution_id') = 'text', 0)
            END
        ELSE 0 END
    END)
);

CREATE TABLE tags (
    id TEXT NOT NULL PRIMARY KEY,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    CHECK (kind IN ('theme', 'component', 'skill', 'snippet')),
    CHECK (length(trim(name)) > 0)
);

CREATE TABLE asset_tags (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, tag_id)
);

CREATE TABLE fileopen (
    id INTEGER NOT NULL PRIMARY KEY DEFAULT 1,
    default_open TEXT NOT NULL DEFAULT 'system',
    json_open TEXT NOT NULL DEFAULT 'inherit',
    html_open TEXT NOT NULL DEFAULT 'inherit',
    custom_apps TEXT NOT NULL DEFAULT '[]',
    CHECK (json_valid(custom_apps) AND json_type(custom_apps) = 'array'),
    CHECK (id = 1),
    CHECK (default_open IN ('system', 'finder', 'textedit', 'vscode') OR (default_open GLOB 'app:/*' AND length(default_open) > 5)),
    CHECK (json_open IN ('system', 'finder', 'textedit', 'vscode', 'inherit') OR (json_open GLOB 'app:/*' AND length(json_open) > 5)),
    CHECK (html_open IN ('system', 'finder', 'textedit', 'vscode', 'inherit') OR (html_open GLOB 'app:/*' AND length(html_open) > 5))
);

CREATE TABLE shortcuts (
    id INTEGER NOT NULL PRIMARY KEY DEFAULT 1,
    command_trigger TEXT NOT NULL DEFAULT '/',
    reference_trigger TEXT NOT NULL DEFAULT '@',
    component_trigger TEXT NOT NULL DEFAULT '$',
    page_trigger TEXT NOT NULL DEFAULT '#',
    snippet_trigger TEXT NOT NULL DEFAULT '%',
    previous_page TEXT NOT NULL DEFAULT 'Primary+ArrowLeft',
    next_page TEXT NOT NULL DEFAULT 'Primary+ArrowRight',
    zoom_in TEXT NOT NULL DEFAULT 'Primary+Equal',
    zoom_out TEXT NOT NULL DEFAULT 'Primary+Minus',
    toggle_overview TEXT NOT NULL DEFAULT 'Primary+KeyO',
    export_deck TEXT NOT NULL DEFAULT 'Primary+KeyP',
    toggle_view TEXT NOT NULL DEFAULT 'Primary+KeyU',
    toggle_source TEXT NOT NULL DEFAULT 'Primary+KeyI',
    switch_theme TEXT NOT NULL DEFAULT 'Primary+KeyT',
    submit_request TEXT NOT NULL DEFAULT 'Primary+Enter',
    suggestion_1 TEXT NOT NULL DEFAULT 'Alt+Digit1',
    suggestion_2 TEXT NOT NULL DEFAULT 'Alt+Digit2',
    suggestion_3 TEXT NOT NULL DEFAULT 'Alt+Digit3',
    CHECK (id = 1),
    CHECK (length(command_trigger) = 1),
    CHECK (length(reference_trigger) = 1),
    CHECK (length(component_trigger) = 1),
    CHECK (length(page_trigger) = 1),
    CHECK (length(snippet_trigger) = 1),
    CHECK (command_trigger <> reference_trigger),
    CHECK (command_trigger <> component_trigger),
    CHECK (command_trigger <> page_trigger),
    CHECK (command_trigger <> snippet_trigger),
    CHECK (reference_trigger <> component_trigger),
    CHECK (reference_trigger <> page_trigger),
    CHECK (reference_trigger <> snippet_trigger),
    CHECK (component_trigger <> page_trigger),
    CHECK (component_trigger <> snippet_trigger),
    CHECK (page_trigger <> snippet_trigger)
);

CREATE INDEX idx_projects_theme ON projects(theme);
CREATE INDEX idx_sessions_project ON sessions(project_id);
CREATE INDEX idx_runs_session ON runs(session_id);
CREATE INDEX idx_runs_project ON runs(project_id);
CREATE UNIQUE INDEX idx_runs_active_project ON runs(project_id)
    WHERE status IN ('pending', 'running', 'waiting', 'cancelling');
CREATE INDEX idx_messages_run ON messages(run_id, seq);
CREATE UNIQUE INDEX idx_messages_response ON messages(run_id)
    WHERE run_id IS NOT NULL AND kind = 'response';
CREATE UNIQUE INDEX idx_messages_tool ON messages(run_id, tool_call_id, kind)
    WHERE kind IN ('tool_call', 'tool_result');
CREATE UNIQUE INDEX idx_messages_context ON messages(session_id, context_position)
    WHERE context_position IS NOT NULL;
CREATE INDEX idx_commands_run ON commands(run_id);
CREATE INDEX idx_commands_group ON commands(command_id, seq);
CREATE UNIQUE INDEX idx_commands_active_group ON commands(command_id)
    WHERE status IN ('pending', 'running', 'cancelling');
CREATE UNIQUE INDEX idx_checkpoints_run ON checkpoints(session_id, run_id)
    WHERE run_id IS NOT NULL;
CREATE UNIQUE INDEX idx_checkpoints_return ON checkpoints(session_id)
    WHERE run_id IS NULL;
CREATE INDEX idx_asset_tags_tag ON asset_tags(tag_id);
