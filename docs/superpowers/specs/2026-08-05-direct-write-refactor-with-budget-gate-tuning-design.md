# 直写落地 + 文件为真相 · 含运行预算与门控调优 · 合并设计文档

> 状态：设计定稿，待实施
> 日期：2026-08-05
> 合并来源：`docs/v2/80-direct-write-and-git-refactor.md`（架构重构）+ 运行预算与完成门控调优（前置缓解）
> 关联：废弃 `docs/v2/30-agent-pipeline-v2.md` 中的 Run 级 staging→commit 事务模型
> 执行要求：**分两阶段、各自一次性执行、执行顺序自定、全程无需用户干预、改动精准**

---

## 0. 背景与根因（一条主线贯穿两阶段）

现行架构把每个 Run 当作数据库事务：产物先写入 `.staging/<run_id>` 私有沙盒，
通过 Completion Gate 后才原子提交（`backend/internal/workflow/transaction.go`）。
该设计在正确性上严谨，但暴露两个真实问题：

1. **失败即全部丢弃**：一次 run 已生成 outline + design + 10 页 spec，仅因
   `RUNTIME_BUDGET_EXCEEDED` 在进入 HTML 阶段前触顶，12 份成果被整体清空——"忙 4 分钟、零产物、无从续做"。
2. **DB 与文件双写隐患**：`slides` 表镜像文件内容（position/title/layout/path），读接口完全信任 DB，
   一旦第二条写路径改动磁盘，DB 元数据与磁盘悄然失联。

同一根因（预算触顶 + 事务丢弃）指向两个**互补**的解法，本文件合并为一份、分阶段落地：

| 阶段 | 定位 | 解决什么 | 是否动架构 |
|---|---|---|---|
| **Phase A · 预算与门控调优** | 症状缓解、低风险先行 | 降低"因预算终止"频率、精简过度/重复门控、省 image token | **否**（局部调优） |
| **Phase B · 直写落地 + 文件为真相** | 根因根治 | 删事务、产物直写磁盘、文件成为唯一内容真相、DB 专注编排 | **是**（80 原定重构） |

**先 A 后 B 的理由**：A 独立、可回退、立即降低触发频率并省 token；B 是硬骨头重构，根治"丢弃"。
A 不改变任何架构不变量，B 落地时无需回退 A 的任何改动（§5 已逐项核对衔接）。

对标 Codex / Claude Code：真相源是**文件系统 + git**，无"内容镜像 DB"。本项目分阶段靠拢该模型；
git 安全网与"废弃 versions 表"留待后续与恢复 UI 一起做。**全程不引入 run_command，agent 无 shell 能力。**

---

## 1. 目标架构（真相边界，Phase B 达成）

| 维度 | 唯一真相源 | 说明 |
|---|---|---|
| **内容** | 磁盘文件 | outline.json / design.json / slides/<id>/spec.json / index.html |
| **内容历史 / 回滚** | `versions` 表（**过渡期保留**） | 后续迁移到 git，届时才废弃该表 |
| **运行时编排** | DB (SQLite) | runs / run_events / run_contexts / steering_inbox / idempotency_records |

原则：
- **唯一内容写路径**：类型化工具（`write_ppt` 等）直写磁盘并同步运行时状态与版本快照。
- **DB 不再镜像内容**：列表/导航所需的 position/title 改为从文件投影。

---

## 2. 范围与非目标（红线）

### 2.1 Phase A 范围（非架构）
- 运行预算维度增删与阈值调整（`domain.go` + `runtime.go`）
- 连续工具失败上限 4 → 5（`domain.go`）
- 完成门控移除 3 条分支：`RUNTIME_BUDGET_EXCEEDED` / `CHANGESET_REQUIRED` / `TARGET_OUT_OF_SCOPE`（`completion.go`）
- `render_slide` 高清截图回灌改为按需（`render_tool.go`）
- 错误码文案微调 + 相关测试同步

### 2.2 Phase B 范围（架构，即 80 原 6 要点）
- 废弃 Run 级 staging→commit 事务（保留 versions 快照写入）
- 内容真相收归文件系统、DB 退出内容、读接口文件投影
- 门控证据来源改造（磁盘观测）、按项目串行化 run

### 2.3 全局非目标（两阶段均严禁）
- **不**引入 run_command / shell 能力，**不**在交互界面或工具入参暴露内部逻辑细节
- **不**在 Phase A 触碰事务 / DB schema / 读路径 / 门控证据源机制（这些是 Phase B 的事）
- **不**在 Phase B 引入超出本文件所列 6 要点之外的新架构改造（精准，不扩张）
- **不**本轮迁移 git / 废弃 versions 表（过渡期保留）

---

## 3. Phase A · 运行预算与门控调优（file-by-file）

### 3.1 `backend/internal/workflow/domain.go` — 预算硬移除 + 调值

`RuntimeBudget` 结构体（L150-159）删除 `MaxToolCalls`、`MaxTokens` 两字段；
`DefaultRuntimeBudget()`（L161-167）改值：

```go
type RuntimeBudget struct {
    MaxTurns                    int
    MaxDuration                 time.Duration
    MaxConsecutiveToolFailures  int
    MaxIdenticalGateRejections  int
    ContextCompactionThreshold  int
    SimpleUpgradeToolRoundTrips int
}

func DefaultRuntimeBudget() RuntimeBudget {
    return RuntimeBudget{
        MaxTurns: 128, MaxDuration: time.Hour,
        MaxConsecutiveToolFailures: 5, MaxIdenticalGateRejections: 3,
        ContextCompactionThreshold: 24000, SimpleUpgradeToolRoundTrips: 6,
    }
}
```

- `ContextCompactionThreshold` **保留**（压缩阈值，与硬上限无关，仍需 token 计数驱动压缩）。
- `CodeBudgetExceeded = "RUNTIME_BUDGET_EXCEEDED"`（L191）**保留**（轮次/时长触顶仍用它终止）。

### 3.2 `backend/internal/workflow/runtime.go` — 移除两维检查

- `budgetExhausted`（L1031-1036）删掉 token 与工具调用两行，只留 turns/duration：
  ```go
  func (r *Runtime) budgetExhausted(state *runtimeState) bool {
      return state.turns >= state.budget.MaxTurns ||
          time.Since(state.started) >= state.budget.MaxDuration
  }
  ```
- 门控上下文构造处（L807-808）删除 `BudgetExhausted: r.budgetExhausted(state),` 赋值行。
- **保留不动**：`state.tokens` 累加（压缩逻辑 L1127 仍用）、`state.toolCalls` 计数（策略升级 L396 仍用）、
  `checkBudget` 主循环调用（L303-308，现仅 turns/duration 触发）、`MaxConsecutiveToolFailures` 检查（L400-401，阈值提到 5）。

### 3.3 `backend/internal/workflow/completion.go` — 移除 3 条门控分支

- 删 `CompletionContext.BudgetExhausted bool` 字段（L53）。
- 删预算门控分支（L268-270，`if ctx.BudgetExhausted { ... RUNTIME_BUDGET_EXCEEDED ... }`）。
- 删 `CHANGESET_REQUIRED` 分支（L281-283，`if ctx.Changes.Count() == 0 { ... }`）。
- 删 `TARGET_OUT_OF_SCOPE` 门控 for 循环（L284-288）。
- **保留不动**：同块内（`if ctx.Strategy != StrategyChat`，L271-279）的 `STAGING_REQUIRED` 与 baseline 校验。

移除依据：`RUNTIME_BUDGET_EXCEEDED` 不再作收尾闸门；`CHANGESET_REQUIRED` 会误伤"只输出内容供确认"的正当写任务；
`TARGET_OUT_OF_SCOPE` 工具层已拦（`ppt_tools.go:33/70/131`、`tools.go:242/248`、`render_tool.go:379`），门控层纯属重复。

### 3.4 `backend/internal/workflow/render_tool.go` — VISUAL 高清截图改按需

拆开 VISUAL 捆绑的两样成本：**渲染 + 客观诊断（溢出/裁切/console 报错/资源失败）保留为硬闸门**（只耗 CPU）；
**高清截图回灌模型（`Detail:"high"`）改为按需**（大额 image token）。

- Schema（L361-369）新增可选入参，面向业务意图命名：
  ```go
  "visual_review": map[string]any{
      "type": "boolean",
      "description": "Request the high-detail screenshot back for visual judgement. Omit for a lightweight diagnostics-only render.",
  },
  ```
- 回灌逻辑（L471-474）改为：`len(blocking) > 0`（出问题需模型判断）或 `visual_review == true`（显式要看）才附高清图，否则只回文字诊断：
  ```go
  observationRaw, _ := json.Marshal(observationData)
  parts := []llm.ContentPart{{Type: "text", Text: string(observationRaw)}}
  visualReview, _ := input.Args["visual_review"].(bool)
  if len(blocking) > 0 || visualReview {
      parts = append(parts, llm.ContentPart{Type: "image", ImageRef: screenshotRef, MIMEType: "image/png", Detail: "high"})
  }
  result.ObservationParts = parts
  ```
- bool 入参解析沿用项目惯例 `v, _ := input.Args["key"].(bool)`（见 `public_events.go:342/345`），**不引入新 helper**。
- **证据账本、blocking 硬闸门、`newEvidence("render", ...)` + `Materialization`（L475-488）一字不动**——省的是 image token，不动安全网。

### 3.5 `backend/internal/model/agent_error.go` — 文案微调

`RUNTIME_BUDGET_EXCEEDED`（L80）**保留定义**；因 token 维度移除，SafeMessage
`"运行达到资源上限，未完成的修改不会提交。"` → `"运行达到轮次或时长上限，未完成的修改不会提交。"`
（`ModelMessage` 不变）。`TARGET_OUT_OF_SCOPE`（L60）定义**保留**（工具层仍用）；`CHANGESET_REQUIRED` 本无独立定义，无需处理。

### 3.6 Phase A 测试触点

已核查：无后端 `_test.go` 直接断言预算字段/被删三 code；前端零引用三 code。仅需处理：
- `ppt_tools_test.go` `TestRenderSlideUsesStagedHTMLAndProducesEvidence`（L254）硬断言"render 永远 2 个 ObservationParts 含图"，
  改为不传 `visual_review` 时断言 `len==1 && [0].Type=="text"`；**新增** `TestRenderSlideAttachesScreenshotOnVisualReview`
  传 `visual_review:true`，断言 `len==2 && [1].Type=="image"`。
- `completionContext` helper（`ppt_tools_test.go:602`）未设 `BudgetExhausted`（已核查），删字段后无需改。

### 3.7 Phase A 执行顺序（编译依赖驱动，一次性）

1. `domain.go` 删字段/改值 → 2. `runtime.go` 删两行/删赋值 → 3. `completion.go` 删字段 + 三分支 →
4. `render_tool.go` 加入参/改按需 → 5. `agent_error.go` 微调 → 6. `ppt_tools_test.go` 改断言 + 新增用例 →
7. `cd backend && go build ./... && go test ./...` 全绿收尾。

---

## 4. Phase B · 直写落地 + 文件为真相（file-by-file，即 80 原 6 要点）

### 4.1 要点 1 · 废弃 Run 级 staging→commit 事务（保留版本快照）
删除 `backend/internal/workflow/transaction.go` 及其在 runtime / completion / render_tool / ppt_tools 的依赖。
产物直接落地项目目录，不再进 `.staging/<run_id>`，不再"失败即回滚丢弃"。
**关键耦合**：当前 versions 快照在事务提交流程（`backend/internal/service/workflow_commit.go:60-92` 的 `addVersion`）写入；
删事务后**必须把这段摘出并挂到直写路径**——类型化工具直写磁盘成功后顺带记一条 version 快照。过渡期 versions 表继续工作，不随事务删。

### 4.2 要点 2 · 内容真相收归文件系统
outline.json / design.json / spec.json / index.html 成为唯一内容真相；唯一写路径是类型化工具，直写磁盘并同步运行时状态与版本快照。

### 4.3 要点 3 · DB 退出内容、专注运行时
删 `slides` 表冗余字段 `spec_path`/`html_path`/`position`/`title`/`layout` 及 `projects` 的 `design_path`/`outline_path`（路径由协议推导，见 §6）。
DB 仅保留 runs / run_events / run_contexts / steering_inbox / idempotency_records 及过渡期 versions 表。

### 4.4 要点 4 · 读接口改为从文件投影
`GET /slides`、`GET /slides/:id` 等不再信任 DB 内容字段，改从 outline.json（`slide_order` 决定顺序）+ spec.json（title/layout）动态读取，消除"接口返回旧值、预览显示新值"。

### 4.5 要点 5 · Completion Gate 证据来源改造
门控不再依赖 Transaction 的 ChangeSet，改为直接观测**磁盘实际变更 + 运行时状态**计算证据要求与 ChangeSet（`completion.go`）。渲染新鲜度仍用 spec.json 的 `revision` 与 html 的 source hash 比对。
**衔接约束（见 §5）**：改造时 Phase A 已移除的三条分支不得复活；VISUAL 客观诊断硬闸门与 `visual_review` 按需图行为保留。

### 4.6 要点 6 · 并发安全：按项目串行化 run
删事务乐观锁后，用"同一项目同一时刻最多一个活跃 run"防并发直写冲突。复用现有 `ErrRunActive`/`RUN_ACTIVE`（`backend/internal/service/errors.go`），
将其从"挡手动编辑"扩展到"挡新 run 创建"——`RunService.CreateRun`（`backend/internal/service/run.go:101`，当前无此检查）在项目存在活跃 run 时返回 409 `RUN_ACTIVE`。

### 4.7 Phase B 实施顺序（80 原建议）
1. **要点 1 / 2 / 5 一组**（强耦合硬骨头）：删事务、改直写、改门控证据源，同批完成；务必先把 versions 快照写入从事务流程摘迁到直写路径。
2. **要点 3 / 4**：DB schema 迁移 + 读接口文件投影。
3. **要点 6**：CreateRun 加 active-run 检查，复用 RUN_ACTIVE。

---

## 5. 跨阶段衔接与不变量（防止 B 踩踏 A）

| 衔接点 | 约束 |
|---|---|
| 门控三分支移除 | Phase B 要点 5 重写门控证据源时，**不得复活** `RUNTIME_BUDGET_EXCEEDED`/`CHANGESET_REQUIRED`/`TARGET_OUT_OF_SCOPE` 三分支 |
| `render_tool.go` | Phase B 要点 1 移除该文件的 Transaction 依赖时，**保留** Phase A 的 `visual_review` 入参与按需图逻辑 |
| `RUNTIME_BUDGET_EXCEEDED` 语义 | Phase A：文案去 token 语义、仍"未提交"；**Phase B 事务删除后**，触顶时部分产物已直写磁盘，SafeMessage 再改为"已保留部分产物、可续做"，`ModelMessage` 同步（不再 staging） |
| VISUAL 证据账本 | Phase A 只改是否回灌图；Phase B 渲染新鲜度改用 spec.json revision + html source hash——两者作用面不同，B 落地保持按需图不变 |
| `MaxConsecutiveToolFailures` 等预算 | Phase B 不涉及预算，A 的调值保持 |

---

## 6. 路径推导协议（Phase B 删字段依据）

路径完全由 slide_id 机械推导，DB 存 path 为纯冗余（`backend/internal/model/slidepath.go`）：

```
SlideDir(id)      = "slides/" + id
SlideSpecPath(id) = "slides/" + id + "/spec.json"
SlideHTMLPath(id) = "slides/" + id + "/index.html"
outline           = "outline.json"
design            = "design.json"
```

顺序/结构真相在 outline.json 的 `slide_order []string` 与 `sections`（`backend/internal/spec/types.go`）。故 slides 表内容字段均可由文件重建。

---

## 7. 统一执行顺序与验证

1. **Phase A 一次性执行**（§3.7）→ `go build ./... && go test ./...` 全绿 → 提交一个独立 commit（可单独回退）。
2. **Phase B 分三组执行**（§4.7）→ 每组后 `go build ./... && go test ./...` → 前端受影响处（读接口投影）跑 `npm run lint && build && test`。
3. 两阶段均**无决策点**：取舍已在本文件锁定，下游 agent 按精确编辑执行；失败按报错定位残留引用逐个修正，不回退方案。

---

## 8. 验收标准

**Phase A**
- `go build ./...` / `go test ./...` 通过；`RuntimeBudget` 无 token/工具调用维度；默认轮次 128、时长 1h、连续失败 5。
- 门控不再产出 `RUNTIME_BUDGET_EXCEEDED`/`CHANGESET_REQUIRED`/`TARGET_OUT_OF_SCOPE`；其余 11 条行为不变。
- `render_slide` 默认只回文字诊断；blocking>0 或 `visual_review=true` 才附高清图；证据账本与硬闸门不变。

**Phase B**
- `transaction.go` 删除，产物直写磁盘，失败不再整体丢弃；versions 快照挂到直写路径且过渡期正常。
- `slides`/`projects` 冗余字段删除，读接口从文件投影，无 DB/磁盘不一致。
- 门控证据源改磁盘观测，且 §5 衔接约束全部满足（三分支未复活、visual_review 保留）。
- CreateRun 对活跃 run 返回 409 `RUN_ACTIVE`。

---

## 9. 风险与回退

| 风险 | 缓解 |
|---|---|
| 删预算维度后长任务无 token/工具数兜底 | 轮次 128 + 时长 1h 双重兜底；Phase B 后失败不再丢弃，风险进一步下降 |
| 按需图导致模型"看不到"渲染结果 | blocking 仍强制附图；模型可传 `visual_review=true`；客观诊断始终回文字 |
| Phase B 删事务引入直写并发冲突 | 要点 6 按项目串行化 run（RUN_ACTIVE） |
| Phase B 摘迁 versions 快照遗漏 | 要点 1 明确"直写成功后顺带记快照"，过渡期 versions 表为历史/回滚唯一机制，测试覆盖 |
| B 踩踏 A 的调优 | §5 衔接表逐项约束，B 落地无需回退 A |

回退：Phase A 集中于 4 源文件 + 1 测试文件，`git revert` 单 commit 完整回退；Phase B 按三组分别提交，可逐组回退。
