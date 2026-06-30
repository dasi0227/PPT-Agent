---
id: ARCH-HARNESS
title: Agent Harness（ReAct 循环 + 工具化）
status: approved
owner: backend
depends_on: [ARCH-RUNTIME, ARCH-TOOLS, ADR-0007]
verifies: []
---

# Agent Harness（ReAct 循环 + function calling + 工具化）

## 它是什么 / 与 Run 的关系

- **Run** = 一次执行的**生命周期外壳**（状态机 + SSE + 控制输入队列），见 [agent-runtime](agent-runtime.md)。
- **Harness** = Run 内部那个**真正的 agent 大脑**：一个 `思考 → 调用工具 → 观察结果 → 再思考` 的 **ReAct 循环**，由 **function calling** 驱动。

```
Run(pending→running→…→done)
  └── 内部驱动 ──▶ Harness ReAct Loop
                     thought → tool_call → observation → … → finish
```

这层分离让**基础设施（SSE/HITL）**与**认知逻辑（循环/工具/上下文）**解耦，可独立测试、独立扩展。

## ReAct 主循环

```
ctx = 装配初始上下文(scope, mode, 目标产物, 资产索引, 用户指令)
tools = 按 scope/mode 动态裁剪的工具集            // 见「动态工具门控」
for turn in 1..MAX_TURNS:
    decision = LLM.next(ctx, tools)               // function calling：选一个工具 + JSON 参数
    emit SSE: thought(decision.thought?)
    if decision.is_finish:                        // 调用 finish 工具
        emit SSE: done(decision.summary); break
    emit SSE: tool_call(name, args)
    obs = Tools.execute(decision.tool, decision.args)   // 确定性脚本执行
    emit SSE: tool_result(obs)
    ctx = ctx.append(decision, obs)               // observation 回灌
    if checkpoint_reached():                       // 排空控制输入队列 / 评估 needs_input
        ctx = drain_control_inputs(ctx)
        if mode==ask and uncertain(): emit needs_input; await input
    if context_over_budget(ctx): ctx = compress(ctx)   // 上下文压缩
else:
    emit SSE: error(MAX_TURNS_EXCEEDED)            // 停止条件兜底
```

**核心差异**：LLM **不把文件内容当对话输出吐出来**，而是**调用工具**去读/改/校验；文件变更是工具的**副作用**。这是「harness」区别于「LLM 包装器」的本质。

## 动态工具门控（scope 隔离的机制级实现）

工具集**按 Run 的 scope/mode 动态构造 function schema**——LLM 物理上只能看到、只能调用被注册的工具。

| scope | 注册的写工具 | 隔离效果 |
|---|---|---|
| `current` / `page x` | `read_slide`、`patch_slide(锁定页)`、`mount_asset(锁定页)`、`validate_slide`、`finish` | LLM 无法调用改其它页 / 改公共层的工具 |
| `overview` | `read_slide(*)`、`patch_common_style`、`apply_theme`、`patch_slide(*)`、`validate_slide`、`finish` | 优先改公共层；跨页改走子代理 |
| `repo` | `search_assets`、`read_asset`、`create_asset`、`patch_asset`、`delete_asset`、`validate_asset`、`finish` | 只动仓库资产，无任何 PPT 页工具 |

| mode | 影响 |
|---|---|
| `normal` | 全部对应 scope 工具 |
| `talk` | **不注册任何写/读产物工具**，只能 `finish`（只说不做） |
| `ask` | 同 scope 工具 + 高 `needs_input` 倾向 |
| `prompt` | 不注册产物工具，只产文本（改写用户输入） |
| `recap` | 只读工具 + `finish` |

> 这把第 2 层的「hash diff 隔离」从约定变成机制：`/page 3` 下根本没有改别页的函数可调。详见 [ARCH-HARNESS-001]。

## 子代理委派（sub-agent）

- **用途（限定）**：仅用于**需要隔离多个独立上下文**的批量任务——
  - 逐页生成（每页一个子代理，独立上下文，避免 8 页互相污染）；
  - `/overview` 跨页 patch（父循环编排，每页一个子 patch 循环）。
- **编辑不走子代理**：单页编辑由主循环直接 `patch_slide`（低延迟）。
- **结构**：父 harness 持有 deck 级目标 → 派发子代理（各带单页上下文）→ 收集 observation → 汇总。
- 子代理同样遵循 ReAct + 工具门控 + 停止条件。

```
父 Harness(generate deck)
  ├── sub-agent(page 0) → write_slide(0) + validate_slide(0)
  ├── sub-agent(page 1) → ...
  └── 汇总 → finish
```

## 上下文管理

| ID | 策略 |
|---|---|
| `ARCH-HARNESS-CTX-001` | token 预算约束；超限按优先级裁剪：目标产物 > 资产细节 > 历史轮次 |
| `ARCH-HARNESS-CTX-002` | 渐进披露：先注入资产**索引**，选中后再展开具体载荷（见 [context-assembly](../50-agent/context-assembly.md)） |
| `ARCH-HARNESS-CTX-003` | 长会话**滚动压缩**：早期轮次的 thought/observation 摘要化保留结论，丢弃冗余原文 |
| `ARCH-HARNESS-CTX-004` | page 模式只注入当前页 html，绝不注入其它页全文 |

## 停止条件（防失控，生产必需）

| ID | 条件 |
|---|---|
| `ARCH-HARNESS-STOP-001` | `MAX_TURNS` 轮次上限，超出发 `error(MAX_TURNS_EXCEEDED)` |
| `ARCH-HARNESS-STOP-002` | LLM 调用 `finish` 工具显式退出 |
| `ARCH-HARNESS-STOP-003` | 连续 N 次工具执行失败 → 熔断退出 |
| `ARCH-HARNESS-STOP-004` | Run 被取消（DELETE）→ 安全终止，保留已落盘产物 |

## 关键约束汇总

| ID | 约束 |
|---|---|
| `ARCH-HARNESS-001` | 工具集 MUST 按 scope/mode 动态裁剪；越权工具 MUST NOT 出现在 function schema 中 |
| `ARCH-HARNESS-002` | LLM MUST 通过工具改文件，MUST NOT 把文件内容作为对话文本直接产出落盘 |
| `ARCH-HARNESS-003` | 每轮 thought/tool_call/observation MUST 转为 SSE 事件（可观测、可追溯） |
| `ARCH-HARNESS-004` | 循环 MUST 有 MAX_TURNS 与 finish 双重退出，杜绝无限循环 |
| `ARCH-HARNESS-005` | 子代理仅用于生成类批量任务；编辑 MUST 用主循环 |
| `ARCH-HARNESS-006` | function call 解析失败 MUST 有最小失败退出路径（不死循环），见 [llm-integration](llm-integration.md) |

## 验收标准（Given-When-Then）

- **AC-HARNESS-001**（`ARCH-HARNESS-001`）
  - GIVEN 一个 `/page 3` 的 Run
  - WHEN 构造 harness 工具集
  - THEN function schema 中不含任何可写第 0/1/2/4… 页或公共层的工具

- **AC-HARNESS-002**（`ARCH-HARNESS-002`）
  - GIVEN 一次编辑
  - WHEN 监听产物变更
  - THEN 文件变更均由工具执行产生，无「LLM 直接吐 html 被落盘」路径

- **AC-HARNESS-004**（`ARCH-HARNESS-STOP-001/002`）
  - GIVEN 一个不调用 finish 的异常循环
  - WHEN 达到 MAX_TURNS
  - THEN 发 `error(MAX_TURNS_EXCEEDED)`，循环终止

## 校验方式

```bash
go test ./internal/harness -run 'TestReActLoop|TestDynamicToolGating|TestStopConditions|TestSubAgentGenerate'
```

## 依赖

- [ARCH-RUNTIME](agent-runtime.md)、[ARCH-TOOLS](tools.md)、[ADR-0007](../90-decisions/0007-harness-react-tools.md)
