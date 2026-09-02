CREATE TABLE IF NOT EXISTS tags (
    id              TEXT PRIMARY KEY,
    scope           TEXT NOT NULL CHECK (scope IN ('theme', 'component', 'skill', 'prompt')),
    key             TEXT NOT NULL,
    name            TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    is_system       INTEGER NOT NULL DEFAULT 0 CHECK (is_system IN (0, 1)),
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    UNIQUE (scope, key),
    UNIQUE (scope, normalized_name)
);

CREATE TABLE IF NOT EXISTS resource_tags (
    resource_type TEXT NOT NULL CHECK (resource_type IN ('theme', 'component', 'skill', 'prompt')),
    resource_id   TEXT NOT NULL,
    tag_id        TEXT NOT NULL,
    PRIMARY KEY (resource_type, resource_id, tag_id),
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_resource_tags_resource
ON resource_tags(resource_type, resource_id);

CREATE TABLE IF NOT EXISTS resource_states (
    resource_type TEXT NOT NULL CHECK (resource_type IN ('component', 'skill', 'prompt')),
    resource_id   TEXT NOT NULL,
    disabled      INTEGER NOT NULL DEFAULT 0 CHECK (disabled IN (0, 1)),
    updated_at    INTEGER NOT NULL,
    PRIMARY KEY (resource_type, resource_id)
);

INSERT INTO tags(id, scope, key, name, normalized_name, is_system, sort_order, created_at, updated_at) VALUES
    ('tag_theme_minimal', 'theme', 'minimal', '极简', '极简', 1, 10, 0, 0),
    ('tag_theme_business', 'theme', 'business', '商务', '商务', 1, 20, 0, 0),
    ('tag_theme_technology', 'theme', 'technology', '科技', '科技', 1, 30, 0, 0),
    ('tag_theme_cool', 'theme', 'cool', '清冷', '清冷', 1, 40, 0, 0),
    ('tag_theme_warm', 'theme', 'warm', '温暖', '温暖', 1, 50, 0, 0),
    ('tag_theme_other', 'theme', 'other', '其它', '其它', 1, 60, 0, 0),
    ('tag_component_card', 'component', 'card', '卡片', '卡片', 1, 10, 0, 0),
    ('tag_component_chart', 'component', 'chart', '统计图', '统计图', 1, 20, 0, 0),
    ('tag_component_table', 'component', 'table', '表格', '表格', 1, 30, 0, 0),
    ('tag_component_list', 'component', 'list', '列表', '列表', 1, 40, 0, 0),
    ('tag_component_process', 'component', 'process', '流程', '流程', 1, 50, 0, 0),
    ('tag_component_metric', 'component', 'metric', '指标', '指标', 1, 60, 0, 0),
    ('tag_component_other', 'component', 'other', '其它', '其它', 1, 70, 0, 0),
    ('tag_skill_workflow', 'skill', 'workflow', '工作流', '工作流', 1, 10, 0, 0),
    ('tag_skill_methodology', 'skill', 'methodology', '方法论', '方法论', 1, 20, 0, 0),
    ('tag_skill_manual', 'skill', 'manual', '操作手册', '操作手册', 1, 30, 0, 0),
    ('tag_skill_experience', 'skill', 'experience', '开发经验', '开发经验', 1, 40, 0, 0),
    ('tag_skill_other', 'skill', 'other', '其它', '其它', 1, 50, 0, 0),
    ('tag_prompt_identity', 'prompt', 'identity', '身份', '身份', 1, 10, 0, 0),
    ('tag_prompt_deliverable', 'prompt', 'deliverable', '交付', '交付', 1, 20, 0, 0),
    ('tag_prompt_constraint', 'prompt', 'constraint', '约束', '约束', 1, 30, 0, 0),
    ('tag_prompt_git', 'prompt', 'git', 'Git', 'git', 1, 40, 0, 0),
    ('tag_prompt_review', 'prompt', 'review', '审查', '审查', 1, 50, 0, 0),
    ('tag_prompt_other', 'prompt', 'other', '其它', '其它', 1, 60, 0, 0);

INSERT INTO resource_tags(resource_type, resource_id, tag_id) VALUES
    ('theme', 'swiss-modern', 'tag_theme_minimal'),
    ('theme', 'corporate-clean', 'tag_theme_business'),
    ('theme', 'blueprint', 'tag_theme_technology'),
    ('theme', 'tokyo-night', 'tag_theme_cool'),
    ('theme', 'xiaohongshu-white', 'tag_theme_warm'),
    ('theme', 'bold-signal', 'tag_theme_other'),
    ('theme', 'editorial-serif', 'tag_theme_other'),
    ('theme', 'warm-pastel', 'tag_theme_other'),
    ('component', 'feature-card', 'tag_component_card'),
    ('component', 'svg-bar', 'tag_component_chart'),
    ('component', 'kv-list', 'tag_component_list'),
    ('component', 'stat-badge', 'tag_component_metric'),
    ('component', 'quote-block', 'tag_component_other'),
    ('skill', 'story-architect', 'tag_skill_methodology'),
    ('skill', 'executive-summary', 'tag_skill_workflow'),
    ('skill', 'visual-hierarchy-review', 'tag_skill_manual');
