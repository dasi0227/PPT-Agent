# 个人仓库三模块一次性实施启动 Prompt

你现在负责在 `/Users/bytedance/Desktop/ByteDance/PPT_Agent` 中完整实现个人仓库 Theme / Component / Skill 三模块。

## 执行要求

这是一次端到端实施任务，不是方案讨论。

- 不允许只输出计划、局部代码或“下一步建议”后中断。
- 必须在同一个会话中持续完成代码阅读、实现、数据结构切换、测试、视觉验证、修复和最终交付。
- 不要在阶段之间等待用户确认；完成一个阶段后直接进入下一阶段。
- 遇到测试失败、类型错误、构建错误或视觉问题时自行定位并修复，不得把修复留给后续会话。
- 只有遇到无法从代码、最新设计文档和原型推导的真实产品决策，且错误选择会造成明显返工时，才向用户提问。
- 上下文压缩或任务耗时不是停止理由；压缩后继续执行。
- 可以并且应当分阶段创建 Git commit。每个 commit 必须是完整、可验证的阶段，不得提交无法构建的中间状态。
- 不要 push，不要 amend，不要使用 destructive git 命令，不要覆盖用户已有修改。
- 开始前阅读 `AGENTS.md`，并遵循其中开发期直接切换、无兼容层的规则。

## 权威资料

发生冲突时，按以下优先级执行：

1. `AGENTS.md`
2. `docs/discuss/2026-08-30-warehouse-three-modules-backend-design.md`
3. 已确认的前端原型：
   - `docs/discuss/2026-08-30-warehouse-main-integration-prototype.html`
   - `docs/discuss/2026-08-30-theme-warehouse-prototype.html`
   - `docs/discuss/2026-08-30-component-warehouse-prototype.html`
   - `docs/discuss/2026-08-30-skill-warehouse-prototype.html`
4. 当前代码和测试
5. 更早的历史文档仅供背景参考

先检查 `git status`。上述设计文档和原型可能仍是未跟踪文件，它们属于本任务成果，不要删除。

## 不可变产品决策

- 个人仓库只有 Theme、Component、Skill，彻底删除旧 Layout / FX 与统一 Asset 协议。
- Theme 是用户选择的全局样式真源；Agent 只读，不能选择或修改主题。
- Component 是供 Agent 模仿的只读 HTML 参考片段，不是插件，不物理注入，不参数化。
- Component 无启用/关闭状态，合法即对 Agent 可用。
- Skill 有启用/关闭状态；禁用技能不能固定选择，也不能动态加载。
- 固定 Skill 最多 3 个；动态加载 Skill 属于独立 Run active skill set，不受固定三项上限影响。
- `load_component` 不提供图片预览。工具活动只显示“已加载 N 个组件”和资源名称列表。
- `load_skill` 的动态加载活动采用相同的紧凑资源列表。
- 所有文件跳转统一使用 Lucide `ExternalLink`，紧跟资源名称；UI 不展示本地路径。
- Component 图标继续使用 Lucide `Component`。
- 不实现最近使用、使用次数、在线编辑或创意工坊。
- 开发期直接切换，不保留旧 API、旧 DB 表、旧 Schema、旧目录或迁移兼容逻辑。

## 后端实施范围

### 1. 三模块真源与初始化

- 真源根目录：
  - `~/.dasi/ppt/assets/themes/<id>/manifest.json`
  - `~/.dasi/ppt/assets/themes/<id>/theme.css`
  - `~/.dasi/ppt/assets/components/<id>/index.html`
  - `~/.dasi/ppt/assets/skills/registry.json`
  - `~/.dasi/ppt/assets/skills/<id>/SKILL.md`
- 目录名是稳定 ID，元数据 `name` 是人类可读展示名，前端不得互相推导。
- Theme manifest 仅包含 `name/description`，不保留 version 与 tags。
- Component `index.html` 内联 `script#meta`、style 和结构，单文件上限 64KB。
- Skill 延续 frontmatter + Markdown，增加中央 `registry.json` disabled 列表。
- 根目录初始化脚本只补写缺失的 `seed/assets` 文件，绝不覆盖用户文件，不写 SQLite；后端启动不负责 seed 安装。
- registry 缺失等价于空 disabled；registry 损坏必须明确报错并停止 Skill 披露。
- 所有模块拒绝符号链接、目录穿越、绝对路径注入和非普通文件。

### 2. 删除旧统一 Asset 协议

- 删除旧 `_assets`、AssetService、Asset handler/routes、Asset model/store/table、统一 manifest/schema、旧 seeder 和语义预筛。
- 删除 Layout / FX seed 与相关解析。
- 直接修改开发期初始 migration，不保留 assets 表兼容迁移。
- 删除无效测试并替换为三模块服务、路由和安全测试。

### 3. Theme Runtime

- 保留 Runtime Base 作为主题无关结构层，通过 `GET /api/v1/runtime/base.css` 提供。
- Theme 真源通过 `GET /api/v1/themes/:id/css` 提供。
- Runtime 管理并规范化 `base-link` 与 `theme-link`，Agent 不拥有这两个节点。
- 不再向项目复制 `common/tokens.css` 或 `common/base.css`。
- `design.json.theme` 保存 Theme ID，是唯一事实来源；默认 `swiss-modern`。
- 新增用户专用换肤接口，使用项目写锁，原子更新 design 与 DB 投影。
- Agent 的 design mutation Schema 不允许 theme，执行层也必须保留当前 theme。
- HTML 新鲜度 hash 排除 theme；截图/缩略图缓存必须包含 theme.css 内容 hash。
- 隔离 Chromium 不得回连主 API。Go 侧解析本次 Base/Theme CSS 后传给 worker，worker 的临时同源 server 为 `/api/v1/runtime/base.css` 和 `/api/v1/themes/:id/css` 提供内容，同时保持外部网络阻断。

### 4. Component / Skill 服务与工具

- Theme、Component、Skill 服务共用严格的 ID、路径和普通文件校验。
- REST 列举/详情 DTO 均包含稳定 `id`、展示 `name` 和 `open_url`。
- Theme `open_url` 指向 theme.css；Component 指向 index.html；Skill 指向 SKILL.md。
- Component REST 无 disabled 字段，无启停接口。
- Skill REST 返回 disabled；PATCH body 为 `{ "disabled": boolean }`，原子写 registry。
- `load_component` 是只读工具，每次按 ID 加载一个合法组件全文。
- `load_skill` 是只读控制工具，支持批量、去重、上限和上下文预算，只允许已启用 Skill。
- 固定和动态 Skill 都进入 Run 动态 user context，不进入稳定 system prompt，不提供给 Semantic Reviewer。
- active skill set 与正文快照写入 checkpoint，恢复后保持；禁用不追溯影响已快照 Run。

### 5. 工具公开事件

- 不要把任意 `ToolResult.Data` 直接暴露给前端。
- 新增类型化 `LoadedResource`，至少包含 kind/id/name/local path/open URL；公开投影只暴露 kind/id/name/open_url。
- `ToolResult`、`CheckpointToolResult`、`ToolCompletedPayload` 和前端 SSE 类型同步增加 loaded resources。
- `load_component` / `load_skill` 的 `tool.completed` 必须携带资源列表，历史恢复后仍能显示。
- 公开事件不得携带组件 HTML、Skill 正文、本地绝对路径或预览图片。

## 前端实施范围

### 1. 主工作台

- 在现有顶栏加入 Warehouse 图标和个人仓库菜单，入口为 Theme / Component / Skill。
- 菜单与页面使用真实 API 数据，不硬编码 prototype 数组。
- `load_component` / 动态 `load_skill` 使用紧凑可展开活动：
  - 标题“已加载 N 个组件/技能”
  - 子项仅显示展示名称和紧随其后的 `ExternalLink`
  - 无图片、路径、kind、tags
- 将现有 `SkillActivity` 的 `Forward` 改为 `ExternalLink`。

### 2. 仓库公共框架

- 复用现有设计系统和 Lucide，不复制四套无关样式。
- 左侧固定 Theme / Component / Skill 导航；右上角 Home 返回主工作台。
- 页面标题不重复显示数量，左下角不显示路径。
- 文件入口统一为名称旁的 `ExternalLink`，tooltip 为“查看文件”。
- 搜索、筛选、选中态、空态、加载态、错误态和禁用交互必须完整。
- 不显示“最近使用”，不实现使用统计。

### 3. Theme

- 左侧主题清单，右侧顶部显示名称、ExternalLink、色板、字体。
- 右侧剩余区域全部用于综合页/封面/数据页预览。
- 切换主题调用项目换肤接口，并刷新真实预览。

### 4. Component

- 管理式网格，卡片不显示重复 Component 图标。
- 过滤项按真实 tags/kind 设计，但不增加状态过滤。
- tags 固定为 `card/metric/comparison/quote/list/chart/process/timeline/other`，界面展示对应中文，不接受任意字符串或重复值。
- 详情顶部显示名称、ExternalLink、标签；剩余区域全部用于安全预览。
- 不显示适用场景、路径、底部打开按钮或最近使用。
- 组件 HTML 必须在 sandboxed iframe/srcdoc 中预览，不能直接注入 React DOM。

### 5. Skill

- 列表左侧绿点表示启用、灰点表示关闭；条目不显示“已启用/已关闭”文字。
- 保留全部/已启用/已关闭筛选。
- 不显示“名称”列标题、顶部状态计数或本地路径。
- 详情顶部显示名称、ExternalLink 和无文字开关；正文只读预览。
- PATCH 成功后更新列表、筛选结果和选择状态；失败回滚并使用现有 Toast。

## 测试与验收

后端至少覆盖：

- Theme/Component/Skill 扫描、解析、非法 ID、目录穿越、符号链接、大小上限。
- seed 仅补缺且不覆盖用户内容。
- registry 缺失、损坏、原子更新、disabled 过滤。
- Component 无状态契约。
- Theme/Base CSS 端点和 Content-Type。
- theme mutation 权限、锁、hash 新鲜度与渲染缓存。
- render worker 的 Base/Theme 同源路由和外部网络阻断。
- load_component/load_skill Schema、预算、去重、禁用、checkpoint/resume。
- Tool public event loaded resources 投影及敏感内容不泄漏。
- 旧 Asset API、表和代码完全移除。

前端至少覆盖：

- 仓库菜单和三页面路由。
- 搜索、筛选、选择、空态、错误态。
- Skill toggle 乐观/失败行为。
- Component sandbox 预览。
- loaded resources 活动展开、历史恢复和 ExternalLink。
- `SkillActivity` 使用 ExternalLink。
- API/SSE parser 对新字段的校验。

必须运行并修复至通过：

```bash
cd backend && go test ./...
cd frontend && pnpm test
cd frontend && pnpm lint
cd frontend && pnpm tsc
cd frontend && pnpm build
```

若修改 Wire provider，使用仓库依赖版本重新生成 `wire_gen.go`，不得手写出与 `wire.go` 不一致的装配代码。

启动本地后端和前端，使用 Playwright 对主工作台、Theme、Component、Skill 页面进行桌面和移动视口截图检查。确认无空白、溢出、遮挡、失效图标、断链、控制错位或控制台错误。修复后再结束。

## 分阶段提交

建议按以下可构建阶段提交，允许根据代码依赖合并相邻阶段，但不得提交失败状态：

1. `docs: finalize personal repository contracts`
2. `refactor: replace legacy assets with repository modules`
3. `feat: add theme runtime and repository APIs`
4. `feat: add component and dynamic skill loading`
5. `feat: add personal repository interface`
6. `test: complete repository integration coverage`

每次提交前只 stage 本阶段相关文件，遵循 `AGENTS.md` commit 格式。最后确认 working tree 中没有遗漏的本任务代码；不要提交无关用户修改。

## 完成定义

只有同时满足以下条件才能结束：

- 三模块真实数据、API、工具、Run 持久化、公开事件和前端 UI 全部贯通。
- 四个确认原型中的关键交互已落到真实应用。
- 所有旧 Asset/Layout/FX 运行路径已删除，不存在新旧双轨。
- 后端与前端全部测试、lint、类型检查和构建通过。
- Playwright 桌面/移动视觉检查通过。
- 已按阶段创建 commits，未 push。
- 最终回复列出提交、关键实现、验证结果、运行 URL，以及任何确实无法完成的外部限制。不得以 TODO 或“后续可继续”代替实现。
