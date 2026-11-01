# TODO：Meta / Constraints 提示词与工具适配

范围：提示词、模型可见工具及其直接依赖的执行链路。本清单不展开项目初始化、普通管理接口和前端改造；实施时须与新资源协议同步切换。

## 执行约束

- **现有提示词已经定稿。创建、完善和重构 Prompt 时，必须完全参照原有表达方式、句式、措辞、结构和写法，仅做新资源职责所需的适配与转换。**
- 优先复用原句，仅替换资源名、工具名、字段路径及必须变化的行为描述；字段说明随职责移动，不另起一套表述。
- 不添加无意义的说明、解释、补充、示例、提醒或通用最佳实践；不借本次调整重写无关规则、改变语气或扩充篇幅。
- 新工具沿用原工具的 Markdown 正文与 JSON 参数说明分工，保持成功返回、审批修改和拒绝反馈的表达方式。
- 直接切换到新资源和工具名称，不保留旧名称别名、兼容层、双读双写或历史数据迁移。
- 不机械替换所有 `manifest`、`design`：`ContextManifest`、`prompt_manifest` 及普通视觉设计表述不属于本次资源重命名。
- 遵循项目 AGENTS.md：独立完成；构建、测试和运行验证交由用户手动执行；不新增普通覆盖率测试。

## 目标协议

| 资源 | 内容 | 编辑工具 |
| --- | --- | --- |
| `.meta.json` | `title`、`language`、`slide_count`、`audience`、`goal` | `edit_meta` |
| `.constraints.json` | 内容与视觉要求、限制 | `edit_constraints` |
| `.outline.json` | 章节、页面、标题和顺序 | `edit_outline` |

```json
{
  "content": {
    "requirements": [],
    "prohibitions": []
  },
  "visual": {
    "requirements": [],
    "prohibitions": []
  }
}
```

- [ ] `edit_manifest` 替换为 `edit_meta`；基本字段说明保留，内容要求与限制移至 `edit_constraints`。
- [ ] `edit_design` 替换为 `edit_constraints`；原视觉字段说明移至 `visual`，原内容字段说明移至 `content`。
- [ ] `read_resource.type` 改为 `meta / constraints / outline / spec / html`；全局资源仍不接受 `slide_id`。
- [ ] 基本信息只存一份，不在约束数组重复记录；`slide_count` 继续表示期望页数，实际页数由 outline 确定。
- [ ] 保留基本信息的空字符串与约束的空数组语义；空约束不要求 Agent 补齐，只记录用户明确提出的全局约束。
- [ ] 保留现有字段长度与数组条数限制，不因合并资源顺便改变限制。

## 1. 编辑语义与 Schema

- [ ] 区分完整文件 Schema 与局部编辑参数 Schema：文件必须包含 `content`、`visual` 及四组数组；工具允许只提供一个领域中的一组数组。
- [ ] 明确局部更新：省略领域或字段保留原值；提供的数组整体替换；`[]` 清空对应数组；`null`、未知字段和无有效字段的空对象不接受。
- [ ] `backend/internal/pptmutation/resource_edit.go`：将 constraints 的顶层替换改为按明确的领域与字段合并，避免覆盖未提交的同组字段；不影响其他资源的编辑语义。
- [ ] `backend/schemas/manifest.schema.json`、`design.schema.json`、`schemas.go`：切换为 meta、constraints 定义、名称和加载映射。
- [ ] `backend/internal/spec/types.go`、`source_validation.go`、`validate.go`：同步工具依赖的数据类型、默认值、严格解码与校验入口。
- [ ] `backend/internal/tooldefinition/inputs.go`：更新工具名称、读取枚举与资源 Schema 映射；为 constraints 生成嵌套局部编辑参数，不直接沿用嵌套必填规则。
- [ ] 编辑成功继续返回完整保存后的 JSON 对象，包含审批期间用户的修改；拒绝不保存变更。

## 2. 已定稿 Prompt 适配

- [ ] `backend/prompts/core/agent.md`：资源名称映射改为 `meta → 基本信息`、`constraints → 全局约束`。
- [ ] `backend/prompts/core/references.md`：按原表格和原句式调整两个资源的 Meaning / Purpose，保留其他资源及上下文说明。
- [ ] `backend/prompts/addition/boundaries.md`：将约束归属改为 `constraints.content`、`constraints.visual`；保留用户明确提出、允许为空、不为初始化补条目的规则。
- [ ] `backend/prompts/playbook/html.md`：将创作参考资源改为 `meta`、`constraints`、`spec`，保留原步骤。
- [ ] `backend/prompts/tool/edit_manifest.md`、`edit_manifest.json`：改为 `edit_meta.md`、`edit_meta.json`，同步模块 ID、desc、文件路径、正文和五个字段说明。
- [ ] `backend/prompts/tool/edit_design.md`、`edit_design.json`：改为 `edit_constraints.md`、`edit_constraints.json`，同步模块 ID、desc、文件路径、正文和嵌套字段说明。
- [ ] `edit_constraints.json`：提供 `content`、`visual` 及 `content.requirements`、`content.prohibitions`、`visual.requirements`、`visual.prohibitions` 六个参数路径的说明；复用原内容与视觉字段措辞。
- [ ] `backend/prompts/tool/read_resource.json`：更新资源类型说明；`read_resource.md` 的通用读取约定原则上保留。
- [ ] `backend/prompts/decision/preview.md`：按原 Input 写法说明 meta、constraints、spec、html 的职责，保留评估规则。
- [ ] `backend/prompts/subagent/reviewer/agent.md`：将 `current_sources` 的资源清单改为 meta、constraints、outline，保留审查方式和输出要求。
- [ ] 核对 `backend/prompts/command/suggest.md`、`command/handoff.md` 的项目上下文说明与文件引用示例，只转换实际受影响的表述。
- [ ] `backend/internal/prompt/catalog.go`：更新工具模块 ID、路径与受影响的版本标识；不改变既定装配顺序。

## 3. 工具读写、注册与错误恢复

- [ ] `backend/internal/workflow/default_tools.go`：替换两个编辑工具的注册名称，沿用原权限和阶段控制。
- [ ] `backend/internal/workflow/ppt_tools.go`：更新工具到资源的映射、字段编辑工具识别、文件识别及读写返回链路。
- [ ] `backend/internal/workflow/ppt_helpers.go`：更新资源解析、ArtifactRef、文件路径、全局资源读取和默认值。
- [ ] `backend/internal/workflow/domain.go`：更新资源种类及其对应的 `deck:meta`、`deck:constraints` 标识。
- [ ] `backend/internal/workflow/resource_errors.go`：更新缺失资源的恢复建议与工具名；核对嵌套校验错误分类，不把用户参数错误误报为内部错误。

## 4. Jev 工具裁剪

- [ ] `backend/internal/workflow/decision_tools.go`：将判断项改为 meta、constraints、outline，并同步结果读取和工具映射。
- [ ] constraints 判断覆盖内容与视觉约束；任一领域可能需要修改时，保留 `edit_constraints`。
- [ ] 按原提问句式转换 Go 中的模型问题，更新裁剪策略标识及缓存依据；`backend/prompts/jev/prune.md` 的通用规则保持原样。

## 5. 上下文、状态与批量依赖

- [ ] `backend/internal/contextengine/types.go`、`loaders.go`、`assembler.go`：切换演示资源的类型、加载路径和上下文表示，保留独立的 `ContextManifest`。
- [ ] `backend/internal/workflow/project_state.go`：更新资源状态字段、源文件读取、变更比较和上下文刷新。
- [ ] `backend/internal/workflow/resource_context.go`：更新资源版本键，保留完整内容与哈希匹配后才判定 fresh 的规则。
- [ ] `backend/internal/workflow/staged_artifacts.go`、`runtime.go`：更新暂存视图与工具提交后的内存上下文。
- [ ] `backend/internal/workflow/tool_dependencies.go`：更新全局编辑串行关系、渲染读取依赖和页面编辑的前置依赖；同一 constraints 文件的编辑不能相互覆盖。
- [ ] `backend/internal/contextengine/project_resource_context.go`：读取新文件，按基本信息、内容约束、视觉约束生成建议与交接所用上下文。
- [ ] `backend/internal/contextengine/command_context.go`：同步新上下文标签的转义，避免资源文本成为伪造结构块。

## 6. 审批、提交与恢复

- [ ] `backend/internal/workflow/runtime.go`：更新全局资源编辑的审批触发名单。
- [ ] `backend/internal/pptmutation/approved_resource.go`：更新审批资源白名单、完整草稿解析、校验与保存。
- [ ] 核对 `backend/internal/workflow/resource_approval.go`：审批基线、用户修改、拒绝反馈和完整保存结果沿用原流程。
- [ ] `backend/internal/workflow/session.go`：更新结构化编辑来源识别，保留语义相等时的冲突处理和回滚行为。
- [ ] `backend/internal/workflow/recovery.go`、`tool_batch_recovery.go`：更新资源路径与恢复后的上下文刷新。
- [ ] `backend/internal/workflow/completion.go`、`validation_evidence.go`：新资源继续进入 Schema 证据与完成检查，不把空约束判为缺陷。

## 7. 渲染评估、审查与其他工具

- [ ] `backend/internal/workflow/preview.go`：更新评估输入块、资源读取及 Go 中的 mismatch 问题；按原句式转换。
- [ ] 确定 mismatch 分类：覆盖基本信息、内容约束、视觉约束、单页 spec 的偏离，避免将原内容要求机械改名为 meta 后丢失归属。
- [ ] `backend/internal/model/preview.go`：同步评估问题、分类枚举、结果校验与版本标识。
- [ ] `backend/internal/workflow/ppt_helpers.go`、`evidence.go`、`backend/internal/spec/render_inputs.go`：将完整 meta、constraints 纳入创作与评估证据，移除旧扁平 Design 的归一化假设。
- [ ] `backend/internal/workflow/image_dependencies.go`：可见演示标题改从 meta 读取；继续区分像素变化与约束变化，不因纯约束编辑强制重渲染相同截图。
- [ ] `backend/internal/workflow/review_context.go`：Reviewer 接收完整 meta、constraints、outline 及相应变更证据，保留 `review_ppt` 调用参数与结果正文。
- [ ] `backend/internal/workflow/project_command_tool.go`：`run_command` 读取新文件后仍能登记为对应资源的完整观察结果。
- [ ] `backend/internal/gitcommit/executor.go`：同时更新 Git 忽略白名单和源文件收集白名单，使新资源进入 `git_commit`。

## 8. 工具事件、名称与差异

- [ ] `backend/internal/model/public_event.go`：更新工具名、审批资源、拒绝结果和 deck target 的校验枚举。
- [ ] `backend/internal/workflow/public_events.go`、`runtime.go`：更新工具目标、显示名称和活动分类；constraints 不再一律归为视觉编辑。
- [ ] `backend/internal/model/public_text.go`：更新文件名、资源名、工具名的用户可见映射。
- [ ] `backend/internal/workflow/final_diff.go`：更新文件到资源的映射和排序；复用现有递归差异算法，保留四组约束的独立字段路径。

## 9. 复用与范围核对

- [ ] 复用 `backend/internal/tooldefinition/schema.go` 的嵌套参数说明能力、`catalog.go` 的工具装配能力及 `backend/internal/prompt/tool_args.go` 的说明加载能力。
- [ ] 保留四种模式、公共 HTML 契约、创作规则、样式规则和 `backend/internal/workflow/prompt_modules.go` 的既定装配顺序。
- [ ] 保留 `edit_outline`、`edit_spec`、`edit_html` 的参数契约；保留 `render_slide`、`review_ppt`、`run_command`、`git_commit` 及建议、交接工具的调用参数。
- [ ] 核对 `docs/note/assemble.html` 中工具清单、裁剪项与上下文示意，仅同步受影响的提示词与工具说明；不修改历史文档。
- [ ] 检查旧资源名与工具名残留，区分演示资源、运行上下文清单和普通设计表述，不做全局字符串替换。

## 10. 既有测试适配与人工验收

- [ ] 同步提示词加载、工具参数、资源读写、Jev 裁剪、审批、批量调用、冲突恢复、评估审查、事件与 Git 白名单的既有测试和夹具。
- [ ] Prompt 逐处对照原稿：仅保留职责、标识、路径及必要行为变化，不出现无关扩写。
- [ ] 手动确认新资源可读取，旧工具名与旧资源类型不再公开或接受。
- [ ] 手动确认仅更新 `content.requirements` 时，另外三组数组完整保留；`[]` 仅清空目标数组；无效参数不产生部分写入。
- [ ] 手动确认审批修改后的完整内容进入工具返回和模型上下文，拒绝不落盘，过期基线不能继续覆盖。
- [ ] 手动确认同批次约束编辑、页面编辑与渲染遵守依赖，后续读取和评估使用最新资源。
- [ ] 手动确认评估与审查同时获得基本信息、内容约束、视觉约束；约束变化使旧评估过期，截图按实际像素依赖决定复用。
- [ ] 手动确认 `run_command` 读取可更新资源观察状态，`git_commit` 包含新资源，事件、差异及完成检查使用新标识。

