# 上下文压缩时间线与标题协议设计

- 状态：已实现
- 日期：2026-09-17
- 关联：
  - `2026-09-04-context-window-compaction-design.md`
  - `2026-09-17-context-window-state-and-layer-refactor-design.md`

## 1. 设计目标

本次变更解决两个问题：

1. 手动压缩完成后，压缩结果只写入 Context Window Store，没有立即加入当前 thread 的 timeline，必须刷新并重新加载 history 才能看到。
2. 当前压缩结果使用独立富卡片，与已经定稿的 Git commit 扁平时间线事件不一致，信息层级过重。

同时为每次压缩新增由压缩模型生成的短标题，使压缩记录可以像提交记录一样被快速扫描和识别。

本文档关于“压缩完成事件”的标题协议、公开数据、即时插入和前端呈现是最新设计，取代 `2026-09-04-context-window-compaction-design.md` 第 7.3 节中的“时间线事件富卡片”方案。上下文窗口六桶协议继续以 `2026-09-17-context-window-state-and-layer-refactor-design.md` 为准。

## 2. 核心决策

### 2.1 压缩结果是时间线事件，不是卡片

压缩完成后显示为一条直接贴在 timeline 中的扁平 disclosure，结构对齐 `GitCommitEvent`：

- 默认只显示图标、标题、元信息和展开箭头。
- 点击整行后，在水平分隔线下展开压缩摘要。
- 不使用外层卡片、独立边框、阴影、徽标头、指标卡片或摘要内层卡片。
- 不保留原 `context-compaction-card` 的绿色描边 flash；新条目只使用 timeline 通用入场反馈。

### 2.2 标题由专用压缩模型调用生成

标题和摘要来自同一次压缩 LLM 请求，不增加第二次模型调用。

从领域语义看，这里需要的是可靠的结构化输出，不是可执行业务工具。当前 `llm.Provider` 没有跨 provider 统一的 JSON Schema structured output，而 tool calling 已被 OpenAI、Kimi、DeepSeek 适配器统一支持，因此本期使用 tool calling 作为结构化输出通道，对齐 `git_commit` 的生成范式。

新增的 `compact_context` 只是一份局部输出 schema：

- 只在 `contextcompact.Compactor.Compact` 发起的专用 LLM 请求中提供。
- 不加入 Runtime 的业务工具注册表。
- 不加入日常 Agent 的 tool definitions。
- 不出现在普通 ReAct 循环、工具活动记录或 `tool.started / tool.completed` 事件中。
- tool call 参数只被 Compactor 解析，不执行外部动作，也不写入持久 transcript。

因此它会被“专用压缩请求的 LLM”看到，但不会暴露给日常工作的主 Agent。

### 2.3 手动与自动压缩共用标题生成

手动和自动压缩继续调用同一个 `Compactor.Compact` 原语：

- 手动压缩：REST endpoint 调用 Compactor，响应同时返回 snapshot 和 compaction。
- 自动压缩：Runtime 阈值触发 Compactor，完成后发出 `context.compacted` SSE。
- 两条路径都持久化同一个必填 `title` 字段，并生成相同的 timeline item。

### 2.4 手动压缩设置 12k Token 收益下限

手动压缩是否值得执行，不按整个上下文窗口的占用率判断，而按 Compactor 实际能够替换的旧 transcript 判断。该值复用正式压缩前的裁剪与分段规则：先移除已被新版替代的渲染图片，再排除需要长期保留的用户指令和最近两轮工具调用。

- `compactable_tokens >= 12000`：允许手动压缩。
- `compactable_tokens < 12000`：按钮禁用并置灰，后端同时拒绝直接请求。
- 12k 是固定产品阈值，由后端通过快照协议下发；前端不自行根据窗口比例推断。
- 自动压缩仍只由现有 85% 窗口阈值触发，不受手动压缩收益下限限制。

这样可以避免在窗口总量看似较高、但绝大多数内容属于系统提示词、运行时资源或必须保留的新近交互时，发起一次几乎无法回收 token 的模型调用。

## 3. 压缩模型输出协议

### 3.1 局部 Tool Schema

工具名称：`compact_context`。

```json
{
  "name": "compact_context",
  "description": "Return a short timeline title and the durable context summary.",
  "parameters": {
    "type": "object",
    "additionalProperties": false,
    "required": ["title", "summary"],
    "properties": {
      "title": {
        "type": "string",
        "minLength": 1,
        "maxLength": 48
      },
      "summary": {
        "type": "string",
        "minLength": 1
      }
    }
  }
}
```

模型必须调用 `compact_context` 恰好一次。Compactor 不接受正文中的 JSON、Markdown code fence 或普通自然语言作为协议降级路径。

### 3.2 Title 约束

模型返回的 `title` 只包含标题正文，不包含 UI 前缀 `compact:`。

约束如下：

- 概括被压缩上下文中最主要的任务、阶段或决策主题。
- 使用与主要对话一致的语言。
- 推荐 6～24 个中文字符；后端硬限制为 1～48 个 Unicode 字符。
- 单行，不允许换行、制表符、HTML、Markdown 标题或列表标记。
- 不以句号结尾，不写“上下文压缩完成”“对话摘要”等无区分度文案。
- 不包含 `compact:`，该前缀仅由前端渲染。

Title 是展示元数据，不进入 `<context_summary>`，避免给后续主 Agent 增加无意义上下文。

### 3.3 Summary 约束

`summary` 继续使用现有固定 Markdown 五小节：

```markdown
## 目标与意图
## 已完成改动
## 关键决策
## 未决问题
## 下一步
```

现有摘要保真、安全边界、附件引用、DOM selection 和未决事项保留规则不变。最终写入持久 transcript 的仍然只有：

```xml
<context_summary>
{summary}
</context_summary>
```

### 3.4 校验与兜底

Compactor 保持“单次 model call”约束，不因标题不合法发起第二次请求。

- 必须恰好返回一次 `compact_context` tool call。
- `summary` 缺失或为空：本次压缩失败，不替换 transcript。
- `title` 为空、超长或包含非法控制字符：摘要仍可使用，标题统一回退为 `整理当前任务上下文`。
- 压缩区为空、无需调用 LLM：使用现有 empty summary，同时使用同一个回退标题。
- tool 名错误、tool call 多于一次或参数结构错误：本次压缩失败，不做文本协议兼容。

tool schema 本身的 token 开销必须计入 `compactRequestTokens`，避免输入裁剪只估算 prompt 和 transcript、遗漏本次新增的工具定义。

## 4. 后端领域模型与持久化

### 4.1 Compactor Result

`contextcompact.Result` 增加必填字段：

```go
type Result struct {
    Title                  string
    Summary                string
    Messages               []llm.Message
    BeforeTranscriptTokens int
    AfterTranscriptTokens  int
    DroppedInputTokens     int
}
```

Compactor 从 `GenerateResponse.ToolCalls[0].Args` 读取 `title` 和 `summary`，不再从 `response.Text()` 读取摘要。

### 4.2 ContextCompaction

`model.ContextCompaction` 增加：

```go
Title string `json:"title"`
```

`title` 是必填新协议字段，不使用 `omitempty`，不保留无标题旧结构兼容分支。

手动和自动记录路径都必须写入 `result.Title`：

- `ContextWindowService.Compact`
- `workflowExecution.recordAutoCompaction`

### 4.3 SQLite

`context_compactions` 表和 `contextCompactionPO` 增加必填 `title TEXT NOT NULL`，PO 与 model 的双向映射同步增加字段。

项目仍处于开发期，本次直接切换为新结构：

- 不为历史无标题记录设计 nullable 字段或运行时回填。
- 不提供双读、默认读或兼容映射。
- 本地旧开发数据库按项目既有开发流程重建，Schema 权威定义直接反映新字段。

所有使用位置参数写入 `context_compactions` 的测试夹具必须切换为显式列名或同步新字段，避免列顺序导致误写。

### 4.4 History 合流

`contextCompactionHistoryEntry` 的 data 增加必填 `title`。Thread history 返回的压缩记录必须始终包含标题；旧的无标题 history shape 由前端拒绝，不进行前端兜底。

## 5. 公开事件与 API

### 5.1 REST Response

`POST /threads/:id/compact` 路径不变，`CompactContextResponse.compaction` 增加 `title`：

```json
{
  "snapshot": {},
  "compaction": {
    "id": "cmp_xxx",
    "thread_id": "thr_xxx",
    "project_id": "prj_xxx",
    "trigger": "manual",
    "title": "收敛上下文协议与前端实现",
    "summary": "## 目标与意图\n...",
    "before_tokens": 53740,
    "after_tokens": 25559,
    "max_tokens": 65536,
    "reclaimed_tokens": 28181,
    "duration_ms": 3600,
    "created_at": 1789634520
  }
}
```

### 5.2 SSE Event

`context.compacted` 事件名和外层结构不变，内部 `compaction.title` 改为必填。

后端 `ValidatePublicEvent` 和前端 `parsePublicEvent` 必须同步验证：

- `title` 为非空安全单行字符串。
- title 不超过 48 个 Unicode 字符。
- 缺少 title 的旧事件直接拒绝。
- 其它数字字段继续要求非负整数，`max_tokens` 必须大于 0。

本次不为手动压缩新增独立 SSE 通道。手动压缩以 REST response 为完成事实；自动压缩继续以当前 Run SSE 为完成事实，避免同一操作从 REST 和 SSE 重复插入。

### 5.3 Context Window 压缩能力字段

REST 快照与 `context.window.updated` SSE 同步增加两个必填整数：

```json
{
  "compactable_tokens": 14320,
  "compact_threshold_tokens": 12000
}
```

- `compactable_tokens` 必须为非负整数，表示当前旧 transcript 中实际可被替换的估算 token。
- `compact_threshold_tokens` 必须为正整数，当前固定为 `12000`。
- 前端只比较这两个服务端事实来决定按钮是否可用。
- 缺少字段的旧快照和旧 SSE 事件直接拒绝，不保留兼容分支。

`POST /threads/:id/compact` 在模型调用前重新计算 `compactable_tokens`。若低于阈值，返回 `COMPACT_BELOW_THRESHOLD`，不调用压缩模型、不替换 transcript，也不创建压缩记录。该后端校验用于处理多窗口、事件延迟和绕过 UI 直接调用等竞态。

## 6. Timeline 即时展示修复

### 6.1 Bug 原因

当前 `contextWindowStore.compact` 在 POST 成功后只执行：

```text
result.snapshot → Context Window Store
```

它没有把 `result.compaction` 写入 `useRunStore.sessions[threadId].timelineItems`。刷新后 Thread History 合流读取到持久化记录，才生成 `context_compaction` timeline item，因此表现为“压缩成功但必须刷新才显示”。

### 6.2 修复原则

建立一份从公开 `ContextCompaction` 到 `ContextCompactionTimelineItem` 的共享纯映射，供三条入口复用：

1. 手动压缩 REST 成功。
2. 自动压缩 `context.compacted` SSE。
3. 页面刷新后的 Thread History hydration。

映射结果使用稳定 ID：

```text
context-compaction:{compaction.id}
```

所有入口按该 ID 执行 upsert：存在则替换，不存在则追加。这样即使稍后 history reload，也不会产生重复条目。

### 6.3 手动压缩完成时序

```text
用户点击压缩
  → Context Window Store 标记 compacting
  → POST /threads/:id/compact
  → 后端完成压缩、持久化并返回 snapshot + compaction
  → 更新 Context Window snapshot
  → 将 compaction 映射并 upsert 到当前 thread timeline
  → 清除 compacting
  → 关闭上下文弹窗
```

只要 POST 已成功返回，timeline item 就必须在同一前端操作内出现，不依赖刷新、history refetch 或新的 SSE 连接。

如果 timeline session 尚不存在，按 `runStore` 的 idle session 初始结构创建，再插入压缩条目；不得因为当前没有 active run 而丢弃手动压缩事件。

## 7. 正式前端呈现

### 7.1 总体结构

正式组件继续命名为 `ContextCompactionActivity`，但其 DOM 与视觉层级改为对齐 `GitCommitEvent` 的扁平 disclosure：

```text
[绿色 Gauge]  compact: <title>                              [>]
               时间 | 触发方式 | 窗口占用与回收量
               ─────────────────────────────────────────────
               展开的 Markdown 压缩摘要
```

明确禁止：

- 外层卡片背景、边框、圆角和阴影。
- “上下文已压缩”绿色徽标。
- 两个并列指标卡片。
- “压缩摘要”二级容器标题。
- 摘要的内层边框、背景卡片、渐隐遮罩和悬浮展开按钮。
- 独立的成功 flash 动画。

### 7.2 图标

- 使用 Lucide `Gauge`，延续上下文窗口的指标语义。
- 成功态完全对齐 commit：圆形容器、`border-success/35`、`bg-success-soft`、`text-success`。
- 图标尺寸、stroke width 和 26px 容器与 `GitCommitEvent` 一致。

### 7.3 标题

展示格式：

```text
compact: <title>
```

- `compact:` 由前端固定添加，使用次级文字色。
- `<title>` 使用 `text-text-900` 和与 commit 相同的字重。
- 整行单行省略，不允许标题增高时间线条目。
- 无 title 的数据不进入组件；不在 UI 层生成兜底标题。

### 7.4 元信息

固定顺序：

```text
YYYY-MM-DD HH:mm | 手动触发 | 窗口 82% → 39% 回收 41.8k Token
```

其中 `|` 在正式 UI 中继续使用 commit 的细竖向分隔线，不渲染字面竖线字符。

格式和颜色：

- 时间：沿用 commit 的本地时间格式。
- 触发方式：`手动触发` 或 `自动触发`。
- `窗口`、压缩前百分比、箭头、`回收`、`Token`：中性色。
- 箭头 `→` 两侧保留一个空格。
- 压缩后百分比：commit success 绿色。
- 仅回收 token 的数值部分（如 `41.8k`）：commit danger 红色。
- `reclaimed_tokens` 为非负整数，UI 不再显示负号。
- 回收量固定格式化为一位小数的 k 值：`(tokens / 1000).toFixed(1) + "k"`。
- `duration_ms` 继续保留在协议和持久化中供诊断使用，但不在本次 UI 中展示。

“窗口占用与回收量”作为不可从中间拆分的元信息组：空间不足时整组换到下一行，禁止出现“窗口 82% → 39%”留在上一行、只有“回收 41.8k Token”落到下一行的孤行。

窗口百分比继续按以下规则计算并取整：

```text
before = max_tokens > 0 ? round(before_tokens / max_tokens * 100) : 0
after  = max_tokens > 0 ? round(after_tokens  / max_tokens * 100) : 0
```

### 7.5 展开摘要

- 整个标题行是 button，点击切换 `aria-expanded`。
- 使用 `ChevronRight`，展开时旋转 90°。
- 复用 `TimelineDisclosure` 的高度与透明度过渡。
- 摘要默认完全不渲染或不可见；点击后才展示。
- 展开区从图标右侧的文本列开始，与 commit 的 `ml-[34px]` 对齐。
- 顶部只有一条 `border-t border-border` 水平分隔线。
- 内容直接使用 `MarkdownMessage` 渲染现有五小节，不增加内层容器。
- 展开后显示完整摘要，不使用旧版 300px 裁切、渐隐或二次“展开全文”。
- 支持键盘聚焦和 reduced-motion；焦点样式与现有 timeline button 一致。

## 8. 前端类型与映射

### 8.1 API Type

`frontend/src/api/types.ts`：

```ts
export interface ContextCompaction {
  // existing fields...
  title: string;
  summary: string;
}
```

### 8.2 Timeline Item

`ContextCompactionTimelineItem` 增加必填：

```ts
title: string;
```

不定义 optional title，也不在组件中使用 `??` 回退。

### 8.3 共享映射

建议提供纯函数：

```ts
contextCompactionTimelineItem(compaction: ContextCompaction): ContextCompactionTimelineItem
```

SSE reducer、history hydrator 和手动 compact store 必须复用这份映射，避免三处独立复制字段后再次发生协议漂移。

## 9. 失败与边界

- Compactor 请求失败：不替换 transcript、不落库、不插入 timeline，前端沿用现有全局错误提示。
- transcript 已替换但持久化失败：保持现有事务顺序要求，不对前端返回成功。
- 手动 POST 成功但 timeline upsert 重复执行：依靠稳定 item ID 覆盖，不重复展示。
- 自动 SSE 重连重放：依靠同一稳定 ID upsert。
- History 中缺少 title：视为旧协议记录并忽略，不展示无标题事件。
- 超长摘要：展开后随 timeline 正常流动，不创建内部滚动区。
- 窄宽度：标题始终单行截断；元信息允许按分组换行；窗口与回收数据不得相互拆开。
- `reclaimed_tokens = 0`：仍显示 `回收 0.0k Token`，不省略。
- `before_tokens / after_tokens = 0`：仍显示对应 `0%`。

## 10. 实现影响面

后端：

- `backend/internal/contextcompact/compactor.go`
- `backend/internal/contextcompact/compactor_test.go`
- `backend/internal/prompt/prompts/command/compact.md`
- `backend/internal/model/context_compaction.go`
- `backend/internal/model/public_event.go`
- `backend/internal/service/context_window.go`
- `backend/internal/service/run.go`
- `backend/internal/service/thread.go`
- `backend/internal/store/sqlite/po.go`
- `backend/internal/store/sqlite/context_compaction_store.go`
- `backend/migrations/` 中的 context compaction Schema 权威定义及相关测试夹具

前端：

- `frontend/src/api/types.ts`
- `frontend/src/api/sse.ts`
- `frontend/src/stores/contextWindowStore.ts`
- `frontend/src/stores/runStore.ts` 或等价 timeline upsert 边界
- `frontend/src/features/agent/eventReducer.ts`
- `frontend/src/features/agent/historyHydrator.ts`
- `frontend/src/features/agent/ContextCompactionActivity.tsx`
- `frontend/src/index.css` 中旧 `context-compaction-card` 动画
- 对应 reducer、hydrator、store、SSE 与组件测试

## 11. 测试要求

### 11.1 后端

- Compactor 每次最多发起一次 LLM 调用。
- 请求只携带局部 `compact_context` schema，并要求 `title + summary`。
- Runtime 的日常业务工具列表不包含 `compact_context`。
- 正确解析一次合法 tool call，并把 title、summary 写入 Result。
- 普通文本、错误 tool 名和多个 tool call 被拒绝，不走文本兼容。
- 非法 title 使用固定回退标题，不增加第二次模型调用。
- 空压缩区不调用 LLM，并返回固定标题和 empty summary。
- tool schema token 被计入压缩请求预算。
- 手动和自动路径都持久化 title。
- SQLite round-trip、项目快照及 fixture 包含 title。
- Thread History 合流包含 title。
- `context.compacted` 公开事件接受合法 title，拒绝缺失、空值、换行和超长 title。

### 11.2 前端

- SSE parser 接受新协议，拒绝无 title 的旧事件。
- SSE reducer、history hydrator 和 REST response 生成完全相同的 timeline item。
- 手动压缩 POST 成功后无需刷新即可插入 timeline。
- 同一 compaction ID 多次进入时只保留一条。
- Timeline session 不存在时也能插入手动压缩记录。
- 默认只显示扁平标题行，摘要不可见。
- 点击整行后通过水平分隔线展开完整 Markdown 摘要。
- 标题显示为 `compact: <title>`。
- `Gauge` 使用与 commit 相同的绿色成功态。
- 元信息顺序、箭头空格、after 百分比绿色、仅 token 数值红色符合本规范。
- `0%` 和 `0.0k Token` 不被省略。
- DOM 中不存在旧富卡片的指标卡、徽标头或嵌套摘要卡片。
- 可压缩旧 transcript 少于 12k Token 时，手动压缩按钮禁用并置灰。
- 前端 SSE 校验要求压缩能力字段完整，不接受缺少字段的旧协议。

## 12. 验收标准

1. 手动压缩成功后，当前 timeline 立即出现一条压缩事件，无需刷新。
2. 自动压缩完成后，事件仍由 SSE 实时出现，且不会与 history 重放形成重复项。
3. 所有新压缩记录都有由专用压缩 LLM 同次生成的 title。
4. `compact_context` schema 只存在于 Compactor 的局部请求，不出现在主 Agent 的日常工具定义中。
5. 压缩事件视觉结构与 commit 一致：扁平、整行可展开、展开区仅由水平分隔线隔开。
6. 标题、图标、元信息、颜色和数字格式严格符合第 7 节。
7. 后端模型、持久化、REST、公开事件、前端类型、SSE 校验、history hydration 与测试一次性切换到新协议，不保留旧结构兼容层。
8. 可压缩旧 transcript 未达到 12k Token 时，前端无法点击手动压缩；绕过前端的请求也会在模型调用前被后端拒绝。

## 13. 非目标

- 不改变 85% 自动压缩阈值和六桶 token 估算协议；12k 下限只约束手动压缩。
- 不改变摘要五小节的业务语义。
- 不让主 Agent 决定何时压缩，也不新增 `/compact` Agent 命令。
- 不新增手动压缩 SSE 通道。
- 不展示压缩模型、调用次数、耗时或内部 tool call。
- 不恢复压缩前 transcript，也不改变纯破坏性替换策略。
