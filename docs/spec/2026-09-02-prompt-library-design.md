# 提示词库与输入框快捷短语设计

**日期：** 2026-09-02\
**状态：** 产品语义、存储方案与交互方案已确认，可用于前后端实现\
**范围：** Prompt SQLite 模型、CRUD API、仓库模块、输入框触发与匹配、彩色提示词片段、键盘交互、测试与验收\
**原型：** `docs/demo/2026-09-02-prompt-library-demo.html`
**图标候选：** `docs/demo/2026-09-02-prompt-icon-options.html`

***

## 1. 结论

新增第四个个人仓库模块「提示词」。它用于管理应用内快捷短语，并在 Agent 输入框中通过 `$` 或 `¥` 触发候选列表。

提示词与 Theme、Component、Skill 的本质不同：

- Theme、Component、Skill 是用户可在外部 IDE 中查看和维护的文件资产。

- Prompt 是仅由产品 UI 管理的结构化应用数据。

- Prompt 以 SQLite 为唯一事实来源，不写入 `.dasi/ppt/assets/prompts`，不生成 JSON 或 Markdown 文件，也不提供 `open_url`。

最终交互：

1. 用户在输入开头、普通空格后或换行后输入 `$` / `¥`。
2. 输入框上方出现紧凑候选列表。
3. 查询使用非前缀包含匹配，范围为 `name + desc + value + tags`。
4. `↑/↓` 切换候选，`Enter` 替换，`Esc` 关闭；鼠标点击同样可替换。
5. 替换后的提示词正文是可继续编辑的蓝色文字，带极浅蓝底纹。
6. 提示词片段之后自动补一个普通颜色空格，并将光标移动到空格后。
7. 发送时仅提交纯文本，不向 Run API 暴露提示词 ID 或富文本结构。

V1 不支持模板变量、参数填写、提示词组合语法、云同步或导入导出。

***

## 2. 已确认产品规则

### 2.1 触发边界

合法触发位置只有：

- 输入框全文开头；

- 前一字符为 ASCII 空格 `U+0020`；

- 前一字符为换行 `\n`。

以下情况不触发：

- 中文或英文标点后；

- 单词、路径、代码、金额中的 `$` / `¥`；

- 光标不是折叠状态；

- IME 正在组词；

- 编辑器禁用、只读或正在执行润色。

`$` 与 `¥` 使用同一提示词命名空间，触发符本身不存入 Prompt。

### 2.2 查询范围

从最近一个合法触发符到当前光标之间的连续非空格、非换行文本为查询词。

匹配规则：

- 对 `name`、`desc`、`value` 和标签名称做 `%query%` 包含匹配；

- 英文字母不区分大小写；

- 中文按原字符匹配；

- 查询为空时显示最近使用的 5 条；

- 查询非空时最多显示 8 条。

排序优先级：

1. `name` 命中；
2. 标签名称命中；
3. `desc` 命中；
4. `value` 命中；
5. 最近使用顺序；
6. `updated_at` 降序；
7. `name` 稳定排序。

### 2.3 键盘规则

- 候选打开后默认选中第一项。

- `ArrowDown` / `ArrowUp` 循环切换候选。

- `Enter` 应用当前候选，不产生换行。

- `Escape` 关闭候选但保留输入内容。

- 鼠标悬停只改变高亮候选；点击应用候选。

- `Cmd/Ctrl + Enter` 始终保留现有发送语义。

- 候选关闭后，普通 `Enter` 继续产生换行。

### 2.4 替换与编辑

- 使用 Prompt `value` 替换完整的 `$query` / `¥query`。

- Prompt 正文以可编辑文本片段插入，不使用不可编辑 Chip。

- 片段采用产品蓝色文字与极浅蓝底纹，不加边框，不做胶囊形状。

- 长文本按普通段落自然换行。

- 用户在片段内部编辑后，片段继续保留提示词样式。

- 片段后自动插入一个普通文本空格，光标落在空格后。

- 删除、剪切、撤销、重做遵循编辑器原生行为。

- 粘贴内容一律按纯文本处理，禁止把外部 HTML 带入编辑器。

- 润色会整体替换文本，因此润色成功后清除所有 Prompt 来源标记，结果全部恢复为普通文本。

- 发送请求仍只使用 `instruction: string`；发送成功后清空文本和片段标记。

### 2.5 Prompt 字段

| 字段                | 规则                                      |
| ------------------- | ----------------------------------------- |
| `id`                | 服务端生成、不可修改的稳定 ID             |
| `name`              | 必填、全库唯一、1–80 字符，不允许换行或制表符 |
| `normalized_name`   | 服务端生成，用于名称大小写不敏感的唯一约束，不出现在 API |
| `desc`              | 必填纯文本，最多 500 字符                 |
| `value`             | 必填纯文本，最多 16KB                     |
| `tags`              | 0–2 个受控枚举值，不允许重复              |
| `created_at`        | 服务端时间                                |
| `updated_at`        | 服务端时间                                |

`name` 是 Prompt 唯一的映射和检索名称，可以直接同时包含中英文，例如 `高管摘要 / Executive Summary`，不再拆分中文 key 和英文 key。名称裁剪首尾空白后以小写形式生成 `normalized_name` 并判断唯一性；`desc` 只承担简短说明，`value` 是选中 Prompt 后实际插入输入框的正文。

### 2.6 标签枚举

标签用于仓库筛选与包含搜索，不出现在输入候选的首屏信息中。每条 Prompt 最多选择 2 个：

| 值           | 中文标签 | 用途          |
| ----------- | ---- | ----------- |
| `structure` | 结构   | 大纲、章节、叙事顺序  |
| `draft`     | 撰写   | 从零生成正文或页面内容 |
| `rewrite`   | 改写   | 精简、扩写、换语气   |
| `summarize` | 总结   | 摘要、结论、要点提取  |
| `analysis`  | 分析   | 洞察、比较、推理    |
| `data`      | 数据   | 数据处理、图表表达   |
| `visual`    | 视觉   | 版式、风格、视觉呈现  |
| `review`    | 审查   | 质量检查、纠错、验收  |
| `other`     | 其他   | 无法归入以上类别    |

不使用自由标签：快捷短语规模有限，固定枚举能保证筛选稳定，并避免同义标签持续膨胀。

***

## 3. 为什么使用 SQLite

Prompt 是产品内快捷短语，不需要外部 IDE 编辑、文件跳转或独立资源搬运。其核心需求是结构化 CRUD、唯一名称映射、原子更新和稳定排序，因此应归入应用数据。

不采用单一 JSON：

- 任意一条修改都要重写整个文件；

- 单点损坏会导致整个提示词库不可用；

- 并发写与唯一性需要重新实现；

- Git/备份差异会集中在一个大文件；

- JSON 作为数据库替代品不会降低实际复杂度。

不采用每条一个 JSON：

- 用户没有外部查看或复制这些文件的需求；

- 会重复实现数据库已有的唯一约束、事务和查询能力；

- 仓库页面是否统一，不要求底层持久化形式相同。

SQLite 已经是本项目的应用数据基础设施。Prompt 只需新增一张表、Store 方法和 migration，不引入第二套一致性机制。

***

## 4. 数据库设计

新增 migration：`backend/migrations/0004_prompts.sql`。

```sql
CREATE TABLE IF NOT EXISTS prompts (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    normalized_name TEXT NOT NULL UNIQUE,
    "desc"          TEXT NOT NULL,
    value           TEXT NOT NULL,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_prompts_updated_at
ON prompts(updated_at DESC, id);
```

说明：

- `normalized_name` 由服务端根据裁剪后的 `name` 生成，禁止客户端提交。

- `tags` 继续通过统一的 `resource_tags` 关系表存储，不在 `prompts` 表重复保存。

- `0006_prompt_name_schema.sql` 在开发期直接重建 Prompt 表并清除旧 Prompt 标签和启停关系，不保留双 key 兼容或历史数据迁移。

- 不依赖 SQLite `NOCASE` 处理 Unicode；归一化逻辑集中在 Service。

- V1 不增加 FTS。输入框会一次加载 Prompt 列表并在前端内存匹配。

- 不把最近使用时间写入该表；最近使用是单设备交互偏好，不是 Prompt 资源属性。

- 删除为硬删除，不做软删除或回收站。

***

## 5. 后端设计

### 5.1 Model

新增 `backend/internal/model/prompt.go`：

```go
type Prompt struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Desc        string `json:"desc"`
    Value       string `json:"value"`
    Tags        []PromptTag `json:"tags"`
    CreatedAt   int64  `json:"created_at"`
    UpdatedAt   int64  `json:"updated_at"`
}
```

`PromptTag` 使用 Prompt 仓库当前受控枚举。API 不返回 `normalized_name`，标签与启停状态仍由统一仓库元数据表投影到响应。

### 5.2 Store

在 `backend/internal/store/store.go` 增加：

```go
CreatePrompt(ctx context.Context, prompt model.Prompt, normalizedName string) error
GetPrompt(ctx context.Context, id string) (model.Prompt, error)
ListPrompts(ctx context.Context) ([]model.Prompt, error)
UpdatePrompt(ctx context.Context, prompt model.Prompt, normalizedName string) error
DeletePrompt(ctx context.Context, id string) error
```

SQLite 实现在 `backend/internal/store/sqlite/prompt_store.go`：

- Create/Update 依赖数据库唯一索引解决并发名称冲突。

- 将唯一约束错误稳定映射为领域错误 `ErrPromptNameConflict`。

- Update 必须先确认 ID 存在，不允许隐式 upsert。

- List 默认按 `updated_at DESC, id ASC` 返回。

### 5.3 Service

新增 `backend/internal/service/prompt.go`，负责：

- 字段裁剪与长度校验；

- `name` / `desc` 的必填、长度与名称唯一性校验；

- `normalized_name` 生成；

- 标签去重、数量与枚举校验；

- ID 和时间生成；

- Store 错误到领域错误的映射；

- Create/Get/List/Update/Delete。

建议领域错误：

```text
ErrPromptNotFound
ErrPromptNameConflict
ErrPromptInvalid
```

Service 不实现 `%LIKE%` 搜索。候选匹配是本地 UI 高频操作，前端持有完整列表可以避免逐键网络请求、竞态和闪烁。

### 5.4 HTTP API

新增 `PromptHandler`，并在 `Router` 注入独立 handler。Prompt 虽显示在仓库中，但不应继续扩大当前文件仓库专用的 `RepositoryHandler`。

| 方法       | 路径                    | 用途          |
| -------- | --------------------- | ----------- |
| `GET`    | `/api/v1/prompts`     | 返回全部 Prompt |
| `GET`    | `/api/v1/prompts/:id` | 返回单条 Prompt |
| `POST`   | `/api/v1/prompts`     | 创建 Prompt   |
| `PUT`    | `/api/v1/prompts/:id` | 完整更新 Prompt |
| `DELETE` | `/api/v1/prompts/:id` | 删除 Prompt   |

创建请求：

```json
{
  "name": "高管摘要 / Executive Summary",
  "desc": "提炼核心结论、关键数据、风险与下一步行动。",
  "value": "请将以上内容整理为高管摘要，突出结论、关键数据、风险与下一步行动。",
  "tags": ["summarize", "rewrite"]
}
```

响应与错误：

- `POST` 成功：`201` + Prompt。

- `PUT` 成功：`200` + Prompt。

- `DELETE` 成功：`204`。

- ID 不存在：`404 PROMPT_NOT_FOUND`。

- 名称冲突：`409 PROMPT_NAME_CONFLICT`，错误 detail 的字段为 `name`。

- 字段非法：`400 PROMPT_INVALID`。

- 其他存储错误：`500`。

***

## 6. 前端数据层

新增：

- `Prompt`、`PromptsResponse`、`PromptWriteRequest` 类型；

- `frontend/src/api/prompts.ts`；

- 轻量 `promptStore`，缓存 Prompt 列表、加载状态和版本号。

`CommandComposer` 与 `PromptRepositoryPage` 共用同一 Store：

- 首次聚焦 Composer 或进入 Prompt 仓库时加载；

- CRUD 成功后直接更新缓存；

- 仓库刷新按钮强制重新加载；

- Composer 后台加载失败不触发全局错误弹窗，不影响普通输入与发送；

- 仓库 CRUD 失败使用现有全局错误提示。

最近使用：

```text
localStorage key: dasi.prompt-recent.v1
value: string[] // Prompt ID，最近使用在前，最多 20 个
```

不存在的 ID 在每次列表加载后清理。

***

## 7. 输入框实现

### 7.1 为什么不能继续使用原生 textarea

原生 `<textarea>` 不能为同一段 value 的不同字符设置不同颜色。通过镜像层叠加文本虽然可以模拟颜色，但会增加选区、滚动、换行测量、IME 和高 DPI 对齐风险。

因此将文本区域抽为 `PromptComposerEditor`，使用受约束的 `contenteditable`：

- 编辑器内部只允许文本节点、`<br>` 与带 `data-prompt-id` 的 `<span>`；

- 粘贴强制为纯文本；

- DOM 只承载当前编辑状态，不接受任意 HTML；

- 提交、润色和长度判断统一通过 `serializePlainText()`；

- React 不在每次击键时重建 DOM，避免 Selection 丢失；

- 项目切换、发送成功和润色成功通过显式命令重置内容。

提示词片段结构：

```html
<span data-prompt-id="p_..." class="composer-prompt-fragment">
  请将以上内容整理为高管摘要……
</span><span> </span>
```

样式：

```css
.composer-prompt-fragment {
  color: var(--color-accent);
  background: color-mix(in srgb, var(--color-accent) 7%, transparent);
}
```

不增加边框、圆角、额外 padding 或不可编辑属性。

### 7.2 触发检测

只在以下事件后重新计算：

- `input`；

- `selectionchange`；

- `compositionend`；

- 编辑器聚焦；

- Prompt 列表更新。

算法：

1. 确认编辑器可用、Selection 折叠且位于编辑器内部。
2. 序列化从编辑器开头到光标的纯文本。
3. 从末尾匹配：

```regex
(?:^|[ \n])([$¥])([^ \n$¥]*)$
```

1. 记录触发符在 DOM 中的 Range，而不是只保存字符串 offset。
2. 以查询词匹配并排序候选。
3. 光标离开 Range、输入空格/换行、编辑器失焦或按下 Esc 时关闭。

触发检测必须忽略 `compositionstart` 到 `compositionend` 之间的中间态，避免中文输入法候选被打断。

### 7.3 候选菜单

菜单是 Composer 内部的 absolute popover：

- 位于输入框正上方，与 Composer 外框左右边缘严格对齐；

- 宽度始终等于 Composer 外框宽度；

- 最大高度约 `280px`，内容滚动；

- 空查询标题为“最近使用”；

- 非空查询标题为“匹配提示词”；

- 每项采用单行紧凑布局：图标、`name`、`desc` 摘要依次排列；

- `NotebookText` 图标直接显示在行内，不增加独立方框、底色或描边容器；

- 图标列宽约 20px，图标到名称间隔 4px，名称到描述间隔 6px；

- `name` 使用主文字色，`desc` 使用次级灰色并在剩余空间内单行截断；

- 不显示 `$` / `¥`；

- 当前项只通过整行浅蓝背景表达选中，不单独强调某一个 key；

- 不显示 Enter 图标或其他尾部操作；

- 无匹配时保持与 Composer 等宽，但空状态高度收紧。

候选弹出不改变 Composer 高度，也不推动时间线内容。

### 7.4 替换算法

1. 使用已保存 Range 选中触发符和查询词。
2. 删除 Range 内容。
3. 插入 `data-prompt-id` span，textContent 为 Prompt `value`。
4. 紧随其后插入普通空格文本节点。
5. 合并相邻普通文本节点，但不合并不同 Prompt span。
6. 将光标折叠到普通空格之后。
7. 更新最近使用列表并关闭候选。
8. 触发 Composer plain-text 状态同步与高度重算。

### 7.5 与现有能力的关系

- `submit()` 改为读取编辑器序列化后的纯文本。

- `text.trim()` 等判断改为 `plainText.trim()`。

- `applyShortcut()` 仍作用于最终纯文本，与 Prompt 展开互不冲突。

- `polishText()` 仍提交完整纯文本；成功后以普通文本重置编辑器。

- 选区恢复使用编辑器适配器，不再依赖 `textarea.selectionStart/selectionEnd`。

- 发送快捷键、停止运行按钮、模式/技能/目标/模型控件保持不变。

- Prompt 列表加载失败仅使快捷短语不可用，不禁用 Composer。

***

## 8. 提示词仓库页面

### 8.1 导航与路由

扩展：

```ts
type RepositorySection = 'theme' | 'component' | 'skill' | 'prompt';
```

新增路由：

```text
/warehouse/prompt
```

`RepositoryShell` 左侧模块顺序：

1. 主题
2. 组件
3. 技能
4. 提示词

提示词图标统一使用 Lucide `NotebookText`（图标候选 F）。提示词没有 `open_url`，详情标题旁不显示 ExternalLink。

### 8.2 页面骨架

Prompt 仓库继续使用与 Theme / Component / Skill 一致的 `RepositoryShell` 和主从分栏骨架：

- 顶部：页面标题“提示词”与搜索框；

- 左侧：320px 单列目录，承载标签筛选与 Prompt 列表；

- 右侧：固定详情头部与主内容区；

- 删除按钮位于详情名称右侧；

- 新建入口在有数据时固定于左侧目录底部，空状态时居中；

- 不使用大弹窗创建 Prompt。

列表项：

- 左侧保留 58×42 的稳定对齐区域，`NotebookText` 直接显示，不增加独立方框、底色或描边；

- 中间上行展示 `name`，中英文可由用户直接写在同一名称中；

- 中间下行展示两行以内的 `desc`；

- 右侧为 ChevronRight；

- 选中态沿用现有浅蓝背景。

筛选栏使用标签枚举，允许选择“全部”或一个标签。搜索范围与 Composer 一致：`name + desc + value + 标签名称` 包含匹配。

### 8.3 查看、编辑与新建

默认查看态：

- 头部展示 `name`，下方展示 `desc`；

- 头部不显示 `value`，描述下方展示标签；

- 标签之间保持 8px 间距并允许换行；

- 右上角为“编辑”按钮；

- 主区域使用唯一的文稿卡片直接展示完整 `value`，不增加“提示词 value”等重复标签；

- 左侧目录中的 `desc` 作为当前列表项摘要，右侧主区域只展示 `value`；

- 删除按钮仍紧随名称标题。

编辑态：

- 使用原位表单，不打开 Modal；

- 字段顺序为 `name`、`desc`、标签、`value`；

- 右上角显示“取消”和“保存”；

- 保存前在前端执行必填与长度校验；

- 保存中禁用重复提交；

- 保存成功回到查看态；

- 名称冲突时错误贴近 `name` 输入框显示；

- 离开存在未保存修改的条目时弹出确认。

新建态：

- 左侧目录保持可见，右侧进入空表单；

- 默认聚焦 `name`；

- 保存成功后选中新条目并进入查看态；

- 取消返回此前选中的 Prompt。

### 8.4 页面状态

- 加载：复用 `RepositoryLoading aside`。

- 全局加载失败：复用 `RepositoryState error`。

- 无匹配：左侧目录显示“没有匹配的提示词”。

- 全库为空：左侧目录居中显示“新建提示词”。

- 删除成功：选择相邻条目；无剩余条目时进入空状态。

- 删除失败：保持当前条目与编辑内容。

***

## 9. 视觉规范

沿用当前仓库 Token：

- 页面背景、边框、字号、圆角、阴影全部复用现有仓库组件；

- 不增加独立主题或渐变；

- 卡片圆角不超过 8px；

- 输入候选菜单保持紧凑，不使用大弹窗；

- Prompt 高亮只使用产品蓝和极浅蓝底纹；

- 输入区弹层必须覆盖在 Composer 上方，不改变右栏宽度或整体布局；

- 文本不得因高亮样式改变行高、字重或换行位置。

原型中的视觉签名是“正文级彩色来源片段”：它看起来仍是句子的一部分，但能让用户明确识别哪些文字来自快捷提示词。

***

## 10. 安全与一致性

- Prompt 正文只作为用户输入文本，不作为 system prompt 或独立高优先级指令注入。

- API 输入统一限制长度，拒绝空正文。

- 前端渲染 Prompt 一律使用 textContent，不使用 `innerHTML`。

- contenteditable 粘贴只接受 `text/plain`。

- 名称唯一性由数据库最终保证，前端预校验仅用于即时反馈。

- CRUD 更新使用完整 PUT，开发期不保留旧结构兼容层。

- Prompt 删除不追溯修改已经发送的历史消息。

***

## 11. 测试计划

### 11.1 后端

- migration 创建表和索引；

- Create/Get/List/Update/Delete；

- ID 稳定、时间更新；

- 名称和描述的必填与长度校验；

- 名称大小写冲突；

- 标签枚举、去重与最多 2 个校验；

- 名称更新冲突映射为 409 并标明字段；

- 字段裁剪、空值和长度边界；

- 删除不存在资源返回 404；

- 并发创建同名 Prompt 仅一个成功；

- Store 与 Handler 错误投影。

### 11.2 前端数据层

- 列表加载与缓存；

- CRUD 成功更新缓存；

- CRUD 失败保留旧状态；

- 最近使用去重、截断和无效 ID 清理；

- Prompt 加载失败不禁用 Composer。

### 11.3 输入框

- 输入开头、空格后、换行后触发；

- 标点后、单词内、金额中不触发；

- `$` 与 `¥` 行为一致；

- 中文 IME 组词期间不触发；

- `%query%` 非前缀匹配；

- 匹配字段优先级与最大条数；

- 空查询最近使用；

- 上下键循环、Enter 替换、Esc 关闭；

- 替换 Range 正确且末尾补普通空格；

- 插入片段可编辑并保持样式；

- 粘贴被净化为纯文本；

- 提交序列化为纯文本；

- 润色成功清除 Prompt 样式；

- Cmd/Ctrl+Enter 发送语义不回归。

### 11.4 仓库页面

- 第四模块导航和路由；

- 统一页面骨架；

- 搜索、选中与空状态；

- 新建、编辑、取消、保存；

- 名称冲突字段错误；

- 未保存修改确认；

- 删除确认与选中项回退；

- 无 ExternalLink；

- 窄屏不溢出。

***

## 12. 建议实施顺序

1. 新增 migration、Model、Store、Service 与 Handler。
2. 增加 API 类型、`promptsApi` 与共享 Prompt Store。
3. 增加仓库导航、路由和 `PromptRepositoryPage`。
4. 抽取 `PromptComposerEditor`，先完成纯文本等价行为。
5. 接入触发检测、候选菜单与替换。
6. 接入最近使用、润色重置和错误降级。
7. 补齐后端、仓库页面和 Composer 自动化测试。
8. 使用桌面与窄屏截图验证菜单位置、文本换行和仓库布局。

***

## 13. 非目标

- 模板变量与参数表单；

- Prompt 的 Agent 动态加载工具；

- Prompt 文件浏览或 `open_url`；

- JSON/Markdown 导入导出；

- 云同步、团队共享与权限；

- 全文搜索、SQLite FTS；

- Prompt 版本历史、回收站；

- 使用频次服务端统计；

- 将 Prompt 来源信息写入 Run 或历史消息。
