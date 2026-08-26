# PPT Agent Tool Surface Design

**日期：** 2026-08-02
**状态：** Accepted，已按同日《PPT Resource、String Tool Protocol 与 Spec Renaming Design》同步

## 1. 范围

本文定义 HTML PPT 业务 Agent 的模型可见工具面。它不是通用 Agent 工具集，不提供
Shell、数据库、磁盘路径、Repo 写入或任意 Artifact 操作。

业务工具固定为：

- `read_ppt`
- `mutate_ppt`
- `mutate_ppt`
- `search_refs`
- `render_slide`

控制动作固定为：

- `update_plan`
- `ask_user`
- `finish`

所有策略运行在同一个连续 ReAct Loop 中。Complex 仅增加动态 Plan；Plan 不是 Workflow
DAG，也不产生逐 Step 子循环或独立 Verify/Repair Stage。

## 2. 公开任务模型

```text
target.artifact = spec | presentation
target.level    = deck | slide
interaction     = talk | ask | execute
strategy        = chat | simple | complex
```

UI 可以把 `spec` 显示为“设计稿”，但 API、事件、Prompt、Runtime 和测试使用 `spec`。

## 3. Resource

模型只能使用四类 Resource：

```json
{"type":"deck","part":"outline"}
{"type":"deck","part":"design"}
{"type":"slide","slide_id":"slide-03","part":"spec"}
{"type":"slide","slide_id":"slide-03","part":"html"}
```

内部唯一键分别为：

```text
deck:outline
deck:design
slide:<slide_id>:spec
slide:<slide_id>:html
```

Resource 不接受磁盘路径、项目路径、staging 路径、数据库 ID、版本快照路径或存储
Artifact Kind。`slide_id` 必须是稳定不透明 ID，不能使用 `current`。

## 4. `read_ppt`

模型可见签名：

```text
read_ppt(resource)
```

成功时 Observation 主体就是目标 staged/committed Resource 中实际保存的完整 String：

- Outline、Design、Slide Spec：JSON 文本；
- Slide HTML：HTML 文本。

不得附加路径、hash、Manifest、revision、phase 或 staging 信息。超过上限时返回
`CONTENT_TOO_LARGE`，不得返回截断正文。

## 5. `mutate_ppt`

模型可见签名：

```text
mutate_ppt(resource, content: string)
```

所有 Resource 的 `content` 都是 String。Runtime 解析 JSON 或 HTML，维护
`schema_version`、revision、`project_id`、`slide_id`、source revision 和时间戳，执行
完整领域校验，再写入当前 Run staging、更新 ChangeSet、失效相关 Evidence，并返回简洁
Observation。模型不能控制 Runtime 管理字段。

大范围创建或重建使用 `mutate_ppt`。一次调用只写一个 Resource。

Design 写入时，Runtime 会在同一 staging 事务内派生项目内部的
`common/tokens.css`；`common/base.css` 在项目创建或一次性布局迁移时建立。两者是
Design 的内部消费者产物，不是第五类模型可见 Resource，不进入模型 ChangeSet。Render
Worker 以当前 Run staging 覆盖 committed 项目读取这些派生文件，Commit 时再与
`design.json` 原子落盘。

## 6. `mutate_ppt`

模型可见签名：

```text
mutate_ppt(resource, edits)
```

`edits` 统一为：

```json
[
  {
    "old_text": "必须唯一匹配的原文",
    "new_text": "替换后的文本"
  }
]
```

JSON 和 HTML 使用同一协议。多个 edit 按参数顺序作用于同一内存候选；每个
`old_text` 必须恰好匹配一次。任一 edit 零匹配或多匹配时整个调用失败，不产生部分
staged change。全部替换后重新解析并执行完整领域校验。

## 7. 权限与 Scope

- `talk`、`ask` 禁止写入。
- 只有 `execute` 可以写 staging。
- `artifact=spec` 禁止写 Slide HTML。
- `artifact=presentation` 可写完成 Presentation 所需的 Outline、Design、Slide Spec
  和 Slide HTML。
- `level=slide` 只能写指定页面；Deck Resource 只读。
- `level=deck` 可操作整份范围内的 Deck 与 Slide Resource。
- 每次实际执行都重新检查 disclosure、interaction、scope、strategy 和 phase。
- 所有正常成功退出必须显式调用 `finish` 并通过 Completion Gate。

稳定错误至少包括：

```text
RESOURCE_INVALID
RESOURCE_NOT_DISCLOSED
TARGET_OUT_OF_SCOPE
RESOURCE_NOT_FOUND
CONTENT_TOO_LARGE
CONTENT_INVALID
EDIT_ANCHOR_NOT_FOUND
EDIT_ANCHOR_AMBIGUOUS
REVISION_CONFLICT
RENDER_FAILED
STAGING_REQUIRED
```

## 8. 一次响应多个 Tool Calls

Provider 与 Runtime 保留模型返回的完整 `ToolCalls[]`。

- 独立 `read_ppt`/`search_refs` 限流并发，默认上限 4。
- 不同页面 `render_slide` 限流并发，默认上限 3。
- `mutate_ppt`/`mutate_ppt` 按模型返回顺序串行。
- 同一 Resource 串行。
- 存在数据依赖时按原顺序串行。
- 写调用失败后 fail-fast，不执行后续依赖调用。
- `update_plan` 单独执行。
- `ask_user` 必须是该响应唯一控制动作。
- `finish` 必须是该响应唯一完成动作。

并发由 Runtime 根据工具性质、资源冲突和依赖决定，参数中没有 `parallel` 字段。每个
Tool Call 分别拥有 `call_id`、预算计数、权限检查、Observation、Evidence，以及成对的
`tool.started`/`tool.completed`。

## 9. Domain Schema

领域 JSON 的唯一权威定义为：

```text
backend/schemas/outline.schema.json
backend/schemas/design.schema.json
backend/schemas/slide-spec.schema.json
```

Tool Schema 只约束 Resource 和操作外壳。Runtime Validator 与 Prompt Compiler 消费同一
份领域 Schema。已被消费者理解的 Object 使用 `additionalProperties: false`。

## 10. 渲染

`render_slide` 名称与参数保持不变。后端启动一个常驻 Render Worker 和一个可复用
Chromium Browser 进程。每次调用创建隔离 BrowserContext/Page，完成截图与诊断后关闭
Context/Page，保留 Browser 供后续调用复用。

默认禁止外部网络；资源必须位于当前项目或当前 Run staging 覆盖层。Worker 支持取消、
超时、健康检查和崩溃重启，不同页面最多并发 3 页。

## 11. Completion

写 HTML 后必须在同一 ReAct Loop 内执行 render—observe—repair，直到受影响页面拥有与
最新输入绑定的有效 Evidence。`finish` 是候选提交动作；Completion Gate 可以返回可执行
问题，Agent 继续同一循环处理。Gate 通过后才允许原子 Commit。
