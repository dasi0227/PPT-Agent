---
id: AGENT-RUNTIME-UPGRADE-ROADMAP
title: HTML PPT Agent Runtime 阶段性专业化升级路线
status: approved-for-planning
owner: shared
date: 2026-08-01
depends_on:
  - docs/superpowers/specs/2026-08-01-artifact-target-blueprint-refactor-design.md
  - docs/v2/30-agent-pipeline-v2.md
---

# HTML PPT Agent Runtime 阶段性专业化升级路线

## 1. 当前成熟度判断

当前项目已经具备专业 Agent Runtime 的基础骨架：

- Project → Thread → Run 的隔离模型。
- Run 状态、取消、SSE、事件续传和 HITL。
- ReAct Loop、工具 schema、工具门控、熔断和上下文上限。
- 文件 Sandbox。
- 设计总监、计划、逐页子 Agent、lint、有限修复和交付。
- SQLite + 文件系统混合持久化与制品版本。

它目前更准确的定位是：

> 可工作的本地垂直 Agent Runtime，而不是可恢复、可度量、可评测、可治理的生产级 Runtime。

后续升级目标不是把它做成通用 Agent 平台，而是让 HTML PPT 这一条垂直链路具备生产级工程属性。

## 2. 主要差距

### 2.1 Run 不可持久恢复

当前活跃 Run、输入队列、needs_input 等待状态、项目锁均在进程内存。服务重启后：

- running/waiting Run 无法恢复。
- 已完成事件可以重放，但执行游标无法继续。
- 项目锁丢失。
- 用户应答队列丢失。
- 数据库可能残留永久 running/waiting 状态。

### 2.2 缺少完整 Trace Envelope

当前 runs 表未完整记录：

- 规范化 WorkSpec。
- Agent/Runner 版本。
- Prompt 版本和摘要。
- LLM provider/model。
- token 输入输出。
- 费用。
- LLM/tool/阶段耗时。
- 重试与降级记录。
- Blueprint/Design/Presentation 输入 revision。

因此结果难以复现，也无法比较优化前后的质量和成本。

### 2.3 Context Assembly 不完整

Thread 历史主要用于前端回放，不是稳定的模型上下文来源。当前 Harness 的超限压缩只是用固定占位文本替换旧消息，不是真实语义摘要。

专业 Context Pack 应明确包含：

- 用户本轮目标。
- target。
- 当前 deck 摘要。
- 当前 section/subsection。
- 目标 slide blueprint。
- design spec 摘要。
- 当前 HTML/DOM 摘要。
- 可用资产。
- Thread 历史摘要。
- 用户运行中追加指令。
- revision 与来源引用。

### 2.4 编排和工具治理仍然偏手工

- Runner 路由硬编码。
- 一个 LLM 回合只处理一个工具调用。
- 工具参数主要由各工具自行解析，缺少统一 schema validation。
- 工具没有幂等键。
- 工具风险等级和确认策略散落在具体工具中。
- 文件与数据库复合写的补偿策略不统一。

### 2.5 观察事件不等于专业可观测性

当前 `thought` 把模型响应文本当作推理过程展示和持久化，不应把原始模型思维文本当作产品级 Trace。

需要区分：

- 用户可见的简洁状态摘要。
- 工程 Span。
- Tool input/output 摘要。
- 内部敏感上下文。
- 不应保存或展示的原始 chain-of-thought。

### 2.6 缺少真实质量评测

现有测试覆盖大量确定性代码，但缺少：

- 真实 LLM 场景集。
- Blueprint 质量评测。
- 浏览器渲染成功率。
- 文本/元素溢出检测。
- 截图视觉回归。
- 跨页一致性评分。
- 叙事结构评分。
- 用户指令遵循率。
- 修复循环成功率。
- 模型/Prompt A/B。

### 2.7 模型接入只有抽象，没有路由能力

虽然存在 LLM interface，但生产路径只有 DeepSeek：

- 无按阶段选模型。
- 无 provider fallback。
- 无结构化输出能力声明。
- 无 token/cost budget。
- 主 ReAct Loop 不使用流式 tool call。
- 无模型级 circuit breaker。

## 3. 升级总原则

1. 领域模型优先于 Runtime 泛化：先完成 Blueprint/Presentation target 重构。
2. 可观测优先于优化：没有基线指标前不做复杂模型路由。
3. 可评测优先于“更聪明”：每个 Prompt/模型升级必须有场景集证明。
4. 可恢复优先于分布式：单机也可以拥有 durable execution。
5. Policy 集中化：安全和确认不能依赖 Prompt 自觉。
6. 保持固定业务流水线：不引入可配置通用 DAG。

## 4. 阶段路线

## Stage R0：领域语义重构

目标：

- 落地 `blueprint/presentation × slide/deck`。
- 建立 DeckBlueprint、SlideBlueprint、DesignSpec 和 revision。
- 合并 generate/edit 为 Presentation Runner。

这是所有 Runtime 升级的前置条件。没有稳定 WorkSpec，后续 Trace、Context、Eval 都缺少一致维度。

验收：

- 见 Artifact Target 设计文档 DoD。

## Stage R1：Run Trace 与可复现执行

目标：

- 每次 Run 都形成完整、可查询的执行档案。

新增 Run Trace Envelope：

```json
{
  "run_id": "run-id",
  "work_spec": {},
  "agent": {
    "name": "presentation-slide",
    "version": "v1"
  },
  "model": {
    "provider": "deepseek",
    "model": "deepseek-chat"
  },
  "prompts": [
    {
      "name": "slide.presentation",
      "version": "v3",
      "content_hash": "sha256..."
    }
  ],
  "inputs": {
    "deck_revision": 3,
    "design_revision": 2,
    "blueprint_revisions": {}
  },
  "usage": {
    "input_tokens": 0,
    "output_tokens": 0,
    "estimated_cost": 0
  }
}
```

实现：

- `run_traces` 或 runs 扩展表。
- `run_spans`：stage、llm、tool、validation。
- 统一结构化日志字段 `run_id/thread_id/project_id/span_id`。
- Prompt 版本与 hash 入 trace。
- LLM Client 返回 usage、latency、request id。
- Tool 执行记录 duration、result status、artifact refs。
- 将 `thought` 迁移为 `status.summary`；不持久化原始 CoT。

验收：

- 任一产物可反查 Run、模型、Prompt、输入 revision、Tool 和耗时。
- 失败 Run 可定位失败阶段。
- 可统计每种 target 的平均耗时、失败率、LLM 调用次数和成本。

## Stage R2：Context Pack 与 Thread Memory

目标：

- 让每个 Runner 获得稳定、可测试、可预算的上下文。

新增 `ContextAssembler`：

```go
type ContextPack struct {
    WorkSpec          WorkSpec
    DeckSummary       string
    SectionContext    string
    SlideBlueprint    *SlideBlueprint
    DesignSummary     string
    PresentationState string
    AssetCandidates   []AssetSummary
    ThreadSummary     string
    RecentTurns       []Message
    RevisionRefs      RevisionRefs
}
```

实现：

- 每个字段有独立 loader 和 token budget。
- Context Pack 作为确定性对象，可快照、测试、hash。
- Thread 历史生成真实语义摘要，保存 summary revision。
- 不再使用固定占位句作为压缩结果。
- 大型 HTML 先提取 DOM/视觉摘要，按需渐进披露。
- Tool observation 分层：摘要进入常驻上下文，完整结果按引用读取。

验收：

- 同一个 Context Pack 和模型配置可复现同类输入。
- Thread 历史能够影响后续请求，但不会无限增长。
- Target slide 不会收到无关页面的完整 HTML。

## Stage R3：Durable Run 与恢复

目标：

- 服务重启后 Run 状态可确定恢复或安全终止。

固定 checkpoint：

- Run 创建完成。
- Design Director 完成。
- Plan 完成。
- 每页物化完成。
- 每轮全局校验完成。
- needs_input 进入等待。
- 交付前。

持久化：

- `run_cursor`
- `stage_state_json`
- `attempt`
- `lease_owner`
- `lease_expires_at`
- `checkpoint_payload`

恢复策略：

- pending → 重新调度。
- running 且 lease 过期 → 从最近 checkpoint 重试。
- waiting → 恢复 needs_input。
- tool side effect 必须有幂等 key。
- 已完成页面通过 revision/hash 跳过。

单机实现即可使用 SQLite job table + worker loop，不要求引入 Redis/Kafka。

验收：

- 在生成第 N 页后杀死进程，重启后从 N+1 或安全重试 N 继续。
- 不产生重复 version、重复 event、重复 artifact。
- waiting Run 重启后仍可接受原 reply_to。

## Stage R4：Tool Runtime 与 Policy

目标：

- 工具从“带 schema 的 Go 方法”升级为可治理能力。

新增：

- 中央 JSON Schema 参数校验。
- `ToolCallID + IdempotencyKey`。
- Capability。
- Risk：
  - `read`
  - `write-reversible`
  - `write-destructive`
  - `external`
- Policy Decision：
  - allow
  - deny
  - require_confirmation
- Timeout、retry、max payload、artifact limits。
- 统一 ToolResult：

```json
{
  "status": "succeeded",
  "summary": "...",
  "artifacts": [],
  "warnings": [],
  "retryable": false
}
```

多工具调用：

- 默认保持串行，确保文件状态可预测。
- 只读且声明可并行的工具允许并行。
- 多页子 Agent 并发要受 project write policy 约束。

验收：

- destructive 工具不能仅靠 Prompt 绕过确认。
- 同一幂等键重试不会重复写入。
- Tool schema 错误在执行前被统一拒绝。

## Stage R5：HTML PPT Eval 与浏览器验证

目标：

- 将“看起来不错”变成有基线、有指标的工程能力。

评测集分层：

### Blueprint Eval

- 目录层级合理性。
- 每页唯一核心句。
- 页面之间信息不重复。
- 叙事连贯度。
- role/archetype 多样性。
- 指令遵循。

### Static HTML Eval

- HTML/CSS 规范。
- 资源引用。
- token 使用。
- accessibility。
- 禁止危险能力。

### Browser Eval

- 页面加载成功。
- console error。
- 文本和元素溢出。
- viewport 16:9。
- 字体加载。
- 图片失效。
- 动画触发。
- 缩略图/主预览一致。

### Visual Eval

- 截图基线。
- 关键区域差异。
- 跨页色彩/字体一致。
- 信息密度。
- 对比度和可读性。
- 可选 vision model reviewer，但必须与确定性检查分开记录。

建立最小场景集：

- 中文商业汇报。
- 英文产品发布。
- 数据密集型报告。
- 品牌主题约束。
- 无图片降级。
- 单页复杂修改。
- 全局视觉调整。
- Blueprint 修改后的精准重建。

验收：

- 每次 Prompt/模型升级可跑同一 eval set。
- 输出成功率、平均分、成本、延迟、修复轮次。
- CI 运行确定性 Eval；真实 LLM/视觉评测可按夜间或人工触发。

## Stage R6：Model Router 与预算

目标：

- 按任务阶段选择模型，并具备可观测降级。

模型能力声明：

- tool calling
- structured output
- vision
- context window
- streaming
- cost class
- latency class

路由示例：

- Blueprint 规划：推理/结构化能力优先。
- Design Director：设计判断能力优先。
- 单页 HTML：代码生成能力优先。
- lint 修复：低成本快速模型优先。
- 视觉 review：vision model。

预算：

- Run token budget。
- stage budget。
- max retries。
- max fix rounds。
- time budget。
- cost ceiling。

降级：

- 主模型不可用 → 同能力备用模型。
- 备用模型仍失败 → deterministic fallback 或明确失败。
- 降级事件写入 Trace。

验收：

- 路由决策可解释并进入 Trace。
- 达到预算上限时安全停止并交付已有产物/warnings。
- Provider 故障不会造成无限重试。

## Stage R7：作品级能力

这些能力提升产品差异，但不应早于 R1–R5：

- DOM 选区与局部多模态编辑。
- 引用/事实来源管理。
- 品牌规范导入。
- PDF/PPTX 导出。
- 演讲模式与 speaker notes。
- 版本 Diff 与可视化回滚。
- 人工审阅 Gate。
- 自动生成封面候选与视觉探索分支。

## 5. 推荐里程碑

| 里程碑 | 内容 | 项目价值 |
|---|---|---|
| M-A | R0 Artifact/Blueprint 重构 | 建立稳定领域语义 |
| M-B | R1 Trace + R2 Context Pack | 让 Agent 可解释、可优化 |
| M-C | R5 Eval 最小闭环 | 让质量升级有证据 |
| M-D | R3 Durable Run | 让长任务可靠 |
| M-E | R4 Tool Policy | 让副作用可治理 |
| M-F | R6 Model Router | 在有指标后优化成本和质量 |
| M-G | R7 作品级功能 | 形成产品差异 |

推荐 R5 早于 R3/R4 全量建设：如果没有 Eval，Runtime 升级只能证明“系统更复杂”，不能证明“Agent 更好”。

## 6. 简历价值表达

完成 R0–R5 后，可以真实描述为：

> 设计并实现面向 HTML Presentation 的垂直 Agent Runtime，具备 Artifact-targeted WorkSpec、版本化 Blueprint、SSE/HITL、工具能力与安全策略、阶段 checkpoint、可恢复执行、模型与 Prompt Trace、浏览器渲染验证和场景化 Eval。

真正体现高级性的不是使用了多少框架，而是：

- 领域模型清楚。
- 运行可恢复。
- 副作用可治理。
- 上下文可追溯。
- 结果可评测。
- 优化有数据。

## 7. 非目标

- 不建设多租户 SaaS 平台。
- 不追求任意业务适用的通用 Agent SDK。
- 不引入用户不可理解的可视化 DAG 编辑器。
- 不为了并行而破坏同一 PPT 的写入一致性。
- 不持久化模型原始 chain-of-thought。
- 不在没有 Eval 基线时盲目增加多模型和多 Agent。

## 8. 路线验收总览

- R0：目标模型统一，Blueprint/Presentation revision 成立。
- R1：任一结果可追溯到模型、Prompt、输入和工具。
- R2：上下文有明确结构、预算、来源和快照。
- R3：重启后 Run 可恢复且无重复副作用。
- R4：工具有 capability、risk、policy 和 idempotency。
- R5：有真实 PPT 场景集与浏览器/视觉质量指标。
- R6：模型选择、预算和降级可解释。
- R7：用户能够完成更高级的演示文稿创作与交付。
