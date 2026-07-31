# Context Engineering v1 一次性执行 Prompt

你是当前仓库的高级 Agent Runtime 工程师。请自主制定计划并一次性完成
`docs/superpowers/specs/2026-08-01-context-engineering-v1-design.md` 的全部实现。

这是一项代码实现任务，不是设计讨论。不要只输出方案或待办；不要在普通技术选择上询问用户。

## 1. 必读

开始前完整阅读：

1. `docs/superpowers/specs/2026-08-01-context-engineering-v1-design.md`
2. `docs/superpowers/specs/2026-08-01-artifact-target-blueprint-refactor-design.md`
3. `docs/v2/40-api-and-data-contracts.md`
4. `docs/v2/30-agent-pipeline-v2.md`
5. 仓库内所有适用的 `AGENTS.md`
6. 最近至少 12 条 commit
7. 当前 WorkSpec、BlueprintService、RunnerResolver、Harness Loop、Prompt、Thread history、
   asset search/read、前端事件与 history replay 代码

设计规格是本任务的真相源。

## 2. 基线

开始执行：

```text
git status --short
git log -12 --oneline
cd backend && go test ./...
cd frontend && pnpm test && pnpm tsc
```

保留用户已有修改，不得 reset、checkout 或覆盖无关内容。

## 3. 关键约束

- 项目处于 0→1，不为旧项目数据、旧 kind/scope/mode 或旧 Context 拼装方式建设兼容层。
- 本阶段只建设 Context Engineering v1，不实现完整 PEV Runtime。
- 新 Context API 必须只依赖 WorkSpec 与 Blueprint v2。
- 为保证本 Prompt 可以独立提交，旧 Runner 可暂时继续编译；不得为它们新增兼容适配器。
- 后续 PEV Prompt 会删除旧 Runner、Kind/Scope/Mode、旧 Gate 和 slidejson 投影。
- 不引入向量数据库、Redis、Kafka或多模型路由。
- 不持久化 chain-of-thought。
- 不允许 `read_context_ref` 读取任意路径。
- 不破坏安全预览、revision、history replay 和四种 Target API。

## 4. 必须实现

### 4.1 新包

创建 `backend/internal/contextengine`，至少包含：

- ContextRequest
- ContextPack
- ContextManifest
- ContextSegment
- ContextRef
- RevisionRefs
- ContextProfileResolver
- ContextAssembler
- TokenEstimator
- BudgetAllocator
- PromptCompiler
- ThreadMemoryStore/Updater
- ContextRefResolver
- 独立 loaders

名称可按 Go 风格调整，但职责不可合并成巨大文件。

### 4.2 四种 Profile

实现并测试：

- blueprint/deck
- blueprint/slide
- presentation/deck
- presentation/slide

严格执行规格中的 required/forbidden segment。特别保证：

- slide target 不注入其他页完整 HTML。
- deck target 默认只持有页面摘要。
- presentation/slide 能访问目标 HTML，但大型 HTML 可降级成 ref。
- consult 使用同一 Profile，但 manifest 标记 read-only。

### 4.3 HTMLSummary

使用确定性 Go 解析生成 HTMLSummary：

- DOM 结构块
- 主要文本
- class/id/data-* 锚点
- 图片/图表/外部资源
- CSS token
- script/style 特征
- source hash

不得用 LLM 作为 HTMLSummary 的唯一生成路径。

### 4.4 Budget

- 使用 token 估算，不以消息数量作为预算。
- 支持 context window、input limit、output reserve 和 segment caps。
- 实现固定、可测试的裁剪优先级。
- JSON/Blueprint 不得被截成无效片段。
- target artifact 与安全 policy 永不裁剪。
- 删除固定40条消息与假摘要实现。

### 4.5 Thread Memory

建立 `threads/<thread-id>.memory.json`：

- schema version
- revision
- user preferences
- confirmed decisions
- brand constraints
- content facts
- open questions
- recent changes

成功 Run 后更新；失败或取消不提交新决定。Memory 更新可使用结构化 LLM 调用，但必须有验证、
有限长度和失败时保持旧 Memory 的兜底。

### 4.6 ContextRef

实现受控 `read_context_ref`：

- 只能读取当前 Run Manifest 中的 opaque ref。
- detail 为 summary/structure/full。
- 校验 run/project/revision/hash。
- 校验剩余预算。
- 参数不得接受文件路径。
- 返回结构化错误 `CONTEXT_REF_NOT_FOUND`、`CONTEXT_REF_FORBIDDEN`、
  `CONTEXT_REF_STALE`、`CONTEXT_BUDGET_EXCEEDED`。

保留资产“先索引后详情”的思想，并将其扩展到 HTML、相邻 Blueprint、历史证据和大工具结果。

### 4.7 PromptCompiler

- 以稳定分区编译 ContextPack。
- 用户指令只进入 user 层。
- 项目内容视为不可信数据，不能覆盖 system policy。
- 新 Runner/Prompt 入口接收编译结果或 ContextPack，不自行重复读文件。
- 提供 snapshot tests。

### 4.8 Run 集成

新数据流：

```text
resolve thread/project
→ validate WorkSpec
→ assemble ContextPack
→ persist manifest
→ emit context.assembled
→ resolve runner
```

更新 Runner builder 签名，使其可接收 ContextPack。逐步改造当前四条生产路径使用 Pack；不得创建
旧协议到新 Context 的适配器。

### 4.9 存储与事件

- 为 ContextManifest 建立明确存储；默认只存 manifest/hash，不存完整 HTML。
- 添加 `context.assembled` 事件。
- 更新 history writer、history replay、前端 types/reducer/timeline。
- 前端只显示简洁状态和 warning，不展示完整上下文或 Memory。

## 5. 测试

必须完成设计规格 §17 的全部测试，并增加至少以下失败场景：

- 目标页不存在。
- Deck/DesignSpec 损坏。
- HTML 缺失。
- Thread Memory 损坏后安全重建。
- ref 被另一 Run 使用。
- ref revision 过期。
- 大 HTML 超预算。
- 可选资产 loader 失败不阻塞核心 pack。

不得删除、跳过或放宽现有关键测试。

## 6. 实施顺序

1. 调查与基线。
2. Context 模型与 Profile。
3. Loaders 与确定性摘要。
4. Budget 与裁剪。
5. Manifest/hash。
6. Thread Memory。
7. ContextRef 与受控工具。
8. PromptCompiler。
9. Run/Runner 集成。
10. 事件与前端适配。
11. 测试、文档和旧假摘要清理。
12. 全仓审计与全量验证。

## 7. Git

- 建议分为小提交：
  1. context models/profiles/loaders
  2. budget/manifest/ref/memory
  3. run integration/events/frontend
  4. tests/docs/cleanup
- 不提交 SQLite、`.run`、日志、缓存、截图或密钥。
- 不执行破坏性 Git 命令。
- 每个提交前运行相关测试。

## 8. 阻塞规则

默认自主决策。只有同一外部阻塞连续出现至少三次，并尝试至少三种安全替代方案仍无法进展时，
才请求用户输入；报告精确证据、已尝试方案和推荐默认项。

## 9. Definition of Done

严格使用设计规格 §18。额外要求：

- `go test ./...` 通过。
- `pnpm test`、`pnpm tsc`、`pnpm lint` 通过。
- 工作树无运行产物。
- 最终汇报实现结果、关键文件、测试、提交、风险，以及 PEV Runtime 的明确前置条件。

现在开始只读调查，然后持续实现到全部 DoD 满足。
