-- Development cutover: resource payloads are initialized explicitly from seed.
DROP TABLE resource_tags;
DROP TABLE resource_states;
DROP TABLE prompts;
DELETE FROM tags WHERE scope = 'prompt';
ALTER TABLE tags RENAME TO tags_previous;
CREATE TABLE tags (
 id TEXT PRIMARY KEY,
 scope TEXT NOT NULL CHECK (scope IN ('theme','component','skill','snippet')),
 key TEXT NOT NULL,
 name TEXT NOT NULL,
 normalized_name TEXT NOT NULL,
 is_system INTEGER NOT NULL DEFAULT 0 CHECK (is_system IN (0,1)),
 sort_order INTEGER NOT NULL DEFAULT 0,
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 UNIQUE(scope,key), UNIQUE(scope,normalized_name)
);
INSERT INTO tags SELECT * FROM tags_previous;
DROP TABLE tags_previous;
INSERT INTO tags VALUES
 ('tag_snippet_identity','snippet','identity','身份','身份',1,10,0,0),
 ('tag_snippet_deliverable','snippet','deliverable','交付','交付',1,20,0,0),
 ('tag_snippet_constraint','snippet','constraint','约束','约束',1,30,0,0),
 ('tag_snippet_git','snippet','git','Git','git',1,40,0,0),
 ('tag_snippet_review','snippet','review','审查','审查',1,50,0,0),
 ('tag_snippet_other','snippet','other','其它','其它',1,60,0,0);
CREATE TABLE resources (
 type TEXT NOT NULL CHECK(type IN ('theme','component','skill','snippet')),
 id TEXT NOT NULL,
 name TEXT NOT NULL,
 normalized_name TEXT NOT NULL,
 description TEXT NOT NULL,
 disabled INTEGER NOT NULL DEFAULT 0 CHECK(disabled IN (0,1)),
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(type,id)
);
CREATE UNIQUE INDEX idx_snippet_name ON resources(normalized_name) WHERE type = 'snippet';
CREATE TABLE resource_tags (
 resource_type TEXT NOT NULL,
 resource_id TEXT NOT NULL,
 tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
 PRIMARY KEY(resource_type,resource_id,tag_id),
 FOREIGN KEY(resource_type,resource_id) REFERENCES resources(type,id) ON DELETE CASCADE
);
CREATE INDEX idx_resource_tags_tag ON resource_tags(tag_id);
