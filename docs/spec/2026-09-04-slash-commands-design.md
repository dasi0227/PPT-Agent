# 输入框 `/` 命令设计

## 一、业务需求

### 1.1 背景

当前输入框已实现四种触发符：`@`（汇总检索）、`#`（页面）、`¥`/`$`（组件）、`%`/`％`（提示词）。触发符迁移完成后，`/` 被预留且当前处于完全静默状态，是一块干净的空地。

本需求要在 `/` 上落地一套**命令系统**。与既有触发符的本质区别在于：

- 既有触发符（`@#¥%`）用于**引用可复用产物**（页面、组件、提示词等），选中后往输入框插入一段彩色文本片段，随下一条消息发送。

- `/` 命令是**纯前后端交互，用于直接触发一个动作或修改 runtime**，不涉及 SQL / 文件层面的产物持久化引用。选中命令即执行动作，不向输入框留下待发送的正文。

### 1.2 命令清单

用户敲下 `/` 后，弹出一个命令列表（视觉形态见需求附图一：单列列表，图标 + 命令名 + 灰色描述 + 右侧状态/回车标记），复用现有单列候选菜单的视觉外壳与键盘导航范式。命令分两类。

**A. 模式切换（等价于在前端点击对应模式按钮）**

- `/plan`：开启计划模式

- `/ask`：开启审问模式

- `/talk`：开启聊天模式

**B. 动作命令**

- `/commit`：等价于点击执行 Git 提交（复用现有 commit 流程）

- `/polish`：等价于点击执行 polish（复用现有 polish 流程）

- `/model`：**二级命令**。确认后展示当前支持的所有模型（需求附图二），再次确认即切换输入框使用的模型。

- `/target`：**二级命令**。确认后展示可选的生成目标，再次确认即切换目标。

- `/kickoff`：**新能力**。类似一次与 Agent 的交互，效果相当于 PM + Owner 的身份，根据上下文与交互历史，产出一份交给**另一个 Agent 去开发**的启动 prompt。产出内容只包含这份启动 prompt 本身。

- `/handoff`：**新能力**。设计背景是当前上下文即将占满，需要把项目进度、历史执行、当前执行状态、核心需求等讲清楚，产出一份**交接 prompt**，确保接手者能快速理解业务背景与开发进度。

> **kickoff 与 handoff 的定位差异**：kickoff 面向「开发启动」（传递意图与目标，让新 Agent 自行探索落地）；handoff 面向「交接准备」（压缩并交代当前进度与状态，让接手者快速进入状态）。

### 1.3 迭代修订需求

用户对初次生成的 kickoff / handoff 产出若不满意，需要能**基于反馈建议重新生成**。由于反馈可能有多轮，产出因此演化为一条**版本链**：每一版 = 基于上一版 + 本轮反馈的结果。

***

## 二、需求决策（已与用户确认）

以下为经过逐条确认（含第一性原理论证）后锁定的决策，作为开发的唯一依据。

### 2.1 范围划分

| 命令                     | 映射                          | 是否需要新后端 |
| ---------------------- | --------------------------- | ------- |
| `/plan` `/ask` `/talk` | composer `setIntent(mode)`  | 否       |
| `/model` `/target`     | composer store setter（二级菜单） | 否       |
| `/commit`              | 现有 git-commit 异步 SSE 流程     | 否（复用）   |
| `/polish`              | 现有 polish 同步 endpoint       | 否（复用）   |
| `/kickoff` `/handoff`  | 新增轻量同步 service              | **是**   |

**结论**：唯一新增的后端能力是 `/kickoff` 和 `/handoff`；其余 6 个命令是**前端命令分发层**，复用既有 state / endpoint。`/plan` 等模式命令仅翻转模式开关，不自动提交任务。

### 2.2 commit / polish 不补「专门工具」

「终态工具」这一概念只在 run 引擎的 ReAct 循环内才有意义（用于替换 `finish` 以驱动前端富卡片）。commit / polish **都不在 run 引擎内运行**：

- polish 是一次确定性 LLM 改写，无循环、无工具，产出回填输入框。

- commit 是独立 executor，已有自己的 `git_commit` 富时间线卡片。

「每个能力有独立后端面 + 独立前端渲染」的产品一致性目前已满足，各自用了最合适的机制。**不给 commit / polish 补工具**，避免无收益返工。

### 2.3 kickoff / handoff 不进 run 引擎（第一性原理结论）

**结论：kickoff / handoff 的第一性形态是「一次同步 Model Call」，不进 run 引擎、无 ReAct 循环、无终态工具。**

论证：run 引擎（ReAct 循环）只为解决两类问题而存在——

1. **信息缺口需靠「行动」闭合**：任务开始时不知道答案，须一边调工具观察环境、一边决定下一步。
2. **副作用需被验证**：产生了改动，需要 completion gate / evidence / finish 契约担保改动落地且自洽。

kickoff / handoff 两者皆不满足：

- 输入（当前上下文 + 交互历史 + 进度状态）全部来自**已知信息**，是对已知信息的**蒸馏 / 转写**，而非对未知环境的**探索**；组装阶段即可一次性喂入。

- 输出是一段 prompt 文本，**零项目改动**、零副作用。

- `/handoff` 触发前提是「上下文快满」，而 ReAct 每多一轮都往上下文里塞更多工具 schema 与结果——用「消耗更多上下文」的机制服务「因上下文不足才发起」的任务，方向相反、自相矛盾。

设计原则是**让计算结构匹配问题结构**：对「大上下文蒸馏 / 转写」任务，最优解是「精心的上下文组装 + 单次（可开 reasoning 的）强模型调用」，而非工具循环。单次调用在质量、延迟、确定性、输出可控性、失败面五个维度全面优于 ReAct 循环。

**该复用的是零件，不是引擎**：ContextEngine 组装、持久化范式、富卡片渲染，均非 run 引擎独占。

### 2.4 输出载体：同步返回，不做流式

经核查，当前系统**没有任何文本 token 流式能力**：

- LLM `Provider` 接口只有一次性 `Generate(ctx, req)`，三个 adapter（openai / deepseek / kimi）全为非流式。

- run 引擎本身也不做文本 token 流，只推结构化 public event，`finish.message` 亦整段返回。

为单个命令去铺 token 流基座属于严重不成比例的基础设施改造，且会波及所有 LLM 调用方。而 kickoff / handoff 是单步、无中间状态节点的生成，也不需要 commit 那种「状态事件流」。

**结论：同步一次性返回（照 polish 机制），无任何 SSE。** 因输出比 polish 长，**超时放宽至 45s**。代价：生成期间用户需等待且无逐字反馈，由前端 loading 态富卡片占位——相对铺 token 流基建是划算取舍。

### 2.5 endpoint 与 service 拆分

- **两个独立 endpoint**：`POST /projects/:id/kickoff`、`POST /projects/:id/handoff`。

- **两个独立 service**，各自实现自己的上下文组装策略（未来分化预留位）。

- 拆分理由：两者**上下文组装本就该不同**（kickoff 偏意图 / 目标 / 约束蒸馏；handoff 偏进度 / 历史 / 当前状态交接），核心逻辑将分道，独立建模比塞 `kind` 分支更诚实。

- SSE 无关的公共设施（错误映射、鉴权、限流等纯样板）仍可抽共享小工具复用，业务主体分开。

### 2.6 本次上下文组装先相同

- **本次先不做上下文组装差异**，两者沿用现成的组装策略（照 `AssemblePolish` 范式），差异化留待后续，不作为本次核心目标。

- **prompt 模板从一开始分开**（kickoff 偏意图蒸馏、handoff 偏进度交接），这是零成本就能区分的部分。

- 说明：「上下文快满」是 handoff 的**用户侧触发动机**，不是 service 的技术约束——service 是独立会话、从线程历史重新组装，并不共享那个「快满」的对话上下文。

### 2.7 持久化与存储模型（第一性原理结论）

**结论：kickoff / handoff 存独立表，展示时按时间戳 merge 进对话流。**

数据的物理归属由「写入者 / 写入时机 / 生命周期 / schema 形状」决定，而非由「展示在哪」决定——「展示合流」与「物理同表」是两个正交维度。判据：

1. **是否对话轮次的一部分**：普通消息由 user→agent 往返驱动写入；kickoff / handoff 是旁路生成，不属于任何一次对话往返。
2. **schema 是否同构**：普通消息是 turn + text；kickoff / handoff 是「一段 prompt 全文 + 类型 + 时间戳 + thread 关联」，不同构，塞进消息表会污染 schema。
3. **生命周期是否独立**：kickoff / handoff 是一次性生成的旁路交付物，生命周期与对话脱钩。
4. **读取路径已解耦**：`History()` 已用「独立存储 + 按时间戳 merge」统一了 commit / steering 的展示，读取层已具备合流任意旁路数据源的能力，展示诉求不构成塞进消息表的理由。

现有系统已存在稳定范式：**特殊消息类型 = 独立存储 +** **`History()`** **里按时间戳 merge**（commit、steering 均如此）。commit 物理存独立表，展示时才合流进时间线——「展示在 thread 里」与「物理独立存储」并不矛盾。

**存储粒度**：kickoff + handoff **共用一张表带** **`kind`** **字段**（产出物形状同构：文本 + 类型 + 时间戳 + thread 关联），与 commit 各自独立。endpoint / service 拆是因为**组装策略**会分化（写入前的事），但**存储物形状**一致，故存储层不拆。

### 2.8 上下文卫生硬约束（已核查证据）

**硬约束：kickoff / handoff / commit 的记录永不进入后续对话喂给 LLM 的历史上下文。**

核查证据：喂给 LLM 的上下文组装链路与前端展示的 `History()` 链路完全独立——

- **LLM 上下文组装**（`contextengine/assembler.go` 的 `loadRecentTurns`）直接读原始 jsonl `threads/{threadID}.jsonl`，且有严格类型白名单，只保留 `user_turn` / `markdown` / `final_result`；从不调用 `History()` / `ListThreadGitCommits` / `ListThreadSteering`。

- `git_commit` 事件根本**不在落盘白名单**（`run/bus.go` 的 `isWhitelistedForHistory`），压根不写入 jsonl，只活在独立表。

- 前端展示链路 `service/thread.go` 的 `History()`（唯一调用者是 HTTP handler）才做 jsonl + steering 表 + git\_commit 表的合流，不碰 LLM 组装。

**据此，kickoff / handoff 落地必须满足三条，硬约束即天然成立、无需额外过滤逻辑**：

1. 存独立表，**不写入** `threads/{threadID}.jsonl`；
2. **不加入** `isWhitelistedForHistory` 落盘白名单；
3. 只在 `History()` 里按时间戳 merge（照 `gitCommitHistoryEntry`），供前端展示。

### 2.9 产出载体：持久化富卡片

- 产出**落持久化富卡片**（照 commit 范式：独立存储 + `History()` 合流 + 前端 `TimelineItem` 类型），展示 prompt 全文 + **复制按钮**，输入框保持不变。

- 理由：产出是「交给他人的交付物」，本应留痕可回看、可复制；回填输入框会与「持久化留痕」矛盾，也会错误暗示「要在本会话发送」。

- **全文折叠**：卡片内 prompt 全文默认**定高折叠**，底部渐隐遮罩上浮一个**居中悬浮按钮**（`展开全文` / `收起`）。折叠与否的判定**以渲染后的实际高度为准**（阈值约 300px），而非字符数——字符数受中英文 / 换行影响不准。内容未超高时不显示遮罩与按钮。

- **生成中 loading 态**：因同步返回、无 token 流（§2.4），生成期间（最长 45s）以 loading 态富卡片占位。占位内容：计时 + 阶段文案 + 骨架屏 + 取消按钮。进度条为前端估算的「假进度」（同步方案拿不到真实进度），仅用于缓解等待焦虑；若认为假进度有误导，可退化为纯计时 + 骨架屏。超时 / 失败按 §2.12 处理（不落库、可重试）。

### 2.10 带反馈的多轮修订：版本链

一旦承认反馈是**累积多轮**的，版本链就是更诚实的数据模型（版本是节点、反馈是边，二者是同一条链的两种视角）。

**数据模型（版本链）**：

- 一次 kickoff / handoff = 一个 **group**（`briefing_id`），下挂**有序多个 version**。

- 每个 version 记：`version_no`、产出全文、**触发它的反馈文本**（v1 为初始生成、反馈为空）、时间戳。

- `History()` merge 时，整个 group 按**最新更新时间**插入**一张卡片**（不是每版一张，避免刷屏）。

**回喂策略（滑动窗口）**：

- 回喂给 LLM = **最近 2 版全文（vN-1、vN-2；不足两版时有几版带几版）+ 全部反馈列表**。

- 后端**完整存储全部版本链**（供未来回看 / 回退），滑动窗口（=2）**只作用于回喂给 LLM 的部分**。

- 理由：反馈短，全量累积成列表满足「反馈是列表」的诉求；产出全文长，设 2 的硬上界防止上下文随修订轮次线性膨胀（尤其保护 handoff 的上下文卫生）。这是「计算结构匹配问题结构」的一致应用。

**前端卡片**（原型确定）：

- **版本切换**：卡片右上角 `‹ n / N ›`，箭头可点切换版本；`N/N` 为最新版；首 / 末版对应箭头置灰。后端存全量版本链，切换即读对应版本。

- **重新生成入口**：置于「复制全文」按钮**左侧**，**仅在浏览最新版时显示**；切到历史版本时隐藏（历史版本纯只读回看）。点击后在卡片**下方**展开反馈输入框 + 取消 / 提交按钮；提交即带反馈追加新 version，版本号 +1 并自动跳到最新版。

- **不展示反馈记录列表**：卡片不单独列出历次反馈（原型确认移除），反馈仅作为生成新版的输入。

- **重新生成基于最新版、版本链线性追加**：因入口仅最新版可见，重试恒基于末版向后追加，不产生分叉。

### 2.11 前置条件与并发

- **run 运行中禁止**触发 kickoff / handoff（与 commit 一致，`runActive` 置灰）。

- **空项目禁止**（需项目已有内容）。

- 复用 composer 当前选中的 model（`modelProfileName`）。

- 无 active thread 时 `ensureActiveThread`。

- **kickoff / handoff / polish / commit 全部互斥**：任一进行中，其余置灰。

- 并发防护**前后端双保险**：前端 busy 态置灰（主防线）+ 后端轻量锁校验「该 thread 无进行中的同类操作」（防脏请求）。

### 2.12 失败处理

- 生成失败（超时 / LLM 报错）：**不落库、不留卡片**，前端 toast 报错并解除 busy 态，用户可重试（与 polish 失败行为一致）。

### 2.13 前端命令菜单交互

- `/` 走**独立的命令菜单分支**，复用现有单列候选菜单的**视觉外壳与键盘导航范式**（单列、↑↓ 环绕、Enter 选中、Esc 关闭、`data-candidate-index` 滚动），但**数据源是静态命令表、回调是「执行动作」而非「插入片段」**。

- **选中命令即执行动作 + 清除** **`/xxx`** **触发文本**（命令是动作非内容，执行完从输入框消失，不污染下次要发送的正文；保留用户在 `/xxx` 之前已输入的正文）。

- **单行展示**：每个命令一行——**主文案为不带斜杠的命令名**（如 `talk` `handoff`，等宽字体高亮），紧跟灰色描述；不再展示「计划模式」这类中文菜单名（中文名降级为文档 / aria label 用途，UI 一级列表不渲染）。触发匹配仍按 `/xxx` 识别，仅展示去掉斜杠。

- **前缀过滤**：`/pl` 只显示匹配项（`/plan` `/polish`）；无匹配显示「无匹配命令」。

- **空 query**（刚敲下 `/`）：显示全部命令。

- **分组**：按三组加分组标题——**模式**（plan / ask / talk）、**操作**（kickoff / handoff / commit / polish）、**设置**（model / target）。

- **二级命令标记**：`/model` `/target` 右侧加 `›`。

- **禁用态**：置灰保留（不隐藏），可在描述位给出不可用原因。仅 kickoff / handoff / commit / polish 参与互斥禁用；模式命令与 model / target 永不禁用。

### 2.14 二级菜单（`/model` `/target`）

- **进入方式**：一级菜单对二级命令按 Enter / 点击 → 一级列表**替换**为二级列表（非并排另开浮层）。

- **数据源**：`/model` 复用已加载的 model profiles，当前选中项（`composer.modelProfileName`）打 ✓；`/target` 复用 scope × object 组合。

- **`/target`** **摊平为一维 4 项**：单页设计稿 / 单页幻灯片 / 整份设计稿 / 整份幻灯片，当前组合打 ✓。

- **键盘模型**：二级内 ↑↓ 环绕；Enter 确认选中并**关闭整个菜单**；Esc **返回一级**（非直接关闭）。

- **层级导航**：二级顶部显示**返回箭头 + 标题**（如「← 选择模型」/「← 选择目标」），点箭头回一级。

- **确认行为**：选中即等价点击对应 selector（`setModelProfileName` / 设置 target）+ 清除触发文本 + 关菜单。

### 2.15 命令文案

> 「菜单名」列仅用于文档说明 / aria label；UI 一级列表**只展示不带斜杠的命令名 + 描述**（见 §2.13）。

| 命令         | 分组 | 菜单名（文档/aria） | 描述                     |
| ---------- | -- | ------------ | ---------------------- |
| `/plan`    | 模式 | 计划模式         | 切换到计划模式                |
| `/ask`     | 模式 | 审问模式         | 切换到审问模式                |
| `/talk`    | 模式 | 聊天模式         | 切换到聊天模式                |
| `/kickoff` | 操作 | 启动简报         | 生成交给新 Agent 的启动 prompt |
| `/handoff` | 操作 | 交接简报         | 生成上下文交接 prompt         |
| `/commit`  | 操作 | 提交           | 执行一次 Git 提交            |
| `/polish`  | 操作 | 润色           | 润色当前输入内容               |
| `/model`   | 设置 | 切换模型         | 选择对话使用的模型              |
| `/target`  | 设置 | 切换目标         | 选择生成目标范围与对象            |

***

## 三、现状实现要点（开发参考）

- **`/`** **当前静默**：触发符迁移后汇总面板改由 `@` 唤起，`/` 无任何绑定。见 `frontend/src/features/agent/promptMatching.ts`。

- **模式**：`RunMode`（`talk` / `ask` / `plan` / `execute`）定义于 `backend/internal/model/run_command.go`；前端靠 `frontend/src/stores/composerStore.ts` 的 `setIntent` 切换。

- **Polish**：同步 POST `/projects/:id/polish`（`backend/internal/service/polish.go`，12s 超时、单次 LLM、无工具），产出回填输入框；上下文由 `contextengine/polish.go` 的 `AssemblePolish` 组装（`memory.Load` + `loadRecentTurns`，仅 `turn == "user"`）；handler 见 `backend/internal/httpapi/polish_handler.go`。

- **Commit**：独立 executor（`backend/internal/gitcommit/executor.go`）+ 异步 SSE，前端 `frontend/src/stores/gitCommitStore.ts` 订阅并渲染 `git_commit` 富时间线卡片。

- **Model / Target**：绑定 composer store 的下拉菜单，`frontend/src/features/agent/ModelSelector.tsx`、`TargetSelector.tsx`；model profiles 由 `CommandComposer.tsx` 加载。

- **History 合流范式**：`backend/internal/service/thread.go` 的 `History()` 读 jsonl + `ListThreadSteering` + `ListThreadGitCommits`，用 `gitCommitHistoryEntry` 按时间戳插入；唯一调用者为 `backend/internal/httpapi/thread_handler.go`。

- **上下文组装（不含旁路）**：`backend/internal/contextengine/assembler.go` 的 `loadRecentTurns` 读原始 jsonl，白名单只保留 `user_turn` / `markdown` / `final_result`；`git_commit` 事件不在 `run/bus.go` 的 `isWhitelistedForHistory`，不落 jsonl。

- **前端时间线**：`frontend/src/features/agent/eventReducer.ts` 有 `git_commit` 的 `TimelineItem` 类型，实时事件与 history 回放共用同一渲染。

- **触发符匹配**：`frontend/src/features/agent/promptMatching.ts`、编辑器 `PromptComposerEditor.tsx`（候选菜单 `role="listbox"`、键盘导航、`data-candidate-index`）。

***

## 四、详细方案

### 4.1 后端：kickoff / handoff service

#### 4.1.1 存储层

新增一张独立表，承载 kickoff / handoff 的版本链（共表，`kind` 区分）。字段草案：

- `briefing_id`：group 主键（一次 kickoff / handoff 的整条版本链）。

- `thread_id`：关联线程。

- `project_id`：关联项目。

- `kind`：`kickoff` | `handoff`。

- `version_no`：版本序号（从 1 递增）。

- `content`：该版本产出的 prompt 全文。

- `feedback`：触发该版本的反馈文本（v1 为空 / 初始生成）。

- `created_at`：版本时间戳。

约束：`(briefing_id, version_no)` 唯一。查询按 `thread_id` 取该线程全部 group，每个 group 取最新版本供卡片展示、取最近 2 版供回喂。

对应新增 store 方法（照 `ListThreadGitCommits` 风格）：

- `AppendBriefingVersion(ctx, ...)`：追加一个版本。

- `ListThreadBriefings(ctx, threadID)`：按 group 聚合返回，供 `History()` 合流。

- `GetBriefingVersions(ctx, briefingID, limit)`：取最近 N 版（回喂用）。

> 遵循 AGENTS.md 开发期约定：直接切到新结构，不为历史数据设计迁移，不保留兼容层。

#### 4.1.2 上下文组装

本次两者**先复用相同组装策略**，照 `AssemblePolish`（`memory.Load` + `loadRecentTurns`）产出基础上下文。重试时额外拼入**最近 2 版全文 + 全部反馈列表**（滑动窗口只作用于回喂）。prompt 模板 kickoff / handoff **分开**：

- `backend/prompts/`（照 `git_commit` 目录风格）下新增 kickoff / handoff 两套模板，分别体现「意图 / 目标 / 约束蒸馏」与「进度 / 历史 / 状态交接」的侧重。

#### 4.1.3 service 与 endpoint

- 两个独立 service（`KickoffService` / `HandoffService`），公共设施（错误映射、超时、锁校验、组装复用）抽共享小工具。

- 单次 `Provider.Generate` 调用（可开 reasoning），**超时 45s**。

- 成功：`AppendBriefingVersion` 落库，返回该 group 最新版本（含 `briefing_id`、`version_no`、`content`、反馈列表）。

- 失败：不落库，返回错误（前端 toast + 解除 busy）。

- 后端**轻量锁**：校验该 thread 无进行中的同类操作，防脏请求。

新增两个 handler（照 `polish_handler.go`）：

- `POST /projects/:id/kickoff`

- `POST /projects/:id/handoff`

请求体（两者同构）：

```json
{
  "thread_id": "...",
  "model_profile_name": "...",
  "briefing_id": "（重试时携带，首次生成为空）",
  "feedback": "（重试时携带本轮反馈，首次为空）"
}
```

前置校验（后端侧）：项目非空、无进行中的互斥操作、model 可用。

#### 4.1.4 History 合流

- 在 `service/thread.go` 的 `History()` 中新增 `ListThreadBriefings` 分支，用 `briefingHistoryEntry`（照 `gitCommitHistoryEntry`）把每个 group 合成一条 entry，按最新版本时间戳插入。

- **严禁**将 briefing 写入 jsonl，**严禁**加入 `isWhitelistedForHistory`（保证不进 LLM 上下文，见 §2.8）。

### 4.2 前端：命令分发层

#### 4.2.1 静态命令表与菜单分支

- 在 `promptMatching.ts` 中恢复 `/` 触发识别，但分发到**独立的命令菜单逻辑**，不复用 `matchPrompts` 等数据驱动匹配。

- 定义静态命令常量表（含 id、命令名、描述、分组【模式 / 操作 / 设置】、是否二级、禁用条件计算）。

- 在编辑器候选菜单中新增命令菜单渲染分支：单列、分组标题、命令名去斜杠展示、`›` 标记、禁用置灰 + 原因、前缀过滤、无匹配态。

#### 4.2.2 命令执行

- 每个命令绑定一个「执行动作」回调，选中即执行并清除 `/xxx` 触发文本：

  - `/plan` `/ask` `/talk` → `composer.setIntent(...)`。

  - `/commit` → 触发现有 commit 流程（等价点击 header 按钮）。

  - `/polish` → 触发现有 polish 流程。

  - `/kickoff` `/handoff` → 调用新 endpoint，进入 busy 态，成功后卡片入时间线。

  - `/model` `/target` → 进入二级菜单（不立即执行）。

#### 4.2.3 二级菜单

- 一级列表被二级列表**替换**（同一浮层内切换），顶部返回箭头 + 标题。

- `/model`：渲染 model profiles，当前项 ✓，Enter 确认 → `setModelProfileName` + 关菜单。

- `/target`：渲染摊平的 4 项组合，当前项 ✓，Enter 确认 → 设置 target + 关菜单。

- 键盘：↑↓ 环绕、Enter 确认关闭、Esc 返回一级、箭头点击返回一级。

#### 4.2.4 富卡片与版本链

- 在 `eventReducer.ts` 新增 `briefing`（或 `kickoff` / `handoff`）的 `TimelineItem` 类型，实时插入与 history 回放共用渲染。

- 卡片内容（原型确定，见 §2.9 / §2.10 与下方 demo）：

  - kind 标识（badge）；

  - prompt 全文，**定高折叠 + 底部居中悬浮按钮**（展开 / 收起），折叠判定以**渲染实际高度**为准（阈值 \~300px）；

  - 右上角 `‹ n / N ›` **版本切换**（后端存全量版本链，箭头切换只读回看）；

  - **复制全文**按钮；其左侧为**重新生成**入口，**仅浏览最新版时显示**；

  - 点重新生成 → 卡片下方展开反馈输入框 + 取消 / 提交；

  - **不展示反馈记录列表**。

- 提交反馈：带 `briefing_id` + `feedback` 重新打同一 endpoint，成功后卡片刷新为新版本并跳到最新版（版本链线性追加，不分叉）。

- **生成 / 重试 loading 态**：以 loading 富卡片占位（计时 + 阶段文案 + 骨架屏 + 取消，进度条为估算假进度，见 §2.9）；超时 / 失败按 §2.12 处理。

- busy 态：生成 / 重试期间，命令菜单中 kickoff / handoff / polish / commit 置灰，发送禁用。

> **原型参考**（`docs/demo/`，仅原型不含最终工程实现）：
>
> - 命令菜单（一级 + 二级、分组、禁用、键盘导航）：`2026-09-04-slash-command-menu-demo.html`
>
> - 富卡片（版本切换、折叠悬浮按钮、重新生成、复制）：`2026-09-04-slash-briefing-card-demo.html`
>
> - 生成中占位（计时 / 进度 / 骨架 / 取消 / 超时 / 失败）：`2026-09-04-slash-briefing-loading-demo.html`

#### 4.2.5 前置条件（前端）

- 计算 `disabled`：`runActive` || 空项目 || 存在互斥进行中操作。

- 无 active thread 时 `ensureActiveThread`（照 commit）。

***

## 五、影响面与不改动项

**改动**：

- 后端：新增 briefing 独立表 + store 方法；新增 kickoff / handoff service、handler、路由；新增两套 prompt 模板；`thread.go` `History()` 增加 briefing 合流分支。

- 前端：`promptMatching.ts` 恢复 `/` 分发；编辑器新增命令菜单分支与二级菜单；`eventReducer.ts` 新增 briefing 时间线类型与卡片；composer / 命令执行回调接线。

**不改动**：

- run 引擎（`runtime.go` 的模式分派 / 工具披露 / finish 判定 / checkpoint）——kickoff / handoff 不进 run 引擎。

- LLM `Provider` 接口与三个 adapter——不引入流式。

- commit / polish 既有后端逻辑——仅前端复用其触发。

- `@` `#` `¥` `%` 既有触发符逻辑。

- `isWhitelistedForHistory` 与 jsonl 落盘范围——保证上下文卫生硬约束。

***

## 六、验证

- **前端**：`promptMatching.test.ts` 增加 `/` 命令识别、前缀过滤、二级菜单键盘导航（↑↓ 环绕、Enter 确认、Esc 返回一级）用例；TypeScript 类型检查通过。

  - 注意（历史教训）：编辑器级集成测试在 JSDOM 大用例下有 OOM 风险，逻辑测试优先放在 `promptMatching.test.ts` 等纯逻辑层。

- **后端**：kickoff / handoff service 的单测（成功落库、失败不落库、重试回喂窗口 = 最近 2 版 + 全部反馈、锁互斥、前置校验）；`History()` 合流顺序（briefing 按时间戳插入、不进 jsonl）。

- **手动**：`/` 各命令行为、二级菜单、卡片复制、多轮反馈重试、run 中 / 空项目 / 互斥的置灰。

