# 上下文窗口与 Compact（auto / manual）后端设计

- 状态：设计定稿（后端 + 前端均已定稿），待实现

- 日期：2026-09-04

- 关联：`2026-09-04-slash-commands-design.md`（复用其 briefing「锁 + service + 单次 model call」范式）、`2026-08-31-seed-bootstrap-design.md`（seed 组装）

## 1. 需求（用户口径原文）

1. 运行过程中，输入 token 达到阈值时**自动触发 compact**，不是 agent 主动调用某个工具去 compact。
2. 非运行时，用户点「压缩」按钮**手动触发 compact**。
3. 上述两个 compact **基于同一套机制**。
4. compact 机制**对齐 cc / codex**：利用 LLM + 上下文工程决定如何压缩。
5. 进度条展示的是**下一次会发给 LLM 的 Prompt Token 大小 / 最大阈值**的比值，并按类型（`read_ppt / run_command / system_prompt / user_prompt / chat_history / 其他`）分桶展示。

## 2. 第一性原理与核心模型

### 2.1 上下文窗口的第一性定义

> 上下文窗口 = **模型下一次调用会看到的 token 序列**。
> compact = 用模型自己把其中冗长部分总结成更短摘要，同时保住决策所需信息。

因此进度条度量的对象就是「此刻若发起一次 model call，那串已经确定的 prompt 有多大」。

### 2.2 每轮 prompt 的真实构成（已核实）

`Agent.Next` 每轮实际拼装（`backend/internal/workflow/runtime.go` L167-170）：

```
messages = [ {system: CompileForRunner(pack)}, {user: CompileForRunner(pack)} ] + state.messages
```

据此把「下一次 prompt」按**能否被 compact** 劈成两层：

| 层                   | 内容                                                                                                                         | 来源               | 能否 compact                         |
| ------------------- | -------------------------------------------------------------------------------------------------------------------------- | ---------------- | ---------------------------------- |
| **SEED（地板层）**       | system\_prompt + 工具 schema + 当前 PPT spec 快照（outline / design / theme = `read_ppt`）+ 本次 run\_command + 本轮 user\_instruction | 每轮从磁盘 spec 确定性重建 | ❌ 不能。是 ground truth，下轮反正重读，压了只丢保真度 |
| **TRANSCRIPT（累积层）** | user 发言 / steering、assistant 文本、tool\_call、tool 观察（read\_ppt 的 HTML、run\_command 的输出）、跨 run 的 chat\_history                | 对话累积、随 run 增长    | ✅ 能。唯一会无限膨胀、需要 LLM 总结的部分           |

**结论：compact 只作用于 TRANSCRIPT；SEED 由 contextengine 的 budget 机制（`assembler.go`** **/** **`types.go`）自行裁剪，compact 不碰。**

### 2.3 双维正交标签

- **分层维度**（SEED / TRANSCRIPT）：决定「能不能被 compact」。

- **分类维度**（6 个用户可见类型）：决定「进度条里显示成哪个色块」。

- 二者正交，一个片段同时带两个标签。例：运行中 `read_ppt` 工具返回的 HTML → layer=TRANSCRIPT（可压）+ type=read\_ppt（显示成 read\_ppt 桶）。

## 3. 关键架构改动（已确认）

> **把「对话 transcript」从 run 内临时对象，提升为 thread 级持久的一等对象**，取代现有有损的 `recent_turns`（`assembler.go` L453，最近 8 轮 + 每条截断 500 字）作为跨 run 续接载体。

- SEED 不变：PPT spec 仍每轮从磁盘重建。仅把「对话流」从有损桥升级为无损持久流。

- 这是让 auto（运行）与 manual（空闲）共享**同一个被压对象**、真正「同一套机制」（需求 3）的前提；也让 cc/codex 那套「一条贯穿始终的 transcript」在本项目成立。

- 收益上限声明：若某份 PPT 巨大，SEED 本身即可能占掉大半窗口，compact 压不动 SEED，此时进度条可能长期偏高。文档如实记录该边界。

## 4. 测量层设计

### 4.1 分子：下一次 prompt token（方案甲，已确认）

- **主数 = 本地估算「完整拼好的下一次 prompt」**（内容在调用前 100% 确定，我们手里有完整字节）。

- **provider 的** **`Usage.InputTokens`** **仅作校准**：每次调用后拿真值与估算比对，得到修正系数，令估算逐渐逼近真值。

- 运行中：每轮 `Agent.Next` 前，`state.messages` 已齐 → 估算精确的下一次 prompt（**含工具 schema**）→ 即进度条分子；调用返回后用 `InputTokens` 校准系数。

- 空闲中：估算「seed + 持久 transcript」→ 分子；沿用上次校准系数（无活跃 provider 真值）。

- 口径统一：任何时刻分子都是「此刻若发一次请求，那串已确定 prompt 的估算 token」。

**必须废弃的现状**：`runtime.go` L493-494 的 `state.tokens += response.Usage.TotalTokens` 是**累计花费**（只增不减），不是窗口占用。改为 `state.tokens = 估算(下一次 prompt)`（快照式、非累加）。

**估算器升级**（现 `contextengine/estimate.go` 的 `StableTokenEstimator` = runes/3 + margin，且 `runtime.go` 的 `approximateMessageTokens` 只数文本、漏算工具）：

- 覆盖：消息文本 + **工具 schema JSON** + 图片 token 近似 + role/格式包裹开销。

- 新增校准状态 `calibrationFactor float64`（thread 级或 run 级，初值 1.0）：`factor = EMA(InputTokens_real / estimate_before_call)`；估算输出 = `raw_estimate * factor`。

- provider 未回报 usage 时（`runtime.go` L495-496 fallback），估算器仍为永远可用的主数，校准是机会性的。

### 4.2 分类：6 个用户可见类型（确定性映射，已确认）

| 用户可见类型             | prompt 片段来源                                                                                                                      | 分层                |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------- | ----------------- |
| **system\_prompt** | `systemPolicy`（`compiler.go` L91）+ **工具 schema**                                                                                 | SEED              |
| **read\_ppt**      | `project_context / target_context / related_context / design_context / theme_context`（`compiler.go` L43-68）+ 运行中 `read_ppt` 工具观察 | SEED + TRANSCRIPT |
| **run\_command**   | `<run_command>` 段（本次指令）+ 运行中命令类工具观察                                                                                              | SEED + TRANSCRIPT |
| **user\_prompt**   | `<user_instruction>` + transcript 里的 user steering                                                                               | SEED + TRANSCRIPT |
| **chat\_history**  | 持久 transcript（升级后）+ `memory` + 旧 recent\_turns 替代物                                                                               | TRANSCRIPT        |
| **其他**             | policy 安全声明、runtime\_state、available\_context\_refs、格式包裹兜底                                                                       | 两层皆有              |

- 工具 schema 归入 **system\_prompt**（系统能力声明），不单列 tools 类。

- 全部为**确定性映射**（由 XML 段名 / 工具名 / 消息角色决定），无需 model call 判断，可测。

### 4.3 分母与阈值（方案甲，已确认）

三个数严格区分：

1. **模型真实上下文窗口**：物理天花板。**现状代码无此值**（`llm.Capabilities` 只有 vision/tools/reasoning，无窗口大小）。
2. **可用输入预算** = 窗口 − 输出预留。
3. **compact 触发阈值** = 窗口 × 触发比例。

决策：

- **进度条 100% = 模型真实窗口**（如 deepseek-chat 64K、kimi 相应值）。63% 即真占窗口 63%，符合用户直觉，对齐 cc/codex。

- **auto-compact 阈值 = 窗口 × 85%**（留 15% 给「压缩这一刻仍在继续的那轮生成 + 安全垫」）。

- **窗口值硬编码进** **`llm.Capabilities`**：新增字段 `ContextWindowTokens int`，各 provider 按型号填真值。

- `DefaultRuntimeBudget().ContextCompactionThreshold`（现 `domain.go` L166 裸常量 24000）改为**从窗口推导**：`threshold = ContextWindowTokens * 0.85`。

## 5. Compact 机制（对齐 cc / codex，已确认）

### 5.1 单次 model call（需求 4 + 事实澄清）

- cc / codex 的「总结」动作本身即**单次 model call**：把 transcript 一分为二 —「要总结的旧部分」+「原样保留的最近部分」，给固定结构化 summarization prompt，模型返回一段结构化摘要，再用「1 条 summary 消息 + 保留区原文」替换整段 transcript。

- 「决定压什么/留什么、token 计数、去重、替换」全是**确定性代码，无 model call**。

- 「多次 model call」只在压缩区超过单次窗口容量时理论上需要递归；本项目因**基座不支持流式、compact 期间全局持锁用户干等**，定死为**单次**。

- 复用 briefing 的 `Generate` 范式（`service/briefing.go` L158-165）：`Reasoning: ProviderDefault` + `MaxOutputTokens` 上限 + 超时。

### 5.2 输入切分

- **保留区**（不压，原样留）：最近 1-2 个完整「assistant → tool → 观察」回合 + 所有 user 原始指令。

- **压缩区**：保留区之前的全部 transcript。

- 先跑现有 `pruneSupersededRenderImages`（`runtime.go` L1995，旧渲染图去重）再压。

### 5.3 超限兜底（方案甲，已确认）

- **不递归总结**。压缩区若超过单次 model call 容量，**从最旧端确定性丢弃**直到可塞入一次调用，再单次总结。

- 简单、可预测、永远单次；因已选不归档（见 5.4），丢弃可接受。

### 5.4 输出与替换（方案乙，已确认：纯破坏性，不归档）

- 模型输出固定小节摘要：目标与意图 / 已完成改动 / 关键决策 / 未决问题 / 下一步。

- 替换逻辑：`state.messages = [1 条 summary 消息] + [保留区原文]`；SEED 不动。

- **纯破坏性**：压完原文即弃，**不归档**。最省存储；代价（压缩失真无法追溯、用户看不到压缩前全过程）已明确接受。

- 同步重写持久 transcript（见 6.1），使下一个 run 的 chat\_history 读到的即压缩后版本。

### 5.5 统一原语（需求 1+2+3）

单一原语：

```
Compactor.Compact(ctx, messages []llm.Message) ([]llm.Message, error)
```

- **auto（运行中）**：ReAct 循环内一步（复用现有 `compactIfNeeded` 钩子，`runtime.go` L454/L1981），估算 ≥ 85% 时触发 → 替换 `state.messages` + 重写持久 transcript → 继续循环。**非 agent 工具调用**，run 全程持同一把锁，无并发。

- **manual（空闲）**：独立 endpoint → 抢项目锁（与 run/commit/briefing 互斥）→ 对持久 transcript 调**同一个** `Compact` → 写回。

- 二者共用同一 `Compact` 实现，仅触发时机 / 数据入口不同。

## 6. 持久化、切面、时序（均按推荐方案）

### 6.1 持久 transcript 落点（新载体）

现有三载体均不存「可复原的 LLM transcript 原文」：run event store（存公开 SSE 事件）、thread history jsonl（有损截断、展示用）、checkpoint（存 MessageSummary 摘要）。

- **新增 thread 级 transcript store**：`threads/<id>.transcript.jsonl`，存 LLM 消息原文（role + content + tool\_call + tool 观察）+ 每条的 `{type, layer}` 标签。

- 物理形式沿用 jsonl + `FSHistoryWriter` 的 per-thread mutex 串行化范式，但**独立文件、独立 schema**，与展示用 `history.jsonl` **解耦**。

- compact（破坏性）直接**重写**该文件：`[summary] + [保留区原文]`。

- 下一个 run 的 seed 中，`chat_history` 从**读该 transcript** 取代旧的 `recent_turns`。

### 6.2 切面记录（不引入 AOP 框架）

- 定位：切面**只贴** **`{type, layer}`** **标签，不算 token、不存内容**。token 由 4.1 估算器统一算并按标签归 6 桶。

- **SEED 侧**：`compiler.go` 的 `writeSection` 每写一段登记 `{section 名 → type}`（天然 XML 段，确定性）。

- **TRANSCRIPT 侧**：每条消息追加进 transcript 时按「产生它的工具名 / 角色」打标签（确定性）。

- Go 无原生 AOP，硬造切面反而重；实现为嵌在 compile 与 transcript append 两处的轻量「标签登记」。

### 6.3 auto-compact 循环时序

复用 `runtime.go` L454 钩子位置（循环顶、`Agent.Next` 前）：

1. 循环顶 → 估算「下一次 prompt」token（含工具 schema）→ 更新 `state.tokens`（快照、非累加）。
2. 若 `≥ 窗口 × 85%` → `Compact`（去重 → 截断超限旧内容 → 单次 model call → `[summary] + [保留区]`）→ 重写 `state.messages` + 持久 transcript → 重估 token。
3. 继续 `Agent.Next`。

### 6.4 manual compact endpoint（复刻 briefing 范式）

- 独立 HTTP endpoint + service，完全复刻 briefing 的锁 + service 结构（`httpapi/briefing_handler.go`、`router.go` L77-79）。

- 路由：`POST /threads/:id/compact`（与 transcript 的 thread 归属对齐）。

- 抢锁失败 / 有活跃 run/commit/briefing → 返回复用现有 `BRIEFING_ACTIVE` 语义的互斥错误码（实现时可命名 `COMPACT_ACTIVE`，与既有风格一致）。

- 空闲态才可点；run/commit/briefing 进行中前端置灰。

### 6.5 进度条数据下发（方案甲）

- **run 中**：新增 SSE 事件类型 `context.window.updated`（遵循现有 `event.go` 命名风格），每轮估算后实时推送 `{ total, max, ratio, buckets: {read_ppt, run_command, system_prompt, user_prompt, chat_history, other} }`。需登记进 `PublicEventTypes` 并补 `ValidatePublicEvent`。

- **空闲态**：新增 REST 查询接口（如 `GET /threads/:id/context-window`）返回同结构快照，由估算器实时算「seed + 持久 transcript」。

- compact 完成后同样推送一帧 `context.window.updated`，前端进度条回落。

## 7. 前端呈现（已定稿）

关联原型：

- 常驻面板 + 6 类分桶下钻：`docs/demo/2026-09-04-context-window-panel-demo.html`

- 状态流转 + 压缩完成事件卡片：`docs/demo/2026-09-04-context-window-states-demo.html`

### 7.1 常驻面板结构（三块，跨所有状态完全一致）

1. **标题栏**：标题「上下文」+ 状态药丸 + 「压缩」按钮。
2. **进度条**：分段染色（6 桶）+ 85% 处红线阈值标记 + 右侧百分比。
3. **分桶**：6 类图例 / tab（下钻明细，每条带 `SEED / TRANSCRIPT` 分层标签）。

**硬约束**：面板结构在任何状态下都不变，只有「状态药丸文案 + 颜色」「进度条数值」「压缩按钮可用性」随状态变化。**不出现任何内联横幅、状态描述行或口径说明文字**（这些只是原型里的旁注，产品不呈现）。

### 7.2 四个运行时形态

| 形态             | 触发          | 状态药丸                 | 压缩按钮       | 进度条    |
| -------------- | ----------- | -------------------- | ---------- | ------ |
| 空闲 idle        | run 结束后     | 中性灰「空闲」（无脉冲）         | 可点         | 上次快照   |
| 运行中 running    | ReAct 循环中   | 蓝色脉冲「运行中」（**不显示轮次**） | 置灰         | 每轮实时刷新 |
| 接近阈值 warning   | 估算逼近 85%    | 橙色脉冲「接近阈值」+ 面板橙色描边   | 置灰         | 逼近红线   |
| 压缩中 compacting | Compact 执行中 | **绿色脉冲**「压缩中」        | spinner 置灰 | 保持原值   |

**无「压缩完成」形态**：压缩结束后面板直接回落到「运行中」（auto）或「空闲」（manual），进度条随之回落。

### 7.3 压缩完成 = 时间线事件富卡片

- 压缩完成**不是面板状态**，而是往对话时间线**追加一条持久富卡片**（复刻 briefing / git\_commit 卡片的落库 + `History` 合流范式），完成瞬间有一次高亮 flash。

- 卡片内容：

  - **头部**：徽标「上下文已压缩」+ 触发副标题「自动触发 · 耗时 Xs」/「手动触发 · 耗时 Xs」（**不含**模型调用次数、不含阈值文案）。

  - **指标**：两格 —「窗口占用 85% → 45%」「回收 Token −26.0k」（**不含**分桶明细列表）。

  - **摘要**：标题「压缩摘要」（**不含**「破坏性替换 / 不归档」等实现旁注）+ 结构化五小节（目标与意图 / 已完成改动 / 关键决策 / 未决问题 / 下一步）。

  - 摘要正文 **300px 定高裁切 + 渐隐遮罩 + 内联「展开全文」**（折叠判定沿用 `scrollHeight > 300`，与 briefing 卡片同口径）。

- 该卡片需作为一种**新 timeline / history 事件类型**持久化，对齐 briefing 卡片「独立存储 + History 合流展示 + 不进 LLM 上下文」的范式。

## 8. 影响面清单（实现时逐项落地，本文档不含代码改动）

- `llm/client.go`：`Capabilities` 增 `ContextWindowTokens`；deepseek / kimi 适配器填真值。

- `workflow/domain.go`：`ContextCompactionThreshold` 由窗口推导（× 0.85）。

- `workflow/runtime.go`：废弃 `state.tokens` 累加，改快照估算；升级 `approximateMessageTokens`（含工具 schema）+ 校准系数；`compactIfNeeded` 接真实 `Compactor`；每轮推送 `context.window.updated`。

- `contextengine/estimate.go`、`compiler.go`、`assembler.go`：估算器升级；seed 分段贴标签；`recent_turns` 让位给持久 transcript。

- 新增 `Compactor` 实现（单次 model call + prompt 包 + 切分/去重/截断 + 破坏性替换）。

- 新增持久 transcript store（`threads/<id>.transcript.jsonl`）+ 读写路径。

- `httpapi/router.go` + 新 handler/service：manual compact endpoint、空闲态 context-window 查询接口。

- `model/event.go` + 校验 + 测试：新增 `context.window.updated` 事件类型与 payload 校验。

- 前端：常驻上下文面板（三块结构固定，四形态）+ 时间线「上下文已压缩」事件卡片，复用 briefing 卡片组件范式；对接 `context.window.updated` SSE 与空闲态 REST 查询。

- 新增「compact 完成」timeline/history 事件类型 + 落库 + `History` 合流（对齐 briefing / git\_commit 卡片，不进 LLM 上下文）。

## 9. 决策台账（本轮 grilling 结论）

| 编号      | 决策点             | 结论                                                     |
| ------- | --------------- | ------------------------------------------------------ |
| D1      | 窗口口径            | thread 级常驻，锚点 = 下一次 run 的 assembled prompt             |
| D2      | 压缩对象            | 统一为「下一次 prompt」，compact 只作用于可累积的 TRANSCRIPT 层          |
| D3      | 架构改动            | 接受：transcript 升级为 thread 级持久对象，取代有损 recent\_turns      |
| D3a     | 分子算法            | 甲：本地估算完整 prompt 为主 + provider InputTokens 校准           |
| D3b     | 分类              | 6 类全展示，确定性映射；工具 schema 归 system\_prompt                |
| D3c     | 分母/阈值           | 甲：100% = 真实窗口；auto 阈值 85%；窗口硬编码进 Capabilities          |
| D4      | model call 次数   | 单次；超限时截断最旧、不递归                                         |
| D4-arch | 归档              | 乙：纯破坏性替换，不归档                                           |
| D5-1    | transcript 落点   | 新增独立 `threads/<id>.transcript.jsonl`，与展示 history 解耦    |
| D5-2    | manual endpoint | 独立 endpoint + service，复刻 briefing 锁 + service 范式       |
| D5-3    | 进度条下发           | 甲：run 中 SSE 实时 + 空闲 REST 查询                            |
| D6-1    | 面板结构            | 三块固定（标题栏 / 进度条 / 分桶），跨状态不变；无横幅、无说明文字                   |
| D6-2    | 运行时形态           | 四态（空闲 / 运行中 / 接近阈值 / 压缩中）；运行中不显轮次；压缩中药丸绿色              |
| D6-3    | 压缩完成呈现          | 不做面板态，回落运行中/空闲；完成事件以时间线富卡片留存，复刻 briefing 范式            |
| D6-4    | 卡片信息            | 头部「触发方式 · 耗时」；指标两格（占用变化 + 回收 token）；摘要五小节 + 300px 定高展开 |

