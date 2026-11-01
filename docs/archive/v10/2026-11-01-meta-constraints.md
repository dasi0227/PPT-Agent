# Meta / Constraints 提示词与工具适配

范围：提示词、模型可见工具及其直接依赖的执行链路；同步项目初始化、管理接口、导出和前端资源协议。

实施状态：代码、Prompt 和既有测试已适配，已逐处对照源码与差异；尚未执行构建、测试和运行验收。

## 执行约束

- **现有提示词已经定稿。创建、完善和重构 Prompt 时，必须完全参照原有表达方式、句式、措辞、结构和写法，仅做新资源职责所需的适配与转换。**
- 优先复用原句，仅替换资源名、工具名、字段路径及必须变化的行为描述；字段说明随职责移动，不另起一套表述。
- 不添加无意义的说明、解释、补充、示例、提醒或通用最佳实践；不借本次调整重写无关规则、改变语气或扩充篇幅。
- 新工具沿用原工具的 Markdown 正文与 JSON 参数说明分工，保持成功返回、审批修改和拒绝反馈的表达方式。
- 直接切换到新资源和工具名称，不保留旧名称别名、兼容层、双读双写或历史数据迁移。
- 不机械替换所有 `manifest`、`design`：`ContextManifest`、`prompt_manifest` 及普通视觉设计表述不属于本次资源重命名。
- 按用户本次授权使用必要的子智能体；构建、测试和运行验证交由用户手动执行；不新增普通覆盖率测试。

## 目标协议

| 资源 | 内容 | 编辑工具 |
| --- | --- | --- |
| `.meta.json` | `title`、`language`、`count`、`audience`、`goal` | `edit_meta` |
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

工具参数：

```ts
edit_constraints(
  type: "content" | "visual",
  requirements?: string[],
  prohibitions?: string[]
)
```

- [x] `edit_manifest` 替换为 `edit_meta`；基本字段说明保留，内容要求与限制移至 `edit_constraints`。
- [x] `edit_design` 替换为 `edit_constraints`；使用必填的 `type` 选择内容或视觉约束，单次调用只编辑一个领域；同时修改两个领域需要两次调用。
- [x] `read_resource.type` 改为 `meta / constraints / outline / spec / html`；全局资源仍不接受 `slide_id`。
- [x] 基本信息只存一份，不在约束数组重复记录；`count` 继续表示期望页数，实际页数由 outline 确定。
- [x] 保留基本信息的空字符串与约束的空数组语义；空约束不要求 Agent 补齐，只记录用户明确提出的全局约束。
- [x] 保留现有字段长度与数组条数限制，不因合并资源顺便改变限制。

## 0. 必须同步的依赖

- [x] `backend/internal/service/project.go`：新项目预建 `.meta.json`、`.constraints.json`、`.outline.json`；五个基本字段为空字符串，四组约束与目录章节为空数组。
- [x] `backend/internal/service/ppt_mutation.go`、`backend/internal/pptmutation/mutation.go`、`patch.go`：项目快照与管理操作切换新名称，约束写入完整嵌套对象，局部修改限定领域路径，保存前严格校验。
- [x] `backend/internal/export/snapshot.go`：导出读取新资源，演示标题从 meta 获取。
- [x] `frontend/src/api/`、`stores/`、工作区与导航：同步类型、快照、审批、事件、资源路由与入口。
- [x] `frontend/src/features/viewer/`、`features/agent/`：基本信息显示五个字段，全局约束显示四组数组；复用现有管理和审批编辑器，同步名称、状态、差异字段与画布标题。

## 1. 编辑语义与 Schema

- [x] 区分完整文件 Schema 与局部编辑参数 Schema：文件保留 `content`、`visual` 及四组数组；工具只接受扁平的 `type`、`requirements`、`prohibitions` 三个参数，不接受嵌套领域对象。
- [x] `type` 必填且仅接受 `content` 或 `visual`；`requirements`、`prohibitions` 至少提供一个，也可同时提供。
- [x] 明确局部更新：省略的数组保留原值；提供的数组整体替换；`[]` 清空对应数组；另一领域保持原值；拒绝 `null`、未知字段、非法 type 和仅提供 type 的调用。
- [x] `backend/internal/pptmutation/resource_edit.go`：按 type 定位文件中的领域，只替换已提供的数组，避免覆盖未提交的同组字段；不把 type 写入文件，不影响其他资源的编辑语义。
- [x] `backend/schemas/manifest.schema.json`、`design.schema.json`、`schemas.go`：切换为 meta、constraints 定义、名称和加载映射。
- [x] `backend/internal/spec/types.go`、`source_validation.go`、`validate.go`：同步工具依赖的数据类型、默认值、严格解码与校验入口。
- [x] `backend/internal/tooldefinition/inputs.go`：更新工具名称、读取枚举与资源 Schema 映射；为 constraints 生成三个扁平参数，按 type 应用对应领域的数组与条目限制，不直接暴露文件 Schema。
- [x] 编辑成功继续返回完整保存后的 constraints JSON 对象，包含两个领域及审批期间用户的修改；拒绝不保存变更。

## 2. 已定稿 Prompt 适配

- [x] `backend/prompts/core/agent.md`：资源名称映射改为 `meta → 基本信息`、`constraints → 全局约束`。
- [x] `backend/prompts/core/references.md`：按原表格和原句式调整两个资源的 Meaning / Purpose，保留其他资源及上下文说明。
- [x] `backend/prompts/addition/boundaries.md`：将约束归属改为 `constraints.content`、`constraints.visual`；保留用户明确提出、允许为空、不为初始化补条目的规则。
- [x] `backend/prompts/playbook/html.md`：将创作参考资源改为 `meta`、`constraints`、`spec`，保留原步骤。
- [x] `backend/prompts/tool/edit_manifest.md`、`edit_manifest.json`：改为 `edit_meta.md`、`edit_meta.json`，同步模块 ID、desc、文件路径、正文和五个字段说明。
- [x] `backend/prompts/tool/edit_design.md`、`edit_design.json`：改为 `edit_constraints.md`、`edit_constraints.json`，同步模块 ID、desc、文件路径、正文和三个参数说明。
- [x] `edit_constraints.json`：只提供 `type`、`requirements`、`prohibitions` 三个参数说明；type 区分内容与视觉，另两项复用原字段措辞并适配所选领域。
- [x] `backend/prompts/tool/read_resource.json`：更新资源类型说明；`read_resource.md` 的通用读取约定原则上保留。
- [x] `backend/prompts/decision/preview.md`：按原 Input 写法说明 meta、constraints、spec、html 的职责，保留评估规则。
- [x] `backend/prompts/subagent/reviewer/agent.md`：将 `current_sources` 的资源清单改为 meta、constraints、outline，保留审查方式和输出要求。
- [x] 核对 `backend/prompts/command/suggest.md`、`command/handoff.md` 的项目上下文说明与文件引用示例，只转换实际受影响的表述。
- [x] `backend/internal/prompt/catalog.go`：更新工具模块 ID、路径与受影响的版本标识；不改变既定装配顺序。

## 3. 工具读写、注册与错误恢复

- [x] `backend/internal/workflow/default_tools.go`：替换两个编辑工具的注册名称，沿用原权限和阶段控制。
- [x] `backend/internal/workflow/ppt_tools.go`：更新工具到资源的映射、字段编辑工具识别、文件识别及读写返回链路；将 type 作为领域选择参数处理，与实际待写字段分开，资源标识始终为 constraints。
- [x] `backend/internal/workflow/ppt_helpers.go`：更新资源解析、ArtifactRef、文件路径、全局资源读取和默认值。
- [x] `backend/internal/workflow/domain.go`：更新资源种类及其对应的 `deck:meta`、`deck:constraints` 标识。
- [x] `backend/internal/workflow/resource_errors.go`：更新缺失资源的恢复建议与工具名；核对 type 与所选领域字段的校验错误分类，不把用户参数错误误报为内部错误。

## 4. Jev 工具裁剪

- [x] `backend/internal/workflow/decision_tools.go`：将判断项改为 meta、constraints、outline，并同步结果读取和工具映射。
- [x] constraints 判断覆盖内容与视觉约束；任一领域可能需要修改时，保留 `edit_constraints`。
- [x] 按原提问句式转换 Go 中的模型问题，更新裁剪策略标识及缓存依据；`backend/prompts/jev/prune.md` 的通用规则保持原样。

## 5. 上下文、状态与批量依赖

- [x] `backend/internal/contextengine/types.go`、`loaders.go`、`assembler.go`：切换演示资源的类型、加载路径和上下文表示；加载时先按完整 Schema 校验原始 JSON，拒绝缺字段、null 与未知键，保留无效源文件的错误分类及独立的 `ContextManifest`。
- [x] `backend/internal/workflow/project_state.go`：更新资源状态字段、源文件读取、变更比较和上下文刷新。
- [x] `backend/internal/workflow/resource_context.go`：更新资源版本键，保留完整内容与哈希匹配后才判定 fresh 的规则。
- [x] `backend/internal/workflow/staged_artifacts.go`、`runtime.go`：更新暂存视图与工具提交后的内存上下文。
- [x] `backend/internal/workflow/tool_dependencies.go`：更新全局编辑串行关系、渲染读取依赖和页面编辑的前置依赖；不同 type 的调用仍编辑同一 constraints 文件，按既有全局编辑规则串行，不能相互覆盖。
- [x] `backend/internal/contextengine/project_resource_context.go`：读取新文件，按基本信息、内容约束、视觉约束生成建议与交接所用上下文。
- [x] `backend/internal/contextengine/command_context.go`：同步新上下文标签的转义，避免资源文本成为伪造结构块。

## 6. 审批、提交与恢复

- [x] `backend/internal/workflow/runtime.go`：更新全局资源编辑的审批触发名单。
- [x] `backend/internal/pptmutation/approved_resource.go`：更新审批资源白名单、完整草稿解析、校验与保存。
- [x] 核对 `backend/internal/workflow/resource_approval.go`：审批基线、用户修改、拒绝反馈和完整保存结果沿用原流程。
- [x] `backend/internal/workflow/session.go`：更新结构化编辑来源识别，保留语义相等时的冲突处理和回滚行为。
- [x] `backend/internal/workflow/recovery.go`、`tool_batch_recovery.go`：更新资源路径与恢复后的上下文刷新。
- [x] `backend/internal/workflow/completion.go`、`validation_evidence.go`：新资源继续进入 Schema 证据与完成检查，不把空约束判为缺陷。

## 7. 渲染评估、审查与其他工具

- [x] `backend/internal/workflow/preview.go`：更新评估输入块、资源读取及 Go 中的 mismatch 问题；按原句式转换。
- [x] mismatch 分类切换为 `meta_mismatch / content_mismatch / visual_mismatch / spec_mismatch / none`，分别覆盖基本信息、内容约束、视觉约束与单页 spec。
- [x] `backend/internal/model/preview.go`：同步评估问题、分类枚举、结果校验与版本标识。
- [x] `backend/internal/workflow/ppt_helpers.go`、`evidence.go`、`backend/internal/spec/render_inputs.go`：将完整 meta、constraints 纳入创作与评估证据，移除旧扁平 Design 的归一化假设。
- [x] `backend/internal/workflow/image_dependencies.go`：可见演示标题改从 meta 读取；继续区分像素变化与约束变化，不因纯约束编辑强制重渲染相同截图。
- [x] `backend/internal/workflow/review_context.go`：Reviewer 接收完整 meta、constraints、outline 及相应变更证据，保留 `review_ppt` 调用参数与结果正文。
- [x] `backend/internal/workflow/project_command_tool.go`：`run_command` 读取新文件后仍能登记为对应资源的完整观察结果。
- [x] `backend/internal/gitcommit/executor.go`：同时更新 Git 忽略白名单和源文件收集白名单，使新资源进入 `git_commit`。

## 8. 工具事件、名称与差异

- [x] `backend/internal/model/public_event.go`：更新工具名、审批资源、拒绝结果和 deck target 的校验枚举。
- [x] `backend/internal/workflow/public_events.go`、`runtime.go`：更新工具目标、显示名称和活动分类；constraints 不再一律归为视觉编辑。
- [x] `backend/internal/model/public_text.go`：更新文件名、资源名、工具名的用户可见映射。
- [x] `backend/internal/workflow/final_diff.go`：更新文件到资源的映射和排序；复用现有递归差异算法，保留四组约束的独立字段路径。

## 9. 复用与范围核对

- [x] 复用 `backend/internal/tooldefinition/schema.go` 的参数说明能力、`catalog.go` 的工具装配能力及 `backend/internal/prompt/tool_args.go` 的说明加载能力。
- [x] 保留四种模式、公共 HTML 契约、创作规则、样式规则和 `backend/internal/workflow/prompt_modules.go` 的既定装配顺序。
- [x] 保留 `edit_outline`、`edit_spec`、`edit_html` 的参数契约；保留 `render_slide`、`review_ppt`、`run_command`、`git_commit` 及建议、交接工具的调用参数。
- [x] 核对 `docs/note/assemble.html` 中工具清单、裁剪项与上下文示意，仅同步受影响的提示词与工具说明；不修改历史文档。
- [x] 检查旧资源名与工具名残留，区分演示资源、运行上下文清单和普通设计表述，不做全局字符串替换。
- [x] 清理本次链路中的重复解码、重复校验和约束哈希转调函数；删除无生产读取的失效记录字段及其写入逻辑，既有测试改为检查实际保存行为。

## 10. 既有测试适配与人工验收

以下手动验收均未执行，待用户确认；未运行构建、测试或浏览器验证。

- [ ] 手动执行前后端构建与相关既有测试。
- [ ] 手动确认新项目的三个资源文件存在，基本字段为空字符串，四组约束和目录章节为空数组。
- [ ] 手动确认管理编辑、审批草稿、导航、源码、差异与导出均使用新资源；修改单个约束数组不覆盖其他数组。

- [x] 同步提示词加载、工具参数、资源读写、Jev 裁剪、审批、批量调用、冲突恢复、评估审查、事件与 Git 白名单的既有测试和夹具。
- [x] Prompt 逐处对照原稿：仅保留职责、标识、路径及必要行为变化，不出现无关扩写。
- [ ] 手动确认新资源可读取，旧工具名与旧资源类型不再公开或接受。
- [ ] 手动确认传入 `type: "content"` 和 requirements 时，另外三组数组完整保留；`[]` 仅清空所选领域的目标数组；非法 type、缺少数组、嵌套参数及其他无效参数不产生部分写入。
- [ ] 手动确认审批修改后的完整内容进入工具返回和模型上下文，拒绝不落盘，过期基线不能继续覆盖。
- [ ] 手动确认同批次约束编辑、页面编辑与渲染遵守依赖，后续读取和评估使用最新资源。
- [ ] 手动确认评估与审查同时获得基本信息、内容约束、视觉约束；约束变化使旧评估过期，截图按实际像素依赖决定复用。
- [ ] 手动确认 `run_command` 读取可更新资源观察状态，`git_commit` 包含新资源，事件、差异及完成检查使用新标识。

