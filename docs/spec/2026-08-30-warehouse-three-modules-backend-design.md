# 个人仓库三模块（theme / component / skill）后端设计

**日期：** 2026-08-30
**状态：** 前后端契约、实现参数与 UI 交互均已确认，可直接进入实现
**范围：** 个人仓库目录协议、theme / component / skill 三模块数据结构与协议、Agent 工具面、Prompt / Context 注入、HTTP 接口、持久化与 seed、前端交互契约、旧统一资产协议的废弃
**兼容策略：** 开发期直接切换，不保留旧 Schema、旧工具、旧接口、旧 `_assets` 目录或旧数据迁移分支（遵循 `AGENTS.md` 数据结构修改原则）

***

## 1. 结论

个人仓库最终收敛为**三个相互独立的模块**，砍掉旧统一资产协议中的 `fx`（动效）与 `layout`（版式，其概念并入 component）：

```text
~/.dasi/ppt/
├── assets/
│   ├── themes/
│   │   └── <theme-name>/
│   │       ├── manifest.json      # 主题元数据（CSS 无法优雅内联，单独成文件）
│   │       └── theme.css          # 完整必需 token；可选正式主题选择器覆写
│   └── components/
│       └── <component-name>/
│           └── index.html         # 自包含片段：内联 <script id="meta"> + 内联 <style> + 结构
└── skills/
    ├── registry.json              # 中央启停清单：仅记录 disabled 列表
    └── <skill-id>/
        └── SKILL.md               # frontmatter（name/description）+ 正文
```

三模块的定位与交互权限：

| 模块            | 定位                         | 元数据存储                                               | 谁来选/用                          | Agent 权限                | 加载方式                                 |
| ------------- | -------------------------- | --------------------------------------------------- | ------------------------------ | ----------------------- | ------------------------------------ |
| **theme**     | 基于完整 token、可选强风格覆写的全局视觉方案 | 独立 `manifest.json`                                  | **用户**（UI 换肤）                  | **只读**（严禁修改主题；可写普通 CSS） | 项目页面 `<link id="theme-link">` 引用仓库真源 |
| **component** | 用户沉淀的 HTML 片段样例库，供 AI 模仿套用 | **内联** `<script type="application/json" id="meta">` | Agent 具名调用                     | 只读引用（不做物理注入/占位替换）       | `load_component` 工具按名拉入上下文           |
| **skill**     | 可复用的技能说明书                  | **内联** frontmatter                                  | 用户固定选择 + Agent 动态 `load_skill` | 只读（不在线编辑；关闭的技能不暴露）      | Run 级动态 user context + 内容快照          |

核心决定一览（均为已拍板内容）：

1. **砍掉 fx 与 layout**，仓库只保留 theme / component / skill 三模块。
2. **theme 采用分层契约**：完整必需 token 是所有 Theme 的强制契约；对白名单类名（如 `.slide-stage`、`.card`、`.slide-title`）的样式覆写是强风格 Theme 的可选能力，纯 token 薄主题同样合法。
3. **theme 应用采用「引用真源」**：Runtime 管理 `<link id="theme-link" href=…>`，通过只读 HTTP 端点直接读取仓库 CSS 真源，一键换肤只切换引用；**不拷贝到项目目录**，脱离仓库即断联可接受，后续由独立「导出」功能负责内联。
4. **theme 保留在** **`design.json`，但采用字段级权限边界**：项目初始化时由 Runtime 写入默认主题；用户通过专用换肤接口修改 `design.json.theme`；Agent 严禁修改主题真源或该字段，但可以修改 `direction` / `density` / `chrome` 及页面普通 CSS。
5. **CSS 护栏仅用 Prompt 软约束**：强制 AI 使用 `var(--token)`，依赖 Prompt 约束与 AI 反思机制，**暂不引入机器静态扫描或自动修复**（弱化/移除旧 `designsystem/lint.go` 的写入拦截式护栏）。
6. **component 是参考而非插件**：预写的 HTML 片段仅作为 references，让 AI 模仿、适配性改写，**系统不做物理注入或占位符替换**；不为其设计复杂的插件协议。
7. **component 具名加载**：新增 `load_component` 工具，Agent 像用 skill 一样按名把组件 HTML 片段拉入上下文。
8. **元数据就近内联**：component（HTML `<script id="meta">`）与 skill（Markdown frontmatter）元数据内联在文件内；仅 theme 因 CSS 格式限制使用独立 `manifest.json`。
9. **三模块统一「文件夹 + 装配文件」结构**，参照 agent skills 的装配方式（牺牲极致单文件自包含，换取跨模块一致性）。
10. **skill 启停用中央** **`registry.json`**：仅维护 `disabled` 列表，扫描到的技能默认全部启用；避免在每个技能目录下新增额外状态文件，保持导入极简。
11. **彻底废弃旧统一资产协议**：删除 `_assets/` 目录、`model.Asset`、`assets` 表、`/assets` CRUD、4-kind manifest（mount/params/fx/layout/requires/cleanup）、seed 拷贝机制、contextengine 资产语义检索，不保留兼容层。
12. **本阶段不做「创意工坊」**：先把三模块的**管理、展示、使用**定下来；用户借助 Agent 创建/沉淀资产的能力后续再设计。
13. **skill 是 Run 级能力**：用户固定选择的技能在 Run 启动时强制加载，Agent 可在 Run 中通过批量 `load_skill` 动态加载其他已启用技能；加载状态只属于当前 Run，恢复时使用该 Run 的内容快照，不跨 Run 继承。
14. **component 无启停状态**：组件是被动参考资源，扫描合法即对 Agent 可用；不增加 disabled registry、启停 API 或状态 UI。
15. **文件跳转统一**：三类仓库详情和工具调用资源列表均由后端返回 `open_url`，前端统一使用 Lucide `ExternalLink`，紧跟资源名称；UI 不展示本地路径。
16. **加载结果不提供图片**：`load_component` / `load_skill` 的工具活动只展示“已加载 N 个…”和可展开的资源名称列表，不生成或返回预览图，不显示路径、kind、tags 等调试元信息。
17. **Component 图标维持 Lucide** **`Component`**：用于个人仓库导航和 `load_component` 工具活动，不引入新图标方案。

***

## 2. 当前问题（现状基线）

当前后端实现的是 ADR-0009 的「统一信封 + 4 类载荷」资产协议，与三模块目标存在结构性冲突：

- **4-kind 统一协议**：`internal/asset/manifest.go:9-14` 定义 `KindLayout/KindComponent/KindTheme/KindFx` 四类；manifest 信封含 `Mount`（append/prepend/replace/wrap）、`Params`、`Payload{HTML,CSS,JS,Tokens}`、`Requires`、`Cleanup` 等重协议字段。

- **统一元数据表**：`internal/model/asset.go` 的 `model.Asset` + `assets` 表（`migrations/0001_init.sql:107-122`，`kind CHECK IN ('layout','component','theme','fx')`）把四类资产索引在一张表里，载荷落在 `work_root/_assets/<kind_dir>/<id>/`。

- **统一 CRUD**：`internal/service/asset.go` 提供 Create/Patch/Delete/Rollback + 目录快照版本化；`internal/httpapi/asset_handler.go` + `router.go:86-91` 暴露 REST `/assets`。这对 theme（应只读）、component（新协议无 manifest/mount/params）、skill（走 registry）均不适用。

- **seed 拷贝**：`internal/asset/seeder.go` 把内嵌 `backend/seed/assets/**` 校验后拷贝到 `work_root/_assets/<kind_dir>/<name>/` 并落库；seed 源含 themes/layouts/components/fx 四类。

- **主题物化为拷贝**：`internal/spec/css.go:13-42` `DesignTokensCSS` 读 seed 内嵌 `assets/themes/<name>/tokens.css`，物化为项目本地 `common/tokens.css`；`internal/service/project.go:206-213` 在项目脚手架阶段写入。这与「link 引用真源、不拷贝」相悖。

- **主题字段权限未分离**：主题名当前存在 `design.json.theme`（`internal/spec/types.go:111`），DB `Project.Theme` 是其投影；但现有设计尚未在后端 Schema 和写路径中区分「用户控制的 theme」与「Agent 控制的 direction / density / chrome」，也未明确换肤不应使页面 HTML 失效。

- **资产语义检索**：`internal/contextengine/assembler.go` 的 `selectAssets` 按 slide spec 元素 intent 与资产 name/description/tags 子串匹配，作为 `assets://index` segment 进 user 上下文。这是为「系统预筛资产」设计的，与 component「Agent 具名 load」的新模型不符。

- **skill 无启停**：`internal/service/skill.go` 扫到的每个 `skills/<id>/SKILL.md` 一律视为启用，无 `registry.json`、无 disabled 过滤。

- **只有固定技能快照，无动态加载工具**：当前 Run 创建时最多选择 3 个技能并把正文快照注入 system prompt，但全后端仍无 `load_skill` / `load_component` 工具；现有业务工具仅 `read_ppt` / `mutate_ppt` / `render_slide`（`internal/workflow/default_tools.go`）。

- **机器护栏**：`internal/designsystem/lint.go` 的 `checkNoHardcodedTheme` 等对 slide HTML 做硬编码颜色/字号/keyframes 拦截，与「仅 Prompt 软约束」方向冲突。

本设计不在上述结构上叠加兼容层，而是直接切换到三模块协议。

***

## 3. 设计目标

### 3.1 必须实现

- 仓库根 `~/.dasi/ppt/` 下形成 `assets/themes/`、`assets/components/`、`skills/` 三个真源目录，各模块结构独立、可被独立扫描。

- theme：以包含完整必需 token、可选正式选择器覆写的 CSS 文件存储，并使用独立 `manifest.json` 保存元数据；后端在仓库读取边界校验 token 后列举主题、读取详情并提供只读 CSS。

- theme 应用：Runtime 管理 `<link id="theme-link">`，其 `href` 指向直接读取仓库真源的 HTTP CSS 端点；换肤只切换引用，不复制 CSS。

- theme 只读：后端不向 Agent 暴露任何修改主题的工具或接口；主题选择是用户 UI 行为，Agent 侧只读获得当前主题的元数据和 CSS 契约。

- component：以「文件夹 + index.html（内联 meta + 内联 CSS + 结构）」存储；后端能列举组件清单（name/description/tags）、按名读取完整 HTML 片段。

- 新增 `load_component` 工具：Agent 按名把组件 HTML 拉入上下文供模仿，系统不注入、不替换。

- skill：保留 `SKILL.md + frontmatter`、单文件 64KB 上限和最多 3 个固定技能；新增批量 `load_skill`、Run 级加载状态与恢复快照；`skills/registry.json` 维护 `disabled` 列表，禁用技能既不进固定选择列表，也不能被动态加载。

- 管理面：三模块各自提供只读的列举/详情接口；skill 额外提供启停接口；theme/component 支持正文预览，不支持在线编辑。

- 废弃：删除旧 `_assets/`、`model.Asset`、`assets` 表、`/assets` CRUD、4-kind manifest、seed 拷贝、`selectAssets`/`AssetCandidate` 检索、layout/fx seed 数据。

- `design.json.theme` 保留为主题名唯一事实来源，项目初始化时由 Runtime 写入默认值；通过字段级权限隔离用户换肤与 Agent 视觉设计。

- 换肤仍正常递增 `design.json.revision`，但页面物化来源哈希必须排除 `theme`，避免仅换肤触发 HTML 重做。

### 3.2 非目标

- 不实现「创意工坊」——即用户通过 Agent 在线创建/编辑/沉淀 theme、component、skill（后续单独设计）。

- 不为 component 设计复杂的插件协议（mount / params / 占位符注入 / 参数化渲染）。

- 不保留 fx（动效）与 layout（版式）作为独立模块。

- 不为旧 `_assets`、`model.Asset`、`assets` 表、旧 manifest、旧 `/assets` 接口保留兼容或迁移分支。

- 不引入机器静态 CSS 扫描 / 自动修复护栏（本阶段仅 Prompt 软约束）。

- 不在本设计内敲定「导出」功能（theme 内联导出）的完整实现，仅约定其为后续独立能力。

- 不支持 theme / component / skill 的在线正文编辑；skill 仅允许切换启停状态。

***

## 4. Theme 模块

### 4.1 定位与分层契约

theme 是「一系列基本元素的配色与样式方案」，采用以下分层契约：

- **Base CSS 层**：由 Runtime 提供稳定、中性的舞台结构、基础排版与公共组件默认样式，不复制到项目目录。

- **Theme Token 层**：每个 Theme 必须完整声明必需 token；纯 token 的薄主题是合法主题。

- **Theme Selector Overrides 层**：强风格 Theme 可以按需覆写正式主题选择器，但不要求每个 Theme 都包含结构选择器。

- **Page CSS 层**：页面 CSS 负责具体布局和单页特殊表达，并优先使用 Theme token。

### 4.2 存储结构

```text
assets/themes/<theme-name>/
├── manifest.json
└── theme.css
```

`manifest.json`（theme 元数据，因 CSS 无法优雅内联 JSON 而单独成文件）：

```json
{
  "name": "Tokyo Night",
  "description": "东京夜：深色冷色霓虹，适合技术分享与开发者场景",
  "tags": ["dark", "neon", "developer"]
}
```

> 说明：旧协议 manifest 中的 `kind` / `source` / `mount` / `params` / `assets` / `requires` / `cleanup` 全部移除。`kind` 由所在目录（`themes/`）隐含，无需字段。
>
> 独立 manifest 的职责只有两个：供仓库管理 UI 列举/检索主题，以及在 Run 中向 Agent 只读披露当前主题的语义信息。它不参与主题选择，也不是 CSS 载荷索引。
>
> 目录名是稳定 Theme ID（如 `tokyo-night`），`manifest.name` 是人类可读展示名（如 `Tokyo Night`），两者不得由前端互相推导。

`theme.css`（包含可选选择器覆写的示例）：

```css
:root {
  --color-bg: #1a1b26;
  --color-fg: #c0caf5;
  --color-primary: #7aa2f7;
  --color-accent: #bb9af7;
  --color-muted: #565f89;
  --font-sans: "Inter", system-ui, sans-serif;
  --text-title: 84px;
  --text-body: 32px;
  --space-4: 16px;
  --radius-md: 12px;
  --shadow-card: 0 8px 24px rgba(0, 0, 0, 0.35);
  --stage-w: 1920;
  --stage-h: 1080;
}

.slide-stage { background: var(--color-bg); color: var(--color-fg); }
.card  { border-radius: var(--radius-md); box-shadow: var(--shadow-card); }
.slide-title { font-size: var(--text-title); color: var(--color-primary); }
```

Theme 也可以只包含上述完整 `:root` token，不提供任何选择器覆写。

### 4.3 应用机制：Runtime 管理的 Base + Theme 真源

- Runtime 负责在预览/渲染文档中注入或规范化两个唯一节点，Agent 不拥有这些节点：

  - `<link id="base-link" rel="stylesheet" href="/api/v1/runtime/base.css">`

  - `<link id="theme-link" rel="stylesheet" href="/api/v1/themes/<name>/css">`

- `base.css` 是 Runtime 内嵌的主题无关结构层，保留 16:9 舞台、基础排版、正式公共组件的中性默认样式和可访问性降级；不再复制到每个项目的 `common/` 目录。

- `theme-link` 端点每次直接读取 `~/.dasi/ppt/assets/themes/<name>/theme.css`，不生成项目副本。Theme CSS 在 Base 之后加载，可覆盖允许的主题选择器。

- 换肤 = 用户 UI 修改 `design.json.theme`，Runtime 据此更新 `theme-link.href`；不要求逐页改写 Agent HTML。

- 前端 `srcdoc` 预览使用同源 `/api/v1/...` 链接。隔离 Chromium 的临时 HTTP server 必须为相同路径提供本次渲染所需的 `base.css` 与已解析 `theme.css` 内容；不得让 Chromium 回连主 API，也不得放宽外部网络拦截。

- **不拷贝** Base 或 Theme CSS 到项目目录；脱离仓库即断联可接受。「导出」时再内联两层 CSS（后续独立能力）。

- 废弃旧的 `common/tokens.css` / `common/base.css` 项目物化路径（`spec/css.go`、`service/project.go`、`ppt_mutation.go` 等）。

### 4.4 主题名事实来源与字段级权限

- **主题 ID 唯一事实来源保留在** **`design.json.theme`**（`spec/types.go:111`），值为目录 ID（如 `swiss-modern`）；因为 theme 属于项目视觉系统，不迁移到 `outline.json`。

- 项目初始化时由 Runtime 写入默认主题（当前默认 `swiss-modern`），`theme` 不允许为空。

- DB `Project.Theme` 保留为 `design.json.theme` 的读时投影/查询字段，不成为第二事实来源。

- `design.json` 内采用字段级所有权：

  - `theme`：用户控制，只能通过专用主题选择服务/API 修改。

  - `direction` / `density` / `chrome`：Agent 可通过既有设计写操作修改。

- `design.write` / `design.patch` 的 Agent 工具 Schema 不接受 `theme`；后端执行 Agent 设计写入时必须原样保留当前 `theme`，不能依赖 Prompt 自觉。

- 用户换肤会修改 `design.json.theme` 并正常递增 `design.json.revision`，以保证文件修订语义真实。

- 页面 HTML 的物化失效判断不得继续直接依赖完整 `DesignRevision` 或完整 `design.json` 原文；`MaterializationSource.DesignRevision` 改为 `DesignContentHash`，只对影响 HTML 内容的设计字段计算稳定哈希（至少包含 `direction` / `density` / `chrome`，明确排除 `theme`），`SourceHash` 也使用相同的规范化设计内容。因此仅换肤不会把所有页面标记为 stale，也不会触发 Agent 重做 HTML。

- HTML 新鲜度与视觉渲染缓存分开：换肤后 HTML 仍为 fresh，但截图、缩略图等位图产物必须把当前 `theme.css` 内容 hash 纳入缓存键并重新渲染。

### 4.5 Agent 只读边界（硬约束）

- **禁止**：Agent 不得修改仓库主题真源 CSS，不得替换/写入项目当前选中的 `design.json.theme`；该字段仅由用户换肤服务维护。

- **允许**：Agent 可以在项目页面内正常编写和修改普通 CSS（页面内 `<style>`、自定义样式、组件适配改造），只要不触碰「主题」本身。

- 后端层面：不提供任何 theme 的 Create/Patch/Delete/Rollback 工具或写接口；`mutate_ppt` 等写工具不得写主题字段。Agent 侧通过只读上下文获得当前主题的 name、元数据以及 `theme.css` 中的 token/白名单选择器契约，足以生成匹配主题的普通页面 CSS，但没有选择或写回能力。

### 4.6 护栏策略

- 仅 Prompt 软约束：在生成 CSS 时强制要求使用 `var(--token)`，依赖 Prompt 约束 + AI 反思机制遵守 token 与白名单类名约定。

- 本阶段不引入机器静态扫描或自动修复；弱化/移除旧 `designsystem/lint.go` 中对 slide HTML 的写入拦截式护栏（如 `checkNoHardcodedTheme`）。

- 必需 token 沿用现有校验基线：`--color-bg`、`--color-fg`、`--color-primary`、`--color-accent`、`--color-muted`、`--font-sans`、`--text-title`、`--text-body`、`--space-4`、`--radius-md`、`--shadow-card`、`--stage-w`、`--stage-h`。Theme Repository 在读取边界执行完整性校验，缺失任一 token 的条目无效并返回包含缺失项的诊断；列表扫描按既有策略跳过无效条目。

- Theme CSS 可覆盖的正式主题选择器为：`html`、`body`、`.slide-scaler`、`.slide-stage`、`.slide-content`、`.slide-title`、`.slide-subtitle`、`.slide-body`、`.card`、`.kicker`、`.metric`、`.metric-value`、`.metric-label`、`.quote`、`.data-table`、`a`，以及这些选择器的伪类/伪元素。Runtime chrome 不在主题覆盖范围内。

- 该选择器集合用于 Prompt、seed 测试和文档一致性，不新增对普通页面 CSS 的写入拦截。

- `designsystem/tokens.go` 的 `requiredTokens` / `LintTokens` 是 Theme Repository 的阻塞式资产校验，但不用于扫描或拦截普通页面 CSS。

***

## 5. Component 模块

### 5.1 定位

component 是「用户预先沉淀的 HTML 片段」，作为 PPT 的某个元素样例。它**不直接嵌入**主会话生成的 HTML，而是作为 **references** 让 AI 模仿学习、尽可能套用，必要时适配性改写。系统**不做物理注入或占位符替换**。旧协议的 layout（版式）概念并入 component。

### 5.2 存储结构（元数据内联）

```text
assets/components/<component-name>/
└── index.html
```

`index.html` 为自包含片段，元数据以内联 JSON block 就近声明（与 skill frontmatter 风格一致）：

```html
<script type="application/json" id="meta">
{
  "name": "能力卡片",
  "description": "特性卡片：图标 + 标题 + 描述的功能点卡片，适合能力罗列",
  "tags": ["card"]
}
</script>
<style>
  .feature-card {
    background: var(--color-bg);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-card);
    padding: var(--space-4);
  }
  .feature-card__title { font-size: var(--text-title); color: var(--color-primary); }
  .feature-card__desc  { color: var(--color-muted); }
</style>
<div class="feature-card">
  <h3 class="feature-card__title">特性标题</h3>
  <p class="feature-card__desc">特性描述文案</p>
</div>
```

约定：

- 目录名 `<component-name>` 为组件 ID（校验规则复用 skill 的 ID pattern 风格：`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`）。

- `meta.name` 是人类可读展示名，不要求等于目录 ID；API 和工具参数使用 ID，UI 使用 name。

- `meta.tags` 是多选枚举，只允许 `card/metric/comparison/quote/list/chart/process/timeline/other`；前端分别展示为卡片、指标、对比、引用、列表、图表、流程、时间线、其他。

- `meta.tags` 同时用于左侧筛选和详情展示；不保留重复的一级 `kind` 分类，也不存储 `svg/badge/number` 等实现细节。

- 内联 `<style>` 使用 `var(--token)`，以便套用当前主题（护栏同 §4.6，仅 Prompt 软约束）。

- `index.html` 单文件上限固定为 64KB。

- `meta` block 缺失或 name/description 为空的组件在列举时跳过（与 skill 校验策略一致）。

- component 不存在启停状态；合法条目始终进入管理面清单与 Agent 轻量目录。

### 5.3 加载机制：`load_component` 工具（新增）

- Agent 通过 `load_component` 按名把组件 `index.html` 全文拉入上下文，供模仿套用。

- 系统不注入、不替换、不做参数化渲染；Agent 自行在项目 HTML 中适配性改写。

- 与 skill 的 `load_skill` 对称：`load_component` 是 read 类工具，返回组件片段正文。

### 5.4 组件清单披露

- Agent 上下文中提供**轻量组件清单**（name / description / tags），供 Agent 判断何时调用 `load_component`。

- 废弃旧 `contextengine` 的 `selectAssets` 系统预筛（按 slide 元素 intent 子串匹配）——改为「清单披露 + Agent 具名主动加载」，不由系统预筛正文。

***

## 6. Skill 模块

### 6.1 保留现状

- 存储：`skills/<skill-id>/SKILL.md`，`---` frontmatter（`name` / `description` 必填）+ 正文。

- 校验：`skillIDPattern`、`maxSkillFileBytes = 64KB`、frontmatter/body 必填（`service/skill.go`）。

- 固定选择：`RunSkill{ID,Name,Description,Content,LocalPath,OpenURL}`、`MaxRunSkills = 3`（`model/run_command.go:59-68`）；Run 创建时解析并快照固定技能，保证恢复可复现。

- 位置不变：`skills/` 直接位于仓库根 `~/.dasi/ppt/skills/`，本次不迁移。

### 6.2 Run 生命周期与动态 `load_skill`

- 用户在 Run 启动前选择的固定技能必须在 Run 创建时强制加载，不需要 Agent 再调用工具。

- 新增独占的只读控制工具 `load_skill`，一次调用接受一组合法且不重复的 skill ID，支持批量加载已启用技能。

- V1 最多扫描并向管理面/Agent 目录披露 64 个合法技能；固定选择仍遵循 `MaxRunSkills = 3`。批量调用的单次上限与上下文预算在实现阶段按工具 Schema 固化，不允许无限制注入。

- 已加载技能正文进入当前 Run 的动态 user context，而不是继续扩展稳定 system prompt；正文带明确来源边界，与用户原始指令分区。

- 固定技能和动态加载技能都维护在 Run 级 active skill set 中，按 ID 去重；其正文快照随 Run 持久化，暂停/恢复后保持一致。

- active skill set 只在当前 Run 生命周期内有效，不写回项目，不被后续 Run 继承。

- Semantic Reviewer 不接收技能正文，避免技能指令影响独立审核。

- 技能在加载前必须同时通过目录校验与 `registry.json` 启用状态校验。某技能已被当前 Run 快照后，即使管理面随后将其禁用，也不追溯改变正在运行或恢复中的该 Run；禁用只阻止新的 Run 选择和新的动态加载。

### 6.3 新增：中央启停清单 `registry.json`

`skills/registry.json`：

```json
{
  "disabled": ["experimental-skill", "legacy-skill"]
}
```

规则：

- **默认全启用**：扫描到的每个合法 `skills/<id>/` 默认为启用；只有列入 `disabled` 的 ID 视为禁用。

- **不新增每目录状态文件**：状态集中在中央清单，保持导入极简（放一个技能文件夹即可用，无需额外写状态）。

- **禁用语义**：

  - `SkillService.List()` 仍可返回全部技能（供管理界面展示，附 `disabled` 标记）；

  - 暴露给 Agent 的固定选择列表与 `Resolve()` **过滤掉 disabled**，禁用技能不作为可选、不可 `load_skill`。

- **缺失语义**：`registry.json` 不存在等价于 `{"disabled":[]}`，即所有合法技能默认启用；首次切换状态时再原子创建文件。

- **损坏语义**：JSON 无法解析、结构非法或无法读取时必须返回明确错误，管理面展示异常，并停止向 Agent 披露/解析技能；不得静默按「全部启用」处理，否则可能重新暴露用户已禁用的技能。

- **写入方式**：启停更新使用同目录临时文件 + 原子 rename，避免并发或进程中断留下半写文件。

### 6.4 管理面能力

- 列举：返回全部技能（含 name/description/disabled/open\_url）。

- 正文预览：支持只读查看 SKILL.md 正文，**不允许在线编辑**。

- 启停：提供切换 `disabled` 的接口（写 `registry.json`）。

***

## 7. 后端接口与工具面

> 接口路径按项目现有 `/api/v1` 风格给出，作为落地建议；`load_component`/`load_skill` 为工具面，非 REST。

### 7.1 REST（管理与展示，只读为主）

| 方法    | 路径                           | 用途                                        | 备注                                      |
| ----- | ---------------------------- | ----------------------------------------- | --------------------------------------- |
| GET   | `/api/v1/runtime/base.css`   | 提供 Runtime 主题无关结构层                        | `text/css`，只读                           |
| GET   | `/api/v1/themes`             | 列举主题（name/description/css\_url/open\_url） | 只读；UI 不展示路径                             |
| GET   | `/api/v1/themes/:name`       | 主题详情 + CSS 预览 + open\_url                 | 只读                                      |
| GET   | `/api/v1/themes/:name/css`   | 直接提供仓库真源 `theme.css`                      | `text/css`，只读，供 `theme-link` 使用         |
| GET   | `/api/v1/components`         | 列举组件（name/description/tags/open\_url）     | 只读；无启停或 kind 字段                         |
| GET   | `/api/v1/components/:name`   | 组件详情 + index.html 预览 + open\_url          | 只读                                      |
| GET   | `/api/v1/skills`             | 列举技能（含 disabled/open\_url）                | 复用现有并扩展                                 |
| GET   | `/api/v1/skills/:id`         | 技能正文预览 + disabled/open\_url               | 只读正文                                    |
| PATCH | `/api/v1/skills/:id`         | `{ "disabled": boolean }`                 | 仅写 registry.json                        |
| PATCH | `/api/v1/projects/:id/theme` | 用户切换项目主题                                  | 校验主题存在；原子更新 `design.json.theme` 与 DB 投影 |

- **theme 仓库资源无写接口**（只读边界）。项目换肤接口与 Agent `mutate_ppt` 使用同一项目写锁，防止并发覆盖；它只切换 `design.json.theme` 引用，不修改 `/themes` 下的主题真源。

- `open_url` 使用现有 `vscode://file/...` 机制指向装配文件（Theme=`theme.css`、Component=`index.html`、Skill=`SKILL.md`）。前端只显示 `ExternalLink`，不渲染 `local_path`。

- 彻底移除旧 `/assets` 全部路由（`router.go:86-91`）与 `asset_handler.go`。

### 7.2 Agent 工具面

| 工具                                         | 类型           | 现状     | 变更                                         |
| ------------------------------------------ | ------------ | ------ | ------------------------------------------ |
| `read_ppt` / `mutate_ppt` / `render_slide` | 业务           | 已有     | 保留；`mutate_ppt` 不得写主题字段                    |
| `load_skill`                               | read/control | **新增** | 批量加载已启用技能到当前 Run active skill set，并持久化内容快照 |
| `load_component`                           | read         | **新增** | 按名加载组件 index.html 片段，供模仿；不注入               |

- 工具注册在 `internal/workflow/tools.go` 的 `ToolRegistry` + `default_tools.go`。

- `load_component` / `load_skill` 均不改变项目文件；`load_skill` 只改变当前 Run 的上下文状态。

### 7.3 文件系统安全边界

- theme / component / skill 的目录 ID 使用同一类受限字符规则，任何请求 ID 都必须先校验再拼接路径。

- 所有读取路径在 `filepath.Clean` / `EvalSymlinks` 后必须仍位于对应模块根目录内，禁止 `..`、绝对路径注入和跨模块读取。

- 模块目录、`manifest.json`、`theme.css`、`index.html`、`SKILL.md` 与 `registry.json` 均拒绝符号链接；正文文件必须是普通文件。

- 列举和加载都执行数量、单文件大小与总上下文预算限制；无效条目不能进入 Agent 可见目录。

- HTTP CSS/详情接口与 `load_component` / `load_skill` 复用同一套解析和校验服务，不能各自实现一套路径规则。

### 7.4 工具结果与公开事件投影

`load_component` / `load_skill` 的成功结果不能仅写入内部 `ToolResult.Data`，因为当前 `tool.completed` 公开事件不会透传任意 Data。新增受控的类型化投影：

```go
type LoadedResource struct {
    Kind      string `json:"kind"` // component | skill
    ID        string `json:"id"`
    Name      string `json:"name"`
    LocalPath string `json:"-"`
    OpenURL   string `json:"open_url,omitempty"`
}
```

- `ToolResult` 新增 `LoadedResources []LoadedResource`；`CheckpointToolResult` 同步保存该字段。

- `ToolCompletedPayload` 和前端 `tool.completed` 类型新增 `resources?: PublicLoadedResource[]`，公开字段只含 `kind/id/name/open_url`。

- `load_component` 每次加载一个组件并返回一个 resource；`load_skill` 可批量加载并返回实际新增或已激活的技能资源列表。

- 公开事件不附带组件 HTML、技能正文、本地绝对路径或预览图片。

- `public_events.go` 对资源字段做显式白名单投影，不允许把任意 `ToolResult.Data` 透传到前端。

### 7.5 已确认的前端交互契约

实现必须以以下原型为视觉与交互基线：

- `docs/discuss/2026-08-30-warehouse-main-integration-prototype.html`

- `docs/discuss/2026-08-30-theme-warehouse-prototype.html`

- `docs/discuss/2026-08-30-component-warehouse-prototype.html`

- `docs/discuss/2026-08-30-skill-warehouse-prototype.html`

统一规则：

- 主工作台右上角使用 Warehouse 图标打开个人仓库菜单；Theme / Component / Skill 仓库右上角使用 Home 返回主工作台。

- 仓库页面标题不重复显示数量，左下角不显示本地路径；所有文件入口使用 Lucide `ExternalLink` 并紧跟资源名称。

- Component 图标维持 Lucide `Component`；Component 无启停状态、无最近使用排序、无卡片重复图标。

- Theme 详情顶部展示名称、ExternalLink、标签/色板/字体，剩余区域用于预览。

- Component 详情顶部展示名称、ExternalLink、标签，剩余区域用于预览；不显示适用场景、文件路径或底部打开按钮。

- Skill 列表以左侧绿色/灰色圆点表达启用/关闭，不在条目内显示状态文字；详情顶部展示名称、ExternalLink 和无文字开关。

- `load_component` / 动态 `load_skill` 活动使用紧凑可展开资源列表：标题为“已加载 N 个组件/技能”，子项只显示资源名称与紧随其后的 `ExternalLink`；不显示图片、路径、kind 或 tags。

- 将现有 `SkillActivity` 的 `Forward` 图标统一替换为 `ExternalLink`。

***

## 8. 数据模型与持久化变更

### 8.1 删除

- `internal/model/asset.go`（`model.Asset`）整体删除。

- `assets` 表（`migrations/0001_init.sql:107-122`）、`store/sqlite/asset_store.go`、`assetPO`（`po.go:394`）删除。

- `internal/asset/` 整个包（manifest / validate / seedfs / seeder / schema）废弃（token 校验能力若保留，迁移到 `designsystem`）。

- `internal/service/asset.go`、`internal/httpapi/asset_handler.go` 删除。

- `internal/contextengine` 中 `AssetCandidate` / `SegmentAssets` / `selectAssets` / `AssetIndexLoader` 相关逻辑删除或改造为「组件清单披露」。

### 8.2 新增/修改

- `outline.json`：不增加主题字段，继续只负责 section / subsection / slide 的叙事结构与顺序。

- `design.json`：保留 `theme` 字段（`spec/types.go:111`），初始化时由 Runtime 写入默认主题；明确其为用户控制字段。

- Agent 的 `design.write` / `design.patch` Schema 排除 `theme`，后端写入时保留原值。

- DB `Project.Theme`：保留为 `design.json.theme` 的读时投影，不作为事实来源。

- 页面物化来源将 `MaterializationSource.DesignRevision` 替换为排除 `theme` 的 `DesignContentHash`，`SourceHash` 同样改用规范化后的 `direction` / `density` / `chrome`；revision 仍随任何 `design.json` 修改正常递增。

- 截图/缩略图等视觉渲染缓存键新增 `theme.css` 内容 hash；换肤不重做 HTML，但必须刷新位图产物。

- `skills/registry.json`：新增读写（`SkillService` 内维护 disabled 过滤）。

- Run 状态：新增独立于 `RunCommand.Skills` 的 active skill set 及正文快照；固定技能在创建时加入，动态技能由 `load_skill` 加入，checkpoint 保存并在恢复时还原。固定选择仍受 `MaxRunSkills = 3` 限制，动态技能不能追加回该字段而触发错误的三项上限。

- 工具活动：`ToolResult` / checkpoint / `ToolCompletedPayload` 增加类型化 loaded resources 投影，供前端恢复可展开的 Component/Skill 加载记录。

- component 无需 DB 表——元数据内联在 `index.html`，运行时扫描目录即可。

- theme 无需 DB 表——元数据在 `manifest.json`，运行时扫描目录即可。

### 8.3 seed（出厂预置）

- 删除 `backend/seed/assets/layouts/**`、`backend/seed/assets/fx/**`。

- `backend/seed/assets/themes/**` 改造：由「manifest.json（含 kind/source/assets）+ tokens.css」改为「manifest.json（精简字段）+ theme.css（完整必需 token，可选正式选择器覆写）」，落地根改为 `~/.dasi/ppt/assets/themes/`。

- `backend/seed/assets/components/**` 改造：由「manifest.json（含 mount/params）+ template.html + style.css」改为「index.html（内联 meta + style + 结构）」，落地根改为 `~/.dasi/ppt/assets/components/`。

- 废弃「拷贝到 `_assets/` + 落 SQLite」的 `seeder.go` 机制；改为启动时将内嵌预置 Theme/Component **仅补写缺失的装配文件**到真源目录，不覆盖任何用户已有文件，不落 SQLite。初始化失败则服务启动失败并返回明确错误。

- `backend/seed/common/base.css` 保留为 Runtime 内嵌结构层，通过 `/api/v1/runtime/base.css` 提供，不再复制到项目 `common/`。

***

## 9. Prompt / Context 变更

- **skill 上下文重构**：现有固定技能正文从 system prompt 迁到带来源标识的 Run 动态 user context；固定技能在 Run 创建时加入，动态技能由 `load_skill` 加入，二者均使用 Run 快照且不提供给 Semantic Reviewer。

- **component 改具名加载**：不再由系统按 slide 元素预筛注入资产正文；改为在上下文提供轻量组件清单（name/description/tags），Agent 主动 `load_component`。

- **theme 只读披露**：上下文中提供当前主题 name、元数据以及 CSS token/白名单选择器契约，供 Agent 生成匹配主题的页面样式，但不提供任何修改主题的手段。

- **护栏 Prompt**：在 CSS 生成相关 Prompt 中加入「强制 `var(--token)`、遵守白名单类名」的软约束条款。

- **引用内容边界**：theme CSS 与 component HTML 作为带来源标识的只读参考数据注入，不能改变 Runtime 策略、工具权限或用户指令；skill 虽作为显式指令加载，也不得覆盖更高优先级 Runtime 安全与工具契约。

***

## 10. 废弃 / 保留 / 改造映射表

| 区域                                                          | 关键位置                                                                          | 处置                                                               |
| ----------------------------------------------------------- | ----------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| 4-kind 统一 manifest（mount/params/fx/layout/requires/cleanup） | `internal/asset/manifest.go`、`asset-manifest.schema.json`                     | 废弃                                                               |
| asset schema 校验                                             | `internal/asset/validate.go`                                                  | 废弃（token 齐备校验迁 `designsystem` 备用）                                |
| seed 拷贝到 `_assets/`                                         | `internal/asset/seedfs.go`、`seeder.go`、`cmd/server/providers.go:36-43`        | 废弃/重写为真源初始化                                                      |
| AssetService CRUD + 快照                                      | `internal/service/asset.go`                                                   | 废弃                                                               |
| REST `/assets`                                              | `internal/httpapi/asset_handler.go`、`router.go:86-91`                         | 废弃                                                               |
| `model.Asset` + `assets` 表                                  | `model/asset.go`、`store/sqlite/*`、`migrations/0001_init.sql:107`              | 废弃                                                               |
| contextengine 资产检索                                          | `assembler.go:selectAssets`、`types.go:AssetCandidate/SegmentAssets`           | 废弃/改造为组件清单披露                                                     |
| 主题名 → 物化 tokens.css                                         | `spec/css.go`、`service/project.go:206-213`、`ppt_mutation.go`、`ppt_helpers.go` | 改造为 link 引用真源                                                    |
| 主题名事实来源与权限                                                  | `spec/types.go:111`（design.theme）、`model/project.go:10`                       | 保留在 `design.json`；Runtime 初始化、用户专用接口修改、Agent Schema 排除           |
| 设计物化失效判定                                                    | `MaterializationSource.DesignRevision`、`SourceHash(..., designRaw)`           | 改为排除 `theme` 的 `DesignContentHash` 和规范化 SourceHash；换肤不触发 HTML 重做 |
| slide HTML 机器护栏                                             | `designsystem/lint.go`（硬编码色/字号/keyframes 拦截）                                  | 弱化/废弃（仅 Prompt 软约束）                                              |
| token 齐备校验                                                  | `designsystem/tokens.go`（requiredTokens/LintTokens）                           | 接入 Theme Repository 读取边界；不扫描普通页面 CSS                         |
| layout 目录解析                                                 | `designsystem/layouts.go`                                                     | 废弃                                                               |
| seed 源（layouts/fx）                                          | `backend/seed/assets/layouts`、`fx`                                            | 废弃                                                               |
| seed 源（themes/components）                                   | `backend/seed/assets/themes`、`components`                                     | 改造（换协议、换落地根）                                                     |
| skill 服务                                                    | `service/skill.go`、`model/run_command.go:59-68`                               | 保留固定选择 + 新增 registry 过滤、64 项目录上限和 Run active skill set           |
| skill 注入                                                    | `workflow/prompt_modules.go:51-61`                                            | 从稳定 system prompt 迁到 Run 动态 user context，审核者隔离                   |
| `load_component` / `load_skill`                             | `workflow/default_tools.go`、`tools.go`                                        | 新增                                                               |
| WorkRoot=`~/.dasi/ppt`                                      | `config.go:157-163`                                                           | 保留 + 约定三模块子路径                                                    |

***

## 11. 迁移与兼容策略

- 开发期直接切换：一次性删除旧 `_assets` 目录、`assets` 表、`/assets` 接口、4-kind 协议，不保留双读/双写/降级分支。

- 前后端、持久化、Schema、测试、相关文档同步切换到三模块协议。

- 不为历史开发数据设计迁移方案；旧 `_assets` 与旧 `assets` 表数据直接废弃。

- `design.json.theme` 保留，不进行跨文件迁移；一次性切换到字段级权限模型，并同步修改 Agent 工具 Schema 与物化失效判定。

***

## 12. 已确认实现参数

- **Component 文件上限**：64KB。

- **首启预置策略**：内嵌 seed 仅补写缺失装配文件，绝不覆盖用户文件，不写数据库。

- **公共基座层**：保留 Runtime Base；通过只读 HTTP 端点与隔离渲染器本地路由提供，不复制到项目。

- **Theme manifest**：不保留 `version` 与 `tags`；仅包含 `name`、`description`。

- **Theme 分层契约**：必需 token 与允许的可选主题选择器以 §4.6 为准；Theme 不要求包含任何结构选择器。

- **Component 状态**：无状态，合法即启用。

- **工具结果媒体**：Component/Skill 加载活动不返回或展示预览图片。

***

## 附：目录协议速查

```text
~/.dasi/ppt/
├── assets/
│   ├── themes/<name>/manifest.json          # {name, description}
│   ├── themes/<name>/theme.css              # 完整必需 token + 可选正式选择器覆写
│   └── components/<name>/index.html         # <script id=meta> + <style> + 结构（自包含）
└── skills/
    ├── registry.json                        # {"disabled": [<skill-id>...]}
    └── <skill-id>/SKILL.md                  # frontmatter{name,description} + 正文
```
