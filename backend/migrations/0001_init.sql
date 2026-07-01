-- AI PPT Builder — 初始 schema 迁移
-- 权威源：docs/30-data-model/sqlite-schema.sql（本文件与其保持同步）。
-- 校验：sqlite3 :memory: < 本文件 应无错误建表成功。
-- 约定：时间戳用 INTEGER（unix 秒）；主键用 TEXT(uuid)；启用外键。

PRAGMA foreign_keys = ON;

-- ───────────────────────── Project（合并原 Deck：一 project 恰一份演示文稿，1:1 无需拆表） ─────────────────────────
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT    PRIMARY KEY,
    title       TEXT    NOT NULL,
    work_dir    TEXT    NOT NULL,
    theme       TEXT    NOT NULL DEFAULT 'default',
    status      TEXT    NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft','generating','ready')),
    design_path TEXT    NOT NULL DEFAULT '',        -- 公共样式层（design tokens）文件相对路径
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- ───────────────────────── Slide ─────────────────────────
CREATE TABLE IF NOT EXISTS slides (
    id               TEXT    PRIMARY KEY,
    project_id       TEXT    NOT NULL,
    idx              INTEGER NOT NULL,
    layout           TEXT    NOT NULL,
    title            TEXT    NOT NULL DEFAULT '',
    json_path        TEXT    NOT NULL,
    html_path        TEXT    NOT NULL,
    current_version  INTEGER NOT NULL DEFAULT 0,
    last_export_at   INTEGER,                       -- backlog：图片导出预留，可空
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE (project_id, idx)
);
CREATE INDEX IF NOT EXISTS idx_slides_project ON slides(project_id);

-- ───────────────────────── Version ─────────────────────────
-- 通用版本表：target_type 区分 slide / project / design / asset 快照
CREATE TABLE IF NOT EXISTS versions (
    id            TEXT    PRIMARY KEY,
    target_type   TEXT    NOT NULL
                          CHECK (target_type IN ('slide','project','design','asset')),
    target_id     TEXT    NOT NULL,
    version_no    INTEGER NOT NULL,
    snapshot_path TEXT    NOT NULL,
    run_id        TEXT,
    created_at    INTEGER NOT NULL,
    UNIQUE (target_type, target_id, version_no)
);
CREATE INDEX IF NOT EXISTS idx_versions_target ON versions(target_type, target_id);

-- ───────────────────────── Thread（对话线程，一 project 多 thread，共享产物） ─────────────────────────
CREATE TABLE IF NOT EXISTS threads (
    id            TEXT    PRIMARY KEY,
    project_id    TEXT    NOT NULL,
    title         TEXT    NOT NULL DEFAULT '',
    history_path  TEXT    NOT NULL,                -- threads/<id>.jsonl
    status        TEXT    NOT NULL DEFAULT 'active'
                          CHECK (status IN ('active','archived')),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_threads_project ON threads(project_id);

-- ───────────────────────── Run ─────────────────────────
CREATE TABLE IF NOT EXISTS runs (
    id          TEXT    PRIMARY KEY,
    thread_id   TEXT,                          -- 挂在某对话线程下（repo 类快操作可 NULL）
    project_id  TEXT,                          -- 冗余便于按项目查询/加锁；repo scope 可 NULL
    kind        TEXT    NOT NULL
                        CHECK (kind IN ('outline','generate','edit','command')),
    scope       TEXT    NOT NULL DEFAULT 'current'
                        CHECK (scope IN ('current','page','overview','repo')),
    page_index  INTEGER,
    mode        TEXT    NOT NULL DEFAULT 'normal'
                        CHECK (mode IN ('normal','talk','ask')),
    command     TEXT,                          -- 显式指令名：prompt/recap/talk/ask 等
    status      TEXT    NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','running','waiting','done','failed','canceled')),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_runs_thread ON runs(thread_id);
CREATE INDEX IF NOT EXISTS idx_runs_project ON runs(project_id);
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);

-- ───────────────────────── Run 事件（持久化，用于断线重连续传 + harness 可观测） ─────────────────────────
-- type 含 harness 事件：thought / tool_call / tool_result / progress / token / artifact / needs_input / info / done / error
CREATE TABLE IF NOT EXISTS run_events (
    run_id     TEXT    NOT NULL,
    seq        INTEGER NOT NULL,
    type       TEXT    NOT NULL,
    payload    TEXT    NOT NULL,          -- JSON 文本
    created_at INTEGER NOT NULL,
    PRIMARY KEY (run_id, seq),
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

-- ───────────────────────── Asset（个人仓库统一资产，全局） ─────────────────────────
-- 四类资产共享统一信封；预置(preset)与用户新增(user)同表，载荷存文件系统
CREATE TABLE IF NOT EXISTS assets (
    id            TEXT    PRIMARY KEY,
    name          TEXT    NOT NULL,
    kind          TEXT    NOT NULL CHECK (kind IN ('layout','component','theme','fx')),
    version       TEXT    NOT NULL DEFAULT '1.0.0',
    source        TEXT    NOT NULL DEFAULT 'user' CHECK (source IN ('preset','user')),
    description   TEXT    NOT NULL DEFAULT '',
    tags          TEXT    NOT NULL DEFAULT '[]',  -- JSON 数组文本
    manifest_path TEXT    NOT NULL,
    dir           TEXT    NOT NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    UNIQUE (name, kind)
);
CREATE INDEX IF NOT EXISTS idx_assets_kind ON assets(kind);
CREATE INDEX IF NOT EXISTS idx_assets_source ON assets(source);
