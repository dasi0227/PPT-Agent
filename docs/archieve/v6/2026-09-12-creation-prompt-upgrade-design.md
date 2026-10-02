# 创作 Prompt 目录重组、内容淬炼与加载链路总结

## 1. 当前结果与依据

本记录覆盖 TODO 第六项的最新实现，接续 `37c5c09` 的创作提示词与权限合同升级。生产 Prompt 从 27 份 Markdown、7 个分散注册器整理为 21 份 Markdown 和一个通用加载包；旧文件、旧模块标识与旧注册入口直接删除，不保留兼容副本。本文描述当前实现，原阶段记录可从 Git 查看。

依据为用户本轮确定的目录和职责、AGENTS.md、TODO.md、当前代码与测试，以及最新画布、Scope、图片、DOM、导出设计。Git 历史核对包括 `37c5c09`、`4cd21af`、`da3c686`、`5a1b8cc`、`431ee15` 和 `90c094d`；不把 `docs/archieve/` 作为当前协议。

参考 [Claude 官方提示指南](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices)，采用清楚目标、解释真实约束、区分政策和材料、保留有价值示例与按需用工具的原则。没有引入供应商专属能力或套用性能结论。

同时参考用户提供的 `/Users/wyw/Desktop/Notes/Agent/System Prompt.md` 和 `User Prompt.md`：只提取职责划分、按类别保护内部信息和按任务选择能力的原则。其中其他项目的角色、工具、权限、Skill、输出格式及执行指令没有成为本项目政策。

## 2. 最终目录与逐文件职责

以下相对 `backend/`，`prompts/` 内仅保留 Markdown：

```text
prompts/
├── core/
│   ├── agent.md                 // 创作 Agent 的身份、能力与 HTML PPT 业务目标
│   ├── output.md                // 凭证、内部政策和实现信息的披露边界，少量语言与真实表达要求
│   ├── reference.md             // 图片、页面、组件、Skill、DOM、简报与摘要的用途和可信度
│   ├── structure.md             // PPT 数据归属、运行时字段及动态生成的可写 Schema
│   ├── quality.md               // 准确性、叙事、信息层级、排版、配色与可读性
│   └── html.md                  // 固定画布、运行时样式与装饰、素材、脚本及静态展示环境
├── mode/
│   ├── chat.md                  // 只读分析与完整 finish 交付
│   ├── grill.md                 // 只读澄清、提问与同一循环内继续，最终 finish
│   ├── plan.md                  // 只读规划、方案内容、create_plan/update_plan 审批交接
│   └── execute.md               // 写权限、原 Run 扩权、计划与工作账本、执行和终止协议
├── runtime/
│   ├── completion.md            // 完成条件、当前证据、渲染观察与 Harness 终止状态
│   └── recovery.md              // 工具失败、失效依赖、完成拒绝和继续执行
├── playbook/
│   ├── spec.md                  // 单页或多页语义设计稿创建与修改
│   ├── slide.md                 // 单页 HTML 创建、局部 patch、重写及验证选择
│   └── deck.md                  // 从零创建、已有结构修改、跨页依赖、分批和续做
├── subagent/
│   └── reviewer/
│       └── agent.md             // 审查职责、实际输入边界、标准与 checks[] 输出
└── command/
    ├── polish.md                // 保留意图的输入润色，仅返回指令纯文本
    ├── handoff.md               // 已完成、未完成、失败、证据与下一步的交接简报
    ├── kickoff.md               // 目标、约束和验收标准构成的启动简报
    ├── compact.md               // 连续任务的固定五节上下文压缩
    └── commit.md                // staged diff 对应的 git_commit title/items 生成
```

### 合并与淬炼

- 原 `core_runtime_policy` 拆成干净的身份定位、引用信任边界、各 Mode 的执行事实和 Runtime 恢复协议，避免 agent.md 承担所有流程。
- `output.md` 改为按信息类别限制披露。普通业务术语、用户自己的内容和页面 HTML、必要的错误与扩权说明可以正常表达；没有添加敏感词黑名单或固定问答选项规范。
- 原只读协作 Playbook 合入 Chat/Grill，规划方法合入 Plan；共同的工具和控制边界在互斥 Mode 中适度重复。Grill 问题的具体参数约束交回真实 Tool Schema。
- 原空目录生成和整套修改合为 `deck.md`，按实际结构选择 init/insert，支持新旧页面混合与部分完成后的续做。单页和 Spec 策略分别保留。
- HTML 环境独立于审美：图表准确性和标签可读性移入 quality，渲染证据与工具观察移入 completion。HTML 保留画布、样式、装饰、素材与静态可用性约定。
- Reviewer 三份始终联用的文件合并成一份；删除 `SemanticReviewInput.Rubric`，避免把相同政策再次放入动态输入。保留现有解析器与 checks[] 协议。
- 五个 Command 保留真实专用输出合同，不获得主 Agent 的写权限。commit 中的精确输出限制属于其专用服务合同，不传播到 core/output。

这些策略不要求先完成所有 Spec，再写所有 HTML；不要求简单修改创建计划、调用 Reviewer 或重复读取不变信息。必要的依赖、权限和证据要求仍然保留。

## 3. 统一加载与组装

```text
backend/embedded_prompts.go          唯一生产 Prompt go:embed 入口
    ↓ 提供 embed.FS
backend/internal/prompt/catalog.go   ID → 路径 + 版本
backend/internal/prompt/loader.go    读取、非空校验、标准化正文、SHA-256
    ├─ workflow/prompt_modules.go    Mode / Scope / Playbook 条件组装
    ├─ workflow/semantic_reviewer.go 独立审查请求与持久化 manifest
    ├─ service/polish.go             输入润色
    ├─ service/briefing.go           kickoff / handoff
    ├─ contextcompact/compactor.go   压缩与 token 预算估算
    └─ service/git_commit.go         提交信息生成
```

加载包不依赖 RunState，不读取运行时磁盘文件。根包仅提供嵌入文件，避免 `go:embed ../` 和业务循环依赖。部署继续是编译嵌入方式。

模块标识按目录表达，例如 `core.agent`、`mode.execute`、`playbook.deck`、`command.polish`。统一发布版本为 `2026-09-12.v5`；每份模块都有实际正文 SHA-256，版本表示协调发布，哈希表示具体内容。未知 ID、缺失文件或空正文明确失败。`WithBody` 用于动态填充后的标准化和哈希计算；Schema 读取失败不再静默漏掉合同。

### 主创作 Agent 的条件组合

所有模式均加载 core.agent、core.output、core.reference 和恰好一个 mode。

| Mode | Playbook | completion | recovery | structure / quality | html |
| --- | --- | --- | --- | --- | --- |
| Chat | 无 | 有 | 无 | 无 | 无 |
| Grill | 无 | 有 | 无 | 无 | 无 |
| Plan | 无，方法在 mode.plan | 无 | 无 | 有 | 对象允许 HTML 时 |
| Execute | 按下面规则选一个 | 有 | 有 | 有 | 对象允许 HTML 时 |

Execute：global → deck；spec → spec；html/presentation 单页 → slide，多页 → deck。空目录和已有目录使用同一 deck 策略，具体行动由当前资源决定。没有 default 文件或 default Playbook；未指定模式仍按既有 RunCommand 默认语义解析为 Chat，这不增加第五种模式。

有效模式始终优先使用 AgentRequest.Mode，其次 Context.Command.Mode。相同有效值用于模块选择、manifest 和动态任务上下文。

| 对象权限 | structure 注入的可写模型 |
| --- | --- |
| spec | slide-spec |
| html | 空 JSON 对象，仍保留资源归属说明 |
| presentation | slide-spec |
| global | manifest、outline、design、slide-spec |

模型来自 `schemas.AgentContract`，由正式 Schema 剔除运行时字段，不维护另一套手写 JSON。页数和页面引用不改变对象权限；Plan 获得模型描述用于规划，不获得写权限。原 Run 扩权后下一轮重新组装并计算填充后的哈希。

### 动态上下文与独立服务

主 Agent 的用户指令、项目内容、当前 Scope/计划/工作账本/证据、Skill、组件和页面引用继续位于 user/context/transcript，图片与 DOM 选择保持已有消息链路，不进入静态 system。

润色和简报原先将项目上下文附在 system 中。本次同步改为 system 仅包含专用政策，user 包含标明来源的数据与当前草稿或简报请求；修订简报仍保留最近两版全文和全部反馈。编译函数只构造动态材料，不再接受政策参数。

Reviewer 仍仅接收结构化文字、证据摘要和检索片段，不接收截图像素；不能据此声称看过视觉效果。Plan 审查不要求已有 HTML 或渲染产物；输出仍为 1–5 个 checks，summary 至少 20 个字符。系统与持久化 manifest 均记录合并模块的路径、版本和实际哈希。

compact 的政策和 token 估算同步使用统一加载入口。commit 的单工具调用合同以及两类简报的持久化、版本反馈行为保持不变。

## 4. 保留的协议与真实限制

- Scope 是写权限边界，WorkLedger 是实际任务进度；引用页面和旧简报不授予额外权限。未进入任务承诺的授权页面不需要重做。
- Execute 才可写；request_privilege 在原 Run 中等待真实结果；已存在执行计划只允许更新步骤状态，批准计划仍是执行合同。
- Runtime 分配正式 ID、revision、schema version 等字段；Outline 管理标题、角色、两级结构和顺序；Spec-only 任务不需要 HTML。
- 1920×1080 固定画布、Runtime 注入样式和公共装饰保持不变；用户选择的主题只读，自定义布局和内联 SVG/CSS/JS 不需要复制模板。
- 附件 HTML 路径仍取已校验的 original_path；缩略图 MIME 不代表原图格式。DOM 选择是可能过期的结构快照，不是截图或额外权限。
- 当前完成检查仍要求全局 Design 变更同步整套 HTML 并取得新鲜渲染证据；本次没有放宽 Gate。正确 HTML 仅证据失效时应重新 render，而非无意义改写。
- 通用素材创建、统一动画生命周期尚未实现；导出是应用独立流程，不是主 Agent 已披露工具。写入、render、视觉判断和导出是不同事实。
- 按 Scope 的结构性路由不进行另一次意图分类：global 范围内的局部请求仍由 deck 策略强调最小相关工作，不能把较广权限当成整套改写要求。
- Prompt 约束不是凭证过滤器或新的安全执法层；没有新增程序级内容过滤。实际授权继续由 Harness 和工具执行层实施。

## 5. 回归验证与效果评估边界

验证覆盖：

- 全量嵌入树与 catalog 双向匹配，所有生产文件为 Markdown，路径不重复，版本非空，正文哈希正确。
- 未知 ID、缺失文件、空正文与空渲染正文的失败路径。
- 4 Mode × 4 对象 × 单/多页，共 32 组最终模型请求模块、恰好一个 Mode、正确的零或一个 Playbook、按权限 Schema 和动态填充哈希。
- 显式/缺省有效模式一致、扩权后合同与哈希刷新、主 Agent 动态上下文隔离。
- Reviewer 单模块、输入隔离和持久化 manifest；polish、kickoff、handoff 的专用政策、版本和动态材料隔离；compact 和 commit 的统一入口。
- 既有权限、计划、WorkLedger、图片、DOM、渲染和完成门槛测试继续执行。

执行结果：`cd backend && go test ./...` 全部通过；`git diff --check` 通过。未修改前端实现，无需增加前端构建验证。

工程测试只证明加载、组装和协议回归，没有运行真实模型 A/B，不宣称任务成功率、工具调用效率或视觉质量改善幅度。后续仍需使用固定模型、初始项目快照和用户任务评估空目录创作、多页局部修改、旧 DOM、图片压缩恢复、扩权拒绝、Spec-only、部分完成续做和缺失 render 等场景，并记录完成率、重试、无关改写、耗时及人工视觉评分。
