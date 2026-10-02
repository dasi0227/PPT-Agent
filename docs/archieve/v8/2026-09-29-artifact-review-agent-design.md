# PPT 成果审查 Agent 重构

## 用户决定

Review Agent 仅审查 artifact，移除对计划、执行过程与最终回复的审查。主 Agent 使用 `review_task(demand)` 发起审查；Reviewer 使用 `submit_review(type, reasons)` 提交结论。`type` 为 `approve / check / refuse`，`reasons` 为所有类型均必填的非空字符串数组，不引入对象化问题结构。

Reviewer 可调用 `read_resource`、`read_image`、`render_slide`。不提供修改、Shell、加载 Skill/组件、用户提问、计划、委派或主任务完成工具。保持专用入口，不增加通用 sub_agent 工具。

前端采用已确认 Demo 的 Microscope 工具行与灰色详情卡片。调用成功时图标统一绿色；内部状态分别显示绿色「审查通过」、黄色「需要核实」、红色「拒绝交付」，不再放状态图标。原因使用无序列表，圆点保持普通深色。进行中显示旋转加载图标，不可展开、不显示展开箭头。此次不更改其他工具的加载交互，也不更改 DESIGN.md。

## 职责与协议

- 用户要求及其后续纠正是验收依据，demand 说明本次审查范围与重点，不能覆盖用户要求。
- approve：必要检查已有充分证据，没有发现影响本次验收的明确问题；原因说明审查了什么及批准依据。
- check：关键事实、要求解释或观察仍不确定，需主 Agent 继续核实；优先用自身读取和渲染能力解决材料缺口。
- refuse：已有证据证明当前成果存在阻碍交付的缺陷；同时存在确定缺陷和不确定事项时使用 refuse，并在原因中说明。
- 主 Agent 根据结论决定下一步。审查不授予写权限，不自动 finish，也不新增强制完成门禁。
- 工具超时、取消、模型文本代替工具提交、非法参数、预算耗尽属于执行失败，没有业务结论。三种合法结论都属于审查调用成功。

工具参数 Schema 和后端校验同时约束合法枚举、非空 reasons 和非空数组项，拒绝额外字段。取消旧 checks/code/summary 协议，不保留旧工具别名或双读。

## 材料与版本

材料由 Runtime 组装，包含：

1. 当前 Run 的用户原始指令，以及运行中注入的补充指令、问答产生的用户要求；附件与 DOM 引用随要求保存。用户选项单独传递。
2. 主 Agent 的 demand 原文。
3. Run 起点与当前项目文件的累计净差异，包括新增、修改、删除。文字文件使用 unified diff，二进制文件提供变化身份，相关图像由读图或渲染查看。
4. 页面顺序、稳定 ID、HTML 存在情况、已有 HTML 的最新渲染元信息及实际图片内容。过期截图明确标记 stale；没有截图的页面不会从页面清单消失。

不复制主 Agent 的对话、工具调用历史、RequirementLedger 完成状态、计划完成状态或自述结果作为审查证明。Reviewer 自己的读取和渲染结果保留在其独立上下文中。

新 Run 在工具执行之前记录源文件基线，保存到 `.runtime/review-baselines/<run-id-hash>.json`。基线包含文本内容及文件身份；二进制保存身份与大小。源文件边界与项目历史一致，排除 `.runtime`、`.git`、`.run`、`.commit-tmp`、`versions`，包含通过 run_command 改动的其他项目文件。恢复相同 Run 使用原基线，不重新以恢复时内容作起点；缺失基线时审查明确失败，不编造差异。基线读取失败不阻止无审查需求的普通任务继续执行。

`RuntimeCheckpoint.review_instructions` 保存原始要求及纠正，不因主 Agent 的文字压缩丢失。快照避免跟随符号链接；材料快照单文件最多 64 MiB、累计最多 128 MiB，超限明确失败，不静默截断。已存在截图全量提供，独立请求使用现有 token 估算检查上下文空间；不为了通过预算而暗中丢页、删图或截断 diff。

结论提交后再次核对项目源文件身份。如果审查期间成果发生变化，不发布已过期的结论，而返回执行失败，要求针对新版本重新审查。

## 执行与恢复

`LLMTaskReviewer` 使用现有模型 Provider，维持独立 messages 和 continuation。系统上下文由现有 `core/quality.md` 与重写后的 `subagent/reviewer/agent.md` 组成。工具层重新注册明确的三个读取/渲染工具，不把默认主 Agent 工具表整体交给 Reviewer。

审查最多 32 次模型响应、128 次读取/渲染调用，5 分钟超时，并受主 Run 的预算和取消控制；连续三次检查工具失败停止。本次独立上下文不进入主 Agent 的文字压缩机制。最终响应必须仅有一次 submit_review；纯文本、混合提交、未知工具均不作为成功结果。

渲染结果进入当前 Run 的证据账本及截图索引，并保存检查点，主 Agent 可以继续使用这些证据；不会把 Reviewer 的读图结果混入主 Agent 的“已读图片”集合，主 Agent 需要查看时仍自行读图。

`pending_review` 在开始前保存，记录父工具调用。结论在发布公共完成事件前写入 pending_review 检查点；恢复时重发已经保存的结论，避免将成功结果误改成失败。恢复发现尚无结论的审查时，将该调用标记为执行中断并向主循环返回失败观察，不虚构中断前的结论。Reviewer 子循环不自动继续旧模型推理；主 Agent 可以重新发起审查。

## 公共事件与界面

复用 `tool.started`、`tool.completed`，审查子工具过程仅进内部 trace，不堆叠到主时间线。

成功完成事件附带 `review: { type, reasons }`；执行失败不含 review，沿用公共 error。后端事件校验、前端 SSE 校验、TypeScript 类型及 reducer 同步更新。通过现有会话日志持久化和 history hydration 恢复，不新增结果表，移除旧 SemanticReviewStore 与诊断结构。

工具行使用「正在审查 PPT 成果」「已审查 PPT 成果」「成果审查未完成」。完成后默认折叠；展开显示三色纯文字状态与完整原因列表。失败显示实际公共错误，不能显示「拒绝交付」代替服务异常。

## 验证交接

遵循仓库约定，本次不运行测试、构建或浏览器验证。新增与更新的行为用例覆盖：

- submit_review 所有结论必须有原因、非法字段与纯文本不能作为提交。
- 渲染、读图、返回图片后的多轮评估，以及工具白名单、取消与上下文超限。
- Run 累计净差异、恢复保留起点、改回原文、缺失基线和最新已有截图。
- 审查拒绝仍为成功工具事件，执行失败没有业务结论。
- plan 模式不披露 review_task；结果返回主循环。
- SSE/history 恢复三种结果，原因列表展示，进行中不可展开。

当前交付状态：实现及源码核对已完成，以上用例尚未执行，编译、真实模型联调与界面验收尚未确认通过。手动验证结果返回前，不将“代码已实现”等同于“整体已验收”。

| 要求 | 实现依据 | 待手动确认 |
| --- | --- | --- |
| 仅成果审查，demand 必填 | runtime.go 的 controlSchemas 与 executeControl；mode/execute.md | execute 可调用，plan 不披露，无 finish 审查门禁 |
| 三种结果及原因必填 | reviewer.go 的 submitReviewSchema / parseReviewSubmission；model/review.go | 真实模型通过工具提交，非法提交明确失败 |
| Run 起点差异与用户要求 | review_context.go；runtime.go 的 review_instructions 检查点 | 多次修改、继续运行、补充指令后材料仍正确 |
| 最新截图、读取与渲染 | buildReviewMaterial；reviewArtifacts 的三个工具白名单 | 截图实际进入模型，过期图可重新渲染并读取 |
| 持久化与中断恢复 | pending_review 检查点；tool.completed 公共事件 | 停止、刷新与恢复不遗失结论、不残留进行中 |
| 结果呈现 | SSE 类型与校验、eventReducer、ToolActivityRow | 三种绿色 Microscope、三色标题、深色圆点、进行中不可展开 |

用户可运行后端 `go test ./internal/workflow ./internal/model ./internal/service ./internal/store/sqlite`，以及前端 `npx vitest run src/features/agent/ReviewActivity.test.tsx src/api/sse.test.ts src/features/agent/historyHydrator.test.ts`。手动验收还需覆盖真实模型读取/渲染后提交、刷新恢复、用户停止、执行失败提示，以及实际页面下的三种结论样式。
