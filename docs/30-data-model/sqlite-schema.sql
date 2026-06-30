-- AI PPT Builder — SQLite schema
-- 机器可校验产物：sqlite3 :memory: < this file 应无错误建表成功。
-- 关联：docs/30-data-model/data-model.md（DATA-MODEL-001 等）
-- 约定：时间戳用 INTEGER（unix 秒）；主键用 TEXT(uuid)；启用外键。

PRAGMA foreign_keys = ON;

-- ───────────────────────── Project ─────────────────────────
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT    PRIMARY KEY,
    title       TEXT    NOT NULL,
    work_dir    TEXT    NOT NULL,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- ───────────────────────── Deck ─────────────────────────
CREATE TABLE IF NOT EXISTS decks (
    id                 TEXT    PRIMARY KEY,
    project_id         TEXT    NOT NULL,
    theme              TEXT    NOT NULL DEFAULT 'default',
    status             TEXT    NOT NULL DEFAULT 'draft'
                               CHECK (status IN ('draft','generating','ready')),
    common_style_path  TEXT    NOT NULL,
    created_at         INTEGER NOT NULL,
    updated_at         INTEGER NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_decks_project ON decks(project_id);

-- ───────────────────────── Slide ─────────────────────────
CREATE TABLE IF NOT EXISTS slides (
    id               TEXT    PRIMARY KEY,
    deck_id          TEXT    NOT NULL,
    idx              INTEGER NOT NULL,
    layout           TEXT    NOT NULL,
    title            TEXT    NOT NULL DEFAULT '',
    json_path        TEXT    NOT NULL,
    html_path        TEXT    NOT NULL,
    current_version  INTEGER NOT NULL DEFAULT 0,
    last_export_at   INTEGER,                       -- backlog：图片导出预留，可空
    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE,
    UNIQUE (deck_id, idx)
);
CREATE INDEX IF NOT EXISTS idx_slides_deck ON slides(deck_id);

-- ───────────────────────── Version ─────────────────────────
-- 通用版本表：target_type 区分 slide / deck / common_style 快照
CREATE TABLE IF NOT EXISTS versions (
    id            TEXT    PRIMARY KEY,
    target_type   TEXT    NOT NULL
                          CHECK (target_type IN ('slide','deck','common_style')),
    target_id     TEXT    NOT NULL,
    version_no    INTEGER NOT NULL,
    snapshot_path TEXT    NOT NULL,
    run_id        TEXT,
    created_at    INTEGER NOT NULL,
    UNIQUE (target_type, target_id, version_no)
);
CREATE INDEX IF NOT EXISTS idx_versions_target ON versions(target_type, target_id);

-- ───────────────────────── Run ─────────────────────────
CREATE TABLE IF NOT EXISTS runs (
    id          TEXT    PRIMARY KEY,
    project_id  TEXT    NOT NULL,
    kind        TEXT    NOT NULL
                        CHECK (kind IN ('outline','generate','edit','command')),
    scope       TEXT    NOT NULL DEFAULT 'deck'
                        CHECK (scope IN ('page','overview','deck')),
    page_index  INTEGER,
    mode        TEXT    NOT NULL DEFAULT 'normal'
                        CHECK (mode IN ('normal','talk','ask')),
    status      TEXT    NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','running','waiting','done','failed','canceled')),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_runs_project ON runs(project_id);
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);

-- ───────────────────────── Run 事件（可选持久化，用于断线重连续传） ─────────────────────────
CREATE TABLE IF NOT EXISTS run_events (
    run_id     TEXT    NOT NULL,
    seq        INTEGER NOT NULL,
    type       TEXT    NOT NULL,
    payload    TEXT    NOT NULL,          -- JSON 文本
    created_at INTEGER NOT NULL,
    PRIMARY KEY (run_id, seq),
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);

-- ───────────────────────── Plugin（个人仓库，全局） ─────────────────────────
CREATE TABLE IF NOT EXISTS plugins (
    id            TEXT    PRIMARY KEY,
    name          TEXT    NOT NULL,
    kind          TEXT    NOT NULL CHECK (kind IN ('style','fx')),
    manifest_path TEXT    NOT NULL,
    dir           TEXT    NOT NULL,
    created_at    INTEGER NOT NULL,
    UNIQUE (name)
);
