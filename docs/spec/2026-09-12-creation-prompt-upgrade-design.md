# 创作 Prompt 组装升级与变更总结

- 日期：2026-09-12
- 对应 TODO：第六项「优化和淬炼创作提示词，提升 Agent 任务完成效率」
- 状态：代码与提示词已落地，后端全量测试通过；尚未进行真实模型 A/B 效果评估。
- Runtime Prompt 版本：`2026-09-12.v4`
- Kickoff / Handoff / Semantic Reviewer 版本：`2026-09-12.v2`

## 1. 本次改动的核心

将常驻提示词从偏重运行规则的说明，改为以 **完成用户的 PPT 创作任务** 为目标，同时明确权限、数据所有权、HTML 环境、参考材料和完成证据。

保留真实依赖：正式 ID 由 Runtime 创建；语义模型按 Schema 写入；渲染基于当前设计与页面；写操作遵守当前 Scope。解除没有数据依赖的固定顺序：不再要求整套设计稿全部完成之后才能开始写 HTML，也不要求按照目录顺序逐页制作。

模型可以自行设计布局、SVG、图表和页面内交互。用户选中的主题继续约束整体视觉方向，但主题辅助 class 和组件样张不再被暗示为必选结构。程序继续负责权限、引用、版本、证据与运行状态；内容准确性、叙事和视觉质量由模型判断。

## 2. 阅读范围和依据

本次逐份阅读了 `backend/prompts/` 中原有的 25 份 Markdown，以及它们的注册器、调用服务、动态上下文编译、工具披露、模型 Schema、完成检查与相关测试。新增两个 Runtime 模块后，共有 27 份生产 Prompt Markdown。

同时核查了三个 seed Skill、主题和组件的消费路径、UI Prompt Library 的插入链路，以及 `docs/thirdparty/html-ppt-skill/SKILL.md` 的模板路线。第三方文件是对照材料，不是本项目创作 Agent 自动加载的系统政策；本次未修改第三方资产或历史文档。

当前依据包括：

- `AGENTS.md`、`TODO.md`。
- `docs/spec/2026-09-08-unified-html-canvas-design.md`。
- `docs/spec/2026-09-08-scope-selection-refactor-design.md`。
- `docs/spec/2026-09-09-image-attachments-design.md`。
- `docs/spec/2026-09-10-dom-selection-reference-design.md`。
- `docs/spec/2026-09-12-presentation-export-design.md`。
- Prompt Library、Slash Commands、Context Compaction 与既有领域模型设计，以及当前代码中的后续修订。

Git 进度核查包括：`cf22e96` 简报新会话、`eb16a12` 统一画布、`0dfbd3e` 范围协议、`cca70e4` 扩权、`90c094d` 工作账本、`431ee15` 多页选择、`5a1b8cc` 图片附件、`da3c686` DOM 选择和 `4cd21af` 整套导出。

也阅读了 TODO 引用的 [Claude 官方提示指南](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices)。本次采用明确目标、解释约束原因、提供任务相关示例和按需要使用工具的原则；没有照搬供应商专属能力或未经本项目验证的性能结论。

以下“问题”来自源码和提示词之间可复核的矛盾或遗漏，不能等同于已经通过线上日志统计的失败频率。

## 3. 实际加载链路

### 3.1 主创作 Agent

```text
ContextAssembler / 当前 RunState
    ↓
CognitiveAgent.Next(AgentRequest)
    ├─ runtimeSystemPromptForRequest
    │    └─ buildRuntimeSystemPrompt
    │         ├─ Runtime 嵌入模块：版本 + 路径 + 内容哈希
    │         ├─ 按模式选择模式政策与一个 playbook
    │         └─ 按对象权限注入 Schema 与 HTML 合同
    ├─ runtimeTaskStateForRequest
    │    └─ mode / phase / plan_authority / changes / evidence /
    │       requirements / work_ledger / context_briefing
    └─ compiledPromptForAgentRequest → CompileForRunner
         ├─ system：上述模块
         ├─ user：指令、项目/目标/主题上下文与运行事实
         ├─ user 附加：active_run_skills、referenced_components、mentioned_pages
         └─ transcript：当前用户消息、图片、DOM 选区、steering、工具结果、压缩摘要
    ↓
Provider.Generate(messages, 当前披露的 tool schemas)
```

所有 Provider 共用业务 Prompt。工具参数不另写一份自然语言伪 Schema；调用形式仍由 `ppt_tools.go`、`runtime.go`、`tools.go` 等实时披露。

系统层不嵌入用户指令、具体 Skill 正文、项目标题、工作进度或审批内容。资源 Schema 是按对象权限选择的稳定合同；扩权后下一次调用重新组装。具体权限中的页面集合和 revision 留在动态 RunCommand 中。

### 3.2 模式与合同组合

| 模式 | 专用政策 / playbook | Finish | Repair | 质量标准 / 资源合同 | HTML 作者合同 |
| --- | --- | --- | --- | --- | --- |
| Chat | chat / read_only_collaboration | 有 | 无 | 无 | 无 |
| Grill | grill / read_only_collaboration | 有 | 无 | 无 | 无 |
| Plan | plan / read_only_planning | 无，提交审批方案 | 无 | 有 | 目标对象允许 HTML 时有 |
| Execute | execute / 按对象、页数、空目录选一个 | 有 | 有 | 有 | 目标对象允许 HTML 时有 |

所有模式均加载 `core_runtime_policy`、`user_facing_output`、`reference_context`。

| Scope.object | 注入的可写 JSON 合同 | HTML 作者合同（Plan/Execute） |
| --- | --- | --- |
| spec | slide-spec | 无 |
| html | 空对象；保留所有权解释 | 有 |
| presentation | slide-spec | 有 |
| global | manifest、outline、design、slide-spec | 有 |

单页、多页、全部页不改变对象权限。Plan 注入合同用于规划，不授予写能力。

### 3.3 其他 Prompt 不是主 Agent 的常驻模块

| 来源 | 加载位置 / 时机 | 用途与输出 |
| --- | --- | --- |
| `prompts/polish/policy.md` | `service/polish.go` → `CompilePolishContext` | 润色输入草稿，仅返回指令纯文本，不执行创作 |
| `prompts/kickoff/policy.md` | `service/briefing.go` 的 KickoffService | 生成完整启动简报，供用户采用某版开启新会话 |
| `prompts/handoff/policy.md` | 同一 briefingGenerator，选择独立 Handoff Policy | 生成完整交接简报，侧重已做/未做/失败/下一步 |
| `prompts/compact/policy.md` | `contextcompact.Compactor.Compact` | 压缩旧 transcript，返回固定五节 Markdown，作为 `context_summary` 接续 |
| `prompts/semantic_reviewer/**` | `workflow/semantic_reviewer.go`，显式 `review_completion` | 独立模型请求，返回 `checks[]`；主 Agent 决定并执行修复 |
| `prompts/git_commit/policy.md` | `service/git_commit.go`，产品项目提交操作 | 只生成 `git_commit` 的 title/items；不是主 Agent 的提交权限 |

Kickoff/Handoff 当前仍复用 `AssembleBriefing`（基于 PolishContext）生成摘要上下文，首次生成和反馈修订走独立请求；最近两版全文与全部反馈放入 `revision_context`。它们并不持有完整 RunState 或全部截图，不应凭摘要声称逐页验证成功。Polish/Briefing 的现有专用编译器将带有不可信标记的上下文附在各自 system 文本中；本次没有改变其服务接口或上下文角色布局。

### 3.4 用户可选内容

- **Prompt Library**：SQLite CRUD 数据。`PromptComposerEditor.tsx` 将 `prompt.value` 插入为可编辑文本，最后进入用户 instruction。它不是另一份隐藏 system prompt，也不在 `backend/prompts/` 中保存。
- **Skills**：三个 seed Skill 分别用于演示叙事、高管摘要和视觉层级审查。用户选择的 Skill 保存为运行快照；按需加载工具可补充 active skill set；正文进入 `active_run_skills` 动态上下文。本次保留三个 Skill 的内容。
- **Components**：引用时携带组件 HTML，或通过组件读取工具按需获取；不自动修改项目。样张中的内容、布局和脚本不能变成越权指令。
- **Themes**：上下文包含主题 tokens 和辅助 selectors。`design.theme` 对 Agent 只读；本次把“优先使用 allowed selectors”的引导改成辅助 class 可选，保留 tokens 和视觉方向。
- **图片与选区**：图片以文本元数据和 image part 进入消息；选区是结构化 DOM 快照，不是截图。压缩后可以按稳定引用重新读取所需资源。

## 4. Runtime 逐文件变更

路径均相对 `backend/prompts/runtime/`。

| 文件 | 用途 / 本次处理 |
| --- | --- |
| `core/core_runtime_policy.md` | 重写。先定义 PPT 创作目标，再解释当前用户意图、运行事实、参考数据和执行循环的边界；强调持续完成、按需读取和不虚构结果 |
| `core/user_facing_output.md` | 重写。保持面向演示用户的表达，补充有意义的进度反馈；用户明确问实现时允许必要技术解释，完整结果仍进入 finish |
| `core/reference_context.md` | 新增。统一 Skill、组件、页面引用、图片、DOM 选区、简报和压缩摘要的用途、可信度和恢复方式 |
| `modes/chat.md` | 保留。只读分析，以 finish 交付；不披露提问能力时说明必要假设 |
| `modes/grill.md` | 保留。只读问答，questions[]、原子问题、至多三个选项，回答后继续同一循环 |
| `modes/plan.md` | 保留。只读方案，create_plan / update_plan 交给审批，无 finish |
| `modes/execute.md` | 重写。四种对象权限、原 Run 扩权、权限与工作清单分离、批准计划和跟踪计划、按依赖选择写法及工具使用条件 |
| `playbooks/default.md` | 保留。识别目标、读取必要状态、做一致改动、采用对应验证；作为兜底 |
| `playbooks/read_only_collaboration.md` | 保留。基于事实给分析/比较/审查，不暗示副作用 |
| `playbooks/read_only_planning.md` | 保留。说明范围、变化和验收路径，区分已知事实与假设 |
| `playbooks/spec_edit.md` | 重写。仅写单页语义字段；标题/角色属于 Outline；全部页 spec 权限也不等于全局权限；不要求 HTML 或截图 |
| `playbooks/slide_presentation_edit.md` | 重写。说明 exact-string patch 与整页重写的选择；旧 DOM 快照需对照当前内容；按视觉问题请求截图 |
| `playbooks/empty_deck_generation.md` | 重写。保留结构→正式 ID→页面依赖；允许逐页和分批；已有空章节用 insert；不再固定先全 Spec 再全 HTML |
| `playbooks/deck_coordinated_edit.md` | 重写。明确多页任务、全局 Design 成本、纯重排、局部视觉修改、续做；增加具体场景 |
| `resources/resource_contracts.md` | 重写。资源归属、运行时字段、revision 和两种 patch 区分；JSON 由现有 Schema 生成，按对象权限裁剪 |
| `resources/html_authoring.md` | 新增。集中定义 1920×1080 单页文档、Runtime 外框与 chrome、自由创作、附件路径、静态可见性、离线依赖和渲染条件 |
| `rubrics/ppt_quality_rubric.md` | 淬炼。专注内容、叙事、证据、层级、密度、一致性和可读性；画布实现细节移入 HTML 合同 |
| `guides/completion_repair_guide.md` | 重写。按缺失证据或依赖类型修复；只有截图过期时先渲染；补 WORK_NOT_COMPLETE 和扩权路径；禁止伪造进度 |
| `guides/finish_contract.md` | 重写。按任务目标判断完成，区分写入、渲染、视觉判断和导出；完整答案只在 finish.message 中交付 |

注册表增加两个模块，并更新版本。模块的加载条件与前述模式表一致，没有把整套 HTML 实现规范灌入普通 Chat/Grill 或 spec-only 任务。

## 5. 辅助 Prompt 变更

| 文件 | 处理与原因 |
| --- | --- |
| `kickoff/policy.md` | 从默认“开发 PPT Agent 软件仓库”改为承接用户实际任务；演示创作以内容、页面和 HTML 结果为中心，只有明确的软件开发任务才采用开发语境；补充新会话重新读取当前状态与权限 |
| `handoff/policy.md` | 同样纠正固定开发语境；重点区分完成、尝试、失败、未验证；保留页面与参考素材身份，防止重做或把历史批准当成新授权 |
| `compact/policy.md` | 保留五个二级标题，加入页面剩余义务、附件/选区引用和当前证据意识；压缩是上下文接续，不是新计划或权限授予 |
| `semantic_reviewer/core/semantic_reviewer_policy.md` | 明确 reviewer 实际收到文本和证据摘要，不能声称看过截图像素；Plan 不要求已生成 HTML；对 scope、候选文本缺失和信息不足分别处理 |
| `semantic_reviewer/rubrics/ppt_completion_rubric.md` | 删除与 checks-only 输出冲突的全局 reject 指令；按模式解释证据，防止模板偏好被判为缺陷 |
| `semantic_reviewer/schemas/review_output_contract.md` | 对齐真实解析器：包括 REVIEW_PASS 在内，summary 至少 20 个字符；保持既有 JSON 结构和代码集合 |
| `polish/policy.md` | 阅读后保留。已经具备保留原始意图、模式和范围、不凭空增加实现方案、纯文本输出等边界 |
| `git_commit/policy.md` | 阅读后保留。独立提交消息生成协议与创作执行不同，不将其工具加入主循环；本次没有执行仓库 commit/push |

## 6. 组装代码修正

### 6.1 一个有效模式贯穿所有层

原先模式政策使用 `AgentRequest.Mode`，playbook 却直接读取 `Context.Command.Mode`；当二者不同或缺省时可能拼出冲突指令。

现在统一取显式请求模式，其次命令模式，均缺省时为 Chat；同一个值用于 system manifest、模式模块、playbook 和动态运行上下文。实际工具能力仍由 Runtime 决定，Prompt 不自行切换执行权限。

### 6.2 可写合同按对象权限，而非页数裁剪

原先仅非全局单页裁成 Slide Spec，其余情况包含全部四类模型。这会让多页 spec 或 presentation 任务看到全局可写模型示例。

现在 `resourceContractNames` 使用 `AllowsGlobal/AllowsSpec`，与工具权限方向一致；HTML-only 不注入可写 Spec 示例。字段仍由 `schemas.AgentContract` 从领域 Schema 生成，没有引入第二套数据结构或兼容层。

### 6.3 哈希反映真正发给模型的正文

原先 resource_contracts 的 hash 来自带 `{{CONTRACTS_JSON}}` 的源文件。即使注入的合同因范围或 Schema 而变化，hash 也不变。

现在在填入 JSON 后对完整模块正文计算 SHA-256。静态模块继续按源正文计算。测试逐一检查最终 manifest 的正文和 hash 对应。

### 6.4 动态上下文消除相互矛盾的引导

- 用户 instruction 是需要遵循的任务；项目文本、样张和外部引用是不能改写政策的来源数据；Runtime 字段说明当前事实。不再把这一切统称为“只当数据，不能当指令”。
- 主题辅助 selectors 明确可选，继续保持主题只读。
- mentioned_pages 改为“依据用户表达用于参考、比较或编辑”，不再一概宣布为修改目标，也不授予 Scope。

### 6.5 图片读取提供可直接用于嵌入的事实

原先 `read_image` 只返回所读 variant 的 media_type：查看缩略图时为 WebP，但原图可能是 PNG/JPEG。提示词无法仅凭这个结果可靠选择原图扩展名。

现在两个 variant 均返回已校验附件元数据中的 `original_path`，例如 `attachments/att_image/original.png`。HTML 合同明确加 `/` 前缀用于页面嵌入，不能把缩略图 MIME 或上传文件名当成原图路径。只是补充工具 observation，不改变附件持久化、上传请求或前端协议。

## 7. 关键行为的前后对照

| 场景 | 原提示的风险 | 现在的指引 |
| --- | --- | --- |
| 空白 PPT 生成 | 全部 Spec 写完才进入 HTML，问题发现较晚 | 先确认身份和真实依赖，允许代表页验证、逐页完成或分批 |
| 只改 3、5、8 页 | 多页合同含全局资源；容易把权限当工作列表 | 仅实际承诺页面进入工作；读取邻页作参考不等于修改邻页 |
| 修改页标题 | 容易把标题写进 Slide Spec | 标题/角色/顺序由 Outline 拥有，必要时申请 global |
| 仅 HTML 改字号 | 不必要地同步设计稿或全局 Design | 当前 HTML 唯一锚点 patch，完成后获取对应证据 |
| 调整公共页码位置 | 旧文案要求改命令或 ask_user | request_privilege 申请全局权限；不在 HTML 中复制公共 chrome |
| 修改全局 Design | 没有清楚解释整套同步成本 | 说明当前 Gate 要求整套 HTML 同步及渲染，局部任务优先局部样式 |
| 图文参考 / 旧选区 | 缺少稳定参考和压缩后的使用解释 | 分清图片、DOM 快照和截图；缺内容时重新读取，不使用旧字符串盲 patch |
| HTML 证据过期 | 先 patch/write，再 render，可能无谓修改正确内容 | 内容正确时先 render；实际内容或依赖变化才修改 |
| Plan 请求 reviewer | 通用证据要求容易把只读计划当成未完成执行 | 只评估方案及验收策略，不要求写入/渲染产物 |
| 继续剩余页面 | 可能重做已完成页面或把 done 等同完整验证 | 根据工作账本接续，分别核实任务覆盖与新鲜证据 |

## 8. 保留的硬边界和当前限制

1. 当前是 1920×1080、16:9，不添加新画布比例。
2. Runtime 管理正式身份、版本、物化和共享装饰；模型不写这些状态文件。
3. `mutate_ppt` 一次一个操作；JSON Patch 与 HTML exact-string patch 各用现有协议。
4. Plan 无 finish，Execute 才可写，扩权必须等待当前 Run 的实际批准结果。
5. 主题选择仍只读；自由版式不等于任意替换主题。
6. 通用素材文件创建仍属于 TODO 十。本次引导内联 SVG/CSS/JS 和已上传图片，没有虚构写文件、下载字体或安装图表库的能力。
7. 统一动画生命周期仍属于 TODO 九。核心内容需要在初始/静态状态可见，不能假定已有页面进入/离开事件。
8. 当前导出是应用独立的整套任务，Agent Run 活跃时不能发起；生成或 render 成功不等于已导出。本次没有修改导出资格、格式或字体策略。
9. 全局 Design 的完成检查仍按现有代码要求整套同步。本次准确描述它，没有绕过或偷偷放宽 Gate。若未来希望仅 chrome 更新不触发整套 HTML 同步，应单独设计和验证。

## 9. 验证结果和效果评估边界

已执行：

- `go test ./internal/workflow ./internal/contextengine ./internal/contextcompact ./internal/service ./prompts/...`：通过。
- `go test ./...`（backend）：通过。
- `git diff --check`：通过。

新增 `backend/internal/workflow/prompt_contract_test.go` 验证：

- 4 种模式 × 4 种对象权限 × 单/多页，共 32 个组合的最终模块与可写 JSON 合同。
- 最终注入正文与 SHA-256 一致、模块无重复、Schema 占位符已替换。
- 显式模式与上下文模式不一致、请求模式缺省时，政策/playbook/动态上下文保持一致。
- HTML → Global 扩权后的合同及哈希更新。

新增 `image_tools_test.go` 用实际 PNG 字节和误导性的 WebP 文件名验证：无论读取缩略图还是原图，返回的嵌入路径都指向真实原图，variant MIME 仍与模型看到的图片一致。

既有测试继续覆盖动态数据不进入 Runtime system、领域模型不包含运行时管理字段、Skill 快照位置、工具授权、扩权、DOM 选择、附件压缩、简报服务和完成检查。

这些是可重复的工程回归测试，不是模型创作质量评测。未调用真实模型进行 A/B，不报告“成功率提高多少”或“token 减少多少”。后续可用同一模型、同一项目初始快照和相同输入对比以下场景：

| 评估任务 | 重点观察 |
| --- | --- |
| 从空目录创作一套指定页数演示 | 是否能完成全部页面；第一张有效预览出现时间；是否只遵守真实依赖 |
| 已有空章节中继续创建页面 | 是否正确使用 insert 而非反复 init |
| 仅改多页图表 / 单页局部字号 | 是否避免无关页、Spec 或 Design 写入 |
| 基于图片改布局，压缩后继续 | 是否保留并按需恢复附件，避免凭文件名猜内容 |
| 旧 DOM 选择中的文字已被更新 | 是否读取当前 HTML 后重新定位，而非重复失败 patch |
| 改公共页码 / 标题但未授权全局 | 是否使用原 Run 扩权并正确处理拒绝 |
| 仅设计稿任务、只读 Plan + reviewer | 是否错误要求 HTML / render / finish |
| 正确 HTML 缺少新鲜 render 证据 | 是否只重新渲染，无无意义 HTML 改写 |
| 部分完成后继续剩余页面 | 已完成页是否被无故重做；失败页是否真正恢复 |
| 自由 SVG / 布局和导出准备 | 是否满足主题方向与静态可读性，是否虚构素材/导出能力 |

每次记录任务完成率、工具调用/错误重试数、无关资源改写数、首张有效预览时间、总耗时、上下文用量及人工视觉评分。仅在这些证据形成后判断效率与质量是否改善。
