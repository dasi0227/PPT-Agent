# Deck Navigator Redesign Design

## 背景

当前左侧目录同时展示了页面顺序、章节层级、页面标题、layout/archetype 机器字段、HTML 物化状态和操作按钮。信息密度过高，且 `hero-cover`、`problem-statement`、`未生成` 这类内部状态或设计稿细节会干扰用户理解目录。

本设计以 `tmp/deck-navigator-redesign-preview.html` 为视觉方向，落地到现有 React 前端。

## 目标

1. 目录只展示用户理解结构需要的信息。
2. 保留页面重排和删除能力。
3. 对已有 HTML 的页面展示低分辨率缩略图。
4. 对没有 HTML 的页面显示中性占位“暂无”。
5. 不在目录中展示 MaterializationBadge、layout/archetype、key message、summary 等设计稿细节。

## 数据来源

目录仍使用现有两个状态源：

- `slidesByProjectId[projectId]`：来自 `/projects/:id/slides`，提供页面顺序、`slide.id`、`slide.title`、HTML 路径和修订游标。
- `specView`：来自 `/projects/:id/spec`，提供 `outline.sections`、`slide_specs` 和 `materialization`。

渲染关系：

```text
Project
└── slides: Slide[]
    └── slide.id
        ├── slide_specs[slide.id] -> section_id / subsection_id
        ├── outline.sections[] -> section title
        └── outline.sections[].subsections[] -> subsection title
```

## 信息架构

每个目录块按当前数据能力分为三层：

- 一级标题：`section.number + section.title`
- 二级标题：`subsection.number + subsection.title`
- 页面行：页码、缩略图、页面标题、操作按钮

当前领域模型没有三级 outline 标题；如果后续增加三级标题，应沿用 dotted number 格式，例如 `1.1.1`。当前实现只格式化已有 section/subsection。

## 编号规则

目录标题使用自然 dotted number：

- `01 引入与定义` -> `1. 引入与定义`
- `1.1 为什么需要 Skill` 保持 `1.1 为什么需要 Skill`

前端只做展示规范化，不修改后端数据。

## 页面行布局

页面行使用四列：

```text
页码 | 缩略图/暂无 | 页面标题 | 操作按钮
```

- 页码固定在最左侧第一列，使用 `index + 1`。
- 缩略图列不再展示 `slide.layout` 或 `spec.visual_intent.archetype`。
- 页面标题使用 `slide.title`，兜底 `spec.title`，再兜底“未命名”。
- 操作按钮保留：上移、下移、删除。

## HTML 缩略图

复用现有预览机制：

- `hasRenderedHTML(slide)` 判断是否有可加载 HTML。
- `useSlideRenderCache(projectId)` 加载并缓存 HTML。
- `IsolatedSlidePreview` 在缩略图容器内低分辨率展示。

没有 HTML、加载中或加载失败时，缩略图位置显示“暂无”。目录不负责解释物化状态。

## 明确移除

- 移除 `MaterializationBadge`。
- 移除 `slide.layout` 白色方框。
- 不展示 `visual_intent.archetype`、`key_message`、`content.summary`、`content.points`。
- 不在目录行展示“未生成 HTML”“设计稿有更新”等状态文案。

## 结构化移动

目录不再按 `slides` 线性扫描后插入章节标题，而是先构造树：

```text
section
  directSlides
  subsection
    slides
```

页面允许没有 `subsection_id`。这表示它是该 section 的直属页，不是脏数据。拖动规则：

- 拖到某个页面行：插入到该页面前方，并继承目标页面的 `section_id/subsection_id`。
- 拖到 section 标题：移动为该 section 的直属页。
- 拖到 subsection 标题：移动到该 subsection 下。
- 上移/下移按钮只在当前直属组或当前 subsection 内移动，不跨组，不隐式改变归属。

后端使用结构化接口：

```json
{
  "ordered_ids": ["slide-01", "slide-02"],
  "placements": [
    {"slide_id": "slide-01", "section_id": "section-intro"},
    {"slide_id": "slide-02", "section_id": "section-intro", "subsection_id": "sub-why"}
  ]
}
```

后端校验：

- `ordered_ids` 必须完整且无重复。
- `placements` 必须完整且无重复。
- `section_id` 必须存在。
- `subsection_id` 如果存在，必须属于该 `section_id`。
- 活跃 run 时拒绝手动重组。

## 验收标准

1. 目录不再出现 `未生成` badge。
2. 目录不再出现 `cover`、`hero-cover`、`problem-statement` 等 layout/archetype 文案。
3. 页码显示在每行最左侧。
4. 一级章节显示为 `1. 标题`，二级章节显示为 `1.1 标题`。
5. 有 HTML 的页面渲染低分辨率缩略图；没有 HTML 的页面显示“暂无”。
6. 上移、下移、删除按钮仍可用。
7. 同一个 section 不因页面移动重复出现。
8. 跨 subsection 拖动会更新页面归属。
