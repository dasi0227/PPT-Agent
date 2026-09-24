CREATE TABLE shortcut_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    revision INTEGER NOT NULL DEFAULT 0,
    overrides_json TEXT NOT NULL DEFAULT '{}'
);
INSERT INTO shortcut_settings (id) VALUES (1);
