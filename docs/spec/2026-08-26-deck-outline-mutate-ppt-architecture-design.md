# Deck、Outline 与 `mutate_ppt` 整体重构设计

**日期：** 2026-08-26  
**状态：** 已实施；2026-08-28 补充 Runtime 工具契约对齐
**范围：** 后端领域模型、项目文件持久化、Agent 工具面、Prompt/Context、物化与渲染、HTTP API、前端状态与目录交互、运行反馈  
**兼容策略：** 开发期直接切换，不保留旧 Schema、旧工具、旧接口或旧数据迁移分支

## 1. 结论

本次重构采用以下最终架构：

```text
deck.json
  演示级创作信息和基础设置

outline.json
  section / subsection / slide 的唯一层级与唯一顺序

design.json
  全局视觉系统和共享 chrome

slides/<slide_id>/spec.json
  单页语义设计稿，不保存结构归属或页码

slides/<slide_id>/index.html
  Agent 制作的单页 HTML 内容，不写死运行时页码

slides/<slide_id>/materialization.json
  Runtime 维护的 HTML 版本、来源和渲染证明
```

Agent 写工具收敛为一个：

```text
mutate_ppt
```

保留：

```text
read_ppt
search_refs
render_slide
```

删除 Agent 可见的旧写工具：

```text
write_ppt
edit_ppt
```

`mutate_ppt` 每次调用执行一个封闭、强类型的领域 operation：

```text
deck.patch

outline.init
outline.insert
outline.move
outline.update
outline.remove

design.write
design.patch

slide.spec.write
slide.spec.patch

slide.html.write
slide.html.patch
```

核心不变量：

1. `slide_id` 是 Runtime 生成的稳定不透明身份，AI、页码、标题和文件名都不能替代它。
2. 页面顺序只存在于 `outline.json` 的有序树中，不再保存独立 `outline_order`。
3. section/subsection 归属只由树的父子关系表达，slide spec 不再重复保存 `section/subsection`。
4. 工作区“第 N 页”和幻灯片画面页码使用同一个派生序号。
5. 页码由 Runtime 预览/导出框架注入，Agent HTML 不写死页码。
6. 空项目先拥有 `deck.json` 和空 outline；`outline.init` 一次性创建结构和稳定 ID，然后 Agent 逐页写 spec 和 HTML。
7. 已声明但 spec 尚未生成的页面是合法 `pending` 状态，读取返回 200，不返回中间态 422。
8. 前端只保存 `currentSlideId`，序号和目录行都从同一个 outline 快照派生。

## 2. 当前问题

当前实现有以下结构性耦合：

- `outline.json` 同时保存标题、目标、受众、规则、section 列表和 `outline_order`。
- section/subsection 层级在 outline 中，页面归属却存于每个 slide spec 的 `section/subsection`。
- 页面移动需要同时提交完整 `ordered_ids` 和完整 placements，并重写受影响 spec。
- Agent 通过通用字符串资源工具 `write_ppt/edit_ppt` 写 outline、design、spec 和 HTML，Runtime 难以在工具 Schema 层表达结构命令与 ID 分配。
- 空项目没有 slide ID，Agent 只能自行构造 `sli-*` 名称，再先写 outline、后写 spec；两次调用之间可能出现 outline 已引用页面但 spec 尚不存在的非法投影。
- materialization 将整份 outline revision/hash 视为每页 HTML 来源；仅重排页面也可能把所有页面标记为陈旧。
- 前端虽然已经使用稳定 `currentSlideId`，但目录仍需从 `outline_order`、sections、spec placement 和 slide 列表拼装。
- 页码如果进入 Agent HTML，重排页面将迫使大量 HTML 重写。

本设计不在这些结构上继续增加兼容层，而是直接消除重复事实来源。

## 3. 设计目标

### 3.1 必须实现

- 空项目可以合法存在，且不需要预造页面。
- Runtime 统一生成 deck、section、subsection、slide 的正式 ID。
- Agent 能先提交完整页面结构，再依据返回 ID 生成各页设计稿和 HTML。
- 页面、section、subsection 的插入、移动、更新和删除都通过同一结构领域服务完成。
- 页面重排不修改 spec，不重写 HTML，不改变选中页面 ID。
- 前端目录、工作区页序、画面页码都从同一 outline 顺序派生。
- Agent 工具 Schema 能直接阻止非法操作组合，不依赖 Prompt 补救。
- Run staging、渲染证据、Completion Gate 和最终 Commit 保持原子性。
- Prompt、Context Engine、工具披露、公开事件和前端文案同步切换到新模型。

### 3.2 非目标

- 不实现多人实时协同、CRDT 或 fractional indexing。
- 不允许通过页码、标题、路径或 AI 生成 slug 作为持久定位。
- 不为旧 `outline.json`、旧 spec、旧数据库开发数据或旧 HTTP 客户端保留兼容。
- 不新增一个只在初始化时使用的独立 `create_deck_structure` 工具。
- 不把 HTML 改成 Runtime 根据 spec 确定性编译；HTML 仍由 Agent 制作。

## 4. 总体架构

```text
用户要求
  ↓
deck.json ────────────── 演示目标、受众、语言、规则、基础设置
  ↓
outline.json ─────────── 唯一结构树与唯一页面顺序
  ↓                       Runtime 分配稳定 ID
slides/<id>/spec.json ── 单页语义设计稿
  ↓
slides/<id>/index.html ─ Agent 制作的 HTML 内容
  ↓
render_slide ─────────── Chromium 截图、诊断与渲染证明
  ↓
materialization.json ─── Runtime 维护的物化关系
```

数据库继续负责 Runtime/索引职责：

- Project、Run、Thread、Event、Plan、Version。
- Slide 稳定身份、所属项目、当前版本指针、导出时间。
- 活跃 Run 冲突与幂等状态。

数据库不负责：

- 页面顺序。
- section/subsection 归属。
- 页面标题、角色、设计稿内容。
- 当前 outline/design/spec 内容 revision 的权威值。

## 5. 项目文件设计

本次 breaking change 将作者资源 Schema 统一升级到下一主版本，例如 `4.0`。具体数字可以沿用仓库统一版本常量，但所有相关文件必须一次性切换。

### 5.1 `deck.json`

`deck.json` 是真实存在的演示级作者资源，不再只是代码中的 `deck:*` 逻辑前缀。

```json
{
  "version": "4.0",
  "revision": 1,
  "project_id": "pro_01...",

  "title": "什么是好 SKILL",
  "goal": "解释优秀 SKILL 的设计原则",
  "audience": "AI 产品开发者",
  "language": "zh-CN",
  "positioning": "专业、具体、可执行",

  "requirements": [
    "包含具体案例"
  ],
  "prohibitions": [
    "避免虚构数据"
  ],

  "canvas": {
    "aspect_ratio": "16:9"
  },

  "numbering": {
    "enabled": true,
    "hidden_roles": ["cover", "end"],
    "format": "number"
  },

  "created_at": 0,
  "updated_at": 0
}
```

所有字段保持顶层或明确设置对象，不引入未定义的 `brief/*` 包装层。

职责：

- 演示标题、目标、受众、语言、定位。
- 用户要求和禁止事项。
- 画布比例。
- 页码行为策略；页码视觉样式仍由 design chrome 负责。

不负责：

- 页面列表或页面顺序。
- section/subsection。
- 单页内容。
- 全局视觉 token。

Runtime 管理：`version/revision/project_id/created_at/updated_at`。

### 5.2 `outline.json`

`outline.json` 只负责叙事层级、节点元数据和顺序。

```json
{
  "version": "4.0",
  "revision": 1,
  "project_id": "pro_01...",
  "sections": [
    {
      "id": "sec_01...",
      "title": "开场",
      "purpose": "建立主题并给出核心判断",
      "slides": [
        {
          "slide_id": "sli_01...",
          "label": "什么是好 SKILL",
          "role": "cover"
        },
        {
          "slide_id": "sli_02...",
          "label": "内容概览",
          "role": "agenda"
        }
      ],
      "subsections": []
    },
    {
      "id": "sec_02...",
      "title": "核心原则",
      "purpose": "解释优秀 SKILL 的判断标准",
      "slides": [],
      "subsections": [
        {
          "id": "sub_01...",
          "title": "可执行性",
          "slides": [
            {
              "slide_id": "sli_03...",
              "label": "从目标到动作",
              "role": "content"
            }
          ]
        }
      ]
    }
  ],
  "created_at": 0,
  "updated_at": 0
}
```

严格两层不变量继续保留，但改为由树直接表达：

- direct section：`slides` 可有页面，`subsections` 必须为空。
- grouped section：`slides` 必须为空，页面全部位于某个 subsection 的 `slides` 中。
- 同一 section 不允许直属页面和 subsection 页面混存。
- slide 是叶子，不允许更深层级。

删除：

- `outline_order`。
- deck 标题、目标、受众、语言、定位、requirements、prohibitions。

`flattenOutline(outline)` 按以下顺序产生唯一全局页序：

```text
依次遍历 sections
  direct section：依次遍历 section.slides
  grouped section：依次遍历 subsections，再依次遍历 subsection.slides
```

### 5.3 Slide spec

```json
{
  "version": "4.0",
  "revision": 1,
  "project_id": "pro_01...",
  "slide_id": "sli_03...",
  "title": "从目标到动作",
  "key_message": "好的 SKILL 必须把目标翻译成稳定、可验证的执行过程",
  "elements": [
    {
      "type": "diagram",
      "intent": "展示目标、判断、动作和验证的闭环"
    }
  ],
  "layout": "process-flow",
  "created_at": 0,
  "updated_at": 0
}
```

删除：

- `section`。
- `subsection`。
- `role`；页面角色由 outline slide node 唯一拥有。
- `position/page_number`；二者从 outline 派生。

保留 `outline.slide.label` 与 `spec.title` 的不同语义：

- `label` 是目录中的简短导航名称。
- `spec.title` 是画面上的标题或标题意图。
- 初始化时二者通常相同，但允许后续因目录可读性而不同。

### 5.4 `design.json`

保留全局视觉职责：

- theme、direction、density。
- 色彩、字体、间距和 token。
- shared chrome 的 placement/style。
- 页码 chrome 的视觉表现。

`deck.numbering` 决定“是否显示、哪些角色隐藏、显示格式”；`design.chrome[type=page_number]` 决定“放在哪里、长什么样”。

### 5.5 `index.html`

HTML 仍由 Agent 制作，属于可版本化作者产物。

HTML 不允许保存：

- 静态页码。
- 依赖当前 ordinal 的文字。
- section/subsection 归属。
- Runtime 管理的外框和导航状态。

预览、缩略图、Render Worker 和导出必须使用同一个 Runtime frame，把以下上下文注入 HTML 外层：

```text
slide_id
ordinal
total
section title/index
subsection title/index
deck numbering policy
design chrome style
```

### 5.6 `materialization.json`

HTML 内容 freshness 与 Runtime frame context 必须分离：

```json
{
  "version": "4.0",
  "artifact": {
    "revision": 3,
    "hash": "sha256:..."
  },
  "source": {
    "deck_revision": 2,
    "outline_node_hash": "sha256:...",
    "spec_revision": 4,
    "design_revision": 3,
    "hash": "sha256:..."
  },
  "frame": {
    "context_hash": "sha256:..."
  },
  "rendered_at": 0
}
```

`outline_node_hash` 只覆盖当前 slide 的语义节点和必要祖先元数据，不包含兄弟顺序或 ordinal。这样纯重排不会使 HTML 内容陈旧。

`frame.context_hash` 覆盖当前 ordinal、总页数、层级标题、numbering policy 和 chrome。frame 是 Runtime 确定性产物；frame 变化不要求 Agent 重写 HTML。

## 6. 稳定 ID、顺序和页码

### 6.1 ID

- 正式 section/subsection/slide ID 全部由 Runtime 生成。
- ID 使用稳定不透明短 ID，例如 `sec_*`、`sub_*`、`sli_*`。
- ID 不包含页码、标题或语义 slug。
- Agent 初始化时只提交 `client_ref`，Runtime 返回 `client_ref -> id` 映射。
- `client_ref` 只在一次工具调用及其结果中存在，不持久化。

### 6.2 顺序

有序数组是持久化顺序的唯一来源。移动数组节点是正常且必要的状态变化，不是性能问题。

不采用：

- 每页 `position` 字段作为权威顺序。
- 链表 next/previous。
- fractional index。
- CRDT。

前端和 Agent 都不提交数字 index 作为定位。结构位置使用稳定锚点：

```json
{
  "parent_id": "sub_01...",
  "before_id": "sli_03..."
}
```

或者：

```json
{
  "parent_id": "sub_01...",
  "after_id": "sli_02..."
}
```

`before_id/after_id` 最多提供一个；均省略表示追加。

### 6.3 页码

```text
ordinal = slide_id 在 flattenOutline(outline) 中的索引 + 1
```

规则：

- 工作区始终显示真实 ordinal。
- 幻灯片画面使用相同 ordinal。
- 封面可以隐藏页码，但仍计为第 1 页；下一页仍显示 2。
- 除非未来明确新增编号策略，否则不支持“封面不计数”或“章节重新编号”。
- 移动页面只改变派生 ordinal，不改变 `slide_id`。

## 7. 初始化流程

### 7.1 项目创建

项目创建时：

1. Runtime 写入有效的 `deck.json`。
2. Runtime 写入空的 `outline.json`，`sections=[]`，处于可初始化状态。
3. Runtime 写入默认 `design.json`。
4. 不创建 slide，不生成 `slide_id`，不创建占位 spec。
5. 前端显示真正的空目录和空画布。

### 7.2 Plan 批准后

前端立即从审批事件进入 running，并显示“正在启动执行”。Runtime 随后的 progress/tool 事件替换为更具体状态。

### 7.3 Agent 初始化结构

Agent 调用：

```json
{
  "op": "outline.init",
  "structure": [
    {
      "client_ref": "opening",
      "title": "开场",
      "purpose": "建立主题",
      "slides": [
        {
          "client_ref": "cover",
          "label": "什么是好 SKILL",
          "role": "cover"
        }
      ],
      "subsections": []
    }
  ]
}
```

Runtime 原子完成：

- 校验严格两层结构。
- 生成正式 section/subsection/slide ID。
- 更新 staging outline。
- 在 RunSession 注册 provisional slide identities。
- 将所有新页面标为 `pending`。
- 返回 ID 映射和 canonical outline revision。

```json
{
  "created": {
    "opening": "sec_01...",
    "cover": "sli_01..."
  },
  "outline_revision": 1,
  "affected_slide_ids": ["sli_01..."]
}
```

后续 `slide.spec.write` 和 `slide.html.write` 只能使用返回的正式 ID。

### 7.4 Pending 是合法状态

已在 outline 中声明、但 spec 尚未生成的页面返回：

```json
{
  "slide_id": "sli_01...",
  "spec_state": "pending",
  "spec": null,
  "html_state": "not_materialized"
}
```

- HTTP 200。
- 前端显示等待/生成中骨架。
- Completion Gate 在整个 Run 完成前要求所有目标页面拥有合法 spec、HTML 和渲染证据。
- 如果 Run 失败或取消，未 Commit 的 provisional identities 和 staging 文件整体丢弃。

### 7.5 Active Run 前端快照

为了让用户看到真实进度，又避免旧的中间态 422：

- `mutate_ppt` 成功后发送带 `content_revision/run_id` 的公开事件。
- 前端只在结构化 mutation 完成后读取 active Run overlay 快照，不在 tool started 或半写入时刷新。
- `GET project content` 支持同一项目当前 active Run 的只读 overlay；无权限的客户端仍只读 committed 快照。
- overlay 快照允许 pending 页面，因此始终是合法自洽状态。
- Run finish 后前端切换到 committed 快照；失败/取消则回到 committed 快照。

## 8. `mutate_ppt` 工具协议

### 8.1 顶层设计

`mutate_ppt` 每次调用只执行一个 operation：

```json
{
  "op": "slide.spec.write",
  "slide_id": "sli_01...",
  "spec": {}
}
```

选择单 operation 而不是 `operations[]` 的原因：

- JSON Schema 可按 `op` 使用清晰的 discriminated union。
- 当前 Provider/Runtime 已支持一次模型响应多个 Tool Calls，并会按资源依赖排序。
- 整个 RunSession 仍是原子 staging；多次工具调用不会产生 committed 部分状态。
- `outline.init` 本身已经能原子创建完整结构。

字段名使用 `op`，不使用外层 `type`，避免和 outline node `type/kind` 混淆。

### 8.2 为什么不使用 `target + type`

不采用：

```json
{
  "target": "html",
  "type": "move"
}
```

因为独立枚举会产生 `html.move`、`design.remove`、`deck.init` 等非法笛卡尔组合。对 Agent 暴露闭合的 namespaced `op` 更可靠。

Runtime 内部和公共事件仍可把 operation 映射为结构化 target/action：

```json
{
  "action": "write",
  "target": {
    "type": "slide",
    "slide_id": "sli_01...",
    "part": "html"
  }
}
```

后端使用显式 switch/typed command 映射，不依赖字符串 split 猜语义。

### 8.3 Operation 清单

#### `deck.patch`

```json
{
  "op": "deck.patch",
  "patch": [
    {
      "op": "replace",
      "path": "/audience",
      "value": "企业管理者"
    }
  ]
}
```

不提供 `deck.write`；`deck.json` 由 Runtime 在项目创建时初始化，Agent 不能整体覆盖用户要求。

#### `outline.init`

- 仅空 outline 可用。
- 输入完整 draft tree，节点使用 `client_ref`。
- Runtime 生成正式 ID。
- 整体原子写入。

#### `outline.insert`

```json
{
  "op": "outline.insert",
  "node": {
    "kind": "slide",
    "client_ref": "case-page",
    "label": "行业案例",
    "role": "content"
  },
  "position": {
    "parent_id": "sub_01...",
    "after_id": "sli_02..."
  }
}
```

- 可插入 section、subsection 或 slide。
- Runtime 生成正式 ID。
- 插入 subsection 到已有直属页面的 section 时，必须显式提供 `direct_slides_policy: "move_into_new_subsection"`；否则拒绝，避免隐式大范围移动。

#### `outline.move`

```json
{
  "op": "outline.move",
  "node_id": "sli_03...",
  "position": {
    "parent_id": "sub_02...",
    "before_id": "sli_04..."
  }
}
```

- section 移动整个 section 子树。
- subsection 移动整个 subsection 子树。
- slide 移动一个叶子。
- Runtime 校验不会产生循环或违反严格两层。
- 纯 reorder 不使 slide spec 或 HTML body stale。

#### `outline.update`

```json
{
  "op": "outline.update",
  "node_id": "sli_03...",
  "changes": {
    "label": "更新后的目录名称",
    "role": "summary"
  }
}
```

白名单：

- section：`title/purpose`。
- subsection：`title`。
- slide：`label/role`。

不能通过 update 修改 ID、children 或顺序。

#### `outline.remove`

```json
{
  "op": "outline.remove",
  "node_id": "sli_03..."
}
```

- 删除 slide：移除 outline node，并删除/归档对应 spec、HTML、materialization 和版本指针。
- 删除 section/subsection：默认只允许空节点。
- 不提供默认级联删除。
- 删除最后一个且仍拥有页面的 subsection 时，必须显式提供 `child_policy: "promote_to_section"`；否则拒绝。

#### `design.write`

- 第一次建立或完整重建 design。
- 输入 Agent 可写字段，Runtime 填充管理字段并派生 tokens。

#### `design.patch`

- 对 `design.json` 使用受限 JSON Patch。

#### `slide.spec.write`

- 第一次生成或完整替换页面设计稿。
- `slide_id` 必须已存在于当前 staging/committed outline。
- Runtime 填充管理字段。

#### `slide.spec.patch`

- 对 spec 使用受限 JSON Patch。
- 不能修改 Runtime 字段和结构字段。

#### `slide.html.write`

- 第一次生成或完整替换 HTML。
- Runtime 完成静态 HTML lint 后写 staging。
- 写入不等于渲染通过，随后仍需 `render_slide`。

#### `slide.html.patch`

```json
{
  "op": "slide.html.patch",
  "slide_id": "sli_03...",
  "edits": [
    {
      "old_text": "<h1>旧标题</h1>",
      "new_text": "<h1>新标题</h1>"
    }
  ]
}
```

- 使用 ordered exact replacement，不使用 JSON Patch。
- 每个 `old_text` 必须唯一命中。
- 任一零命中或多命中则整次 operation 失败。
- 修改后重新执行完整 HTML lint。

### 8.4 受限 JSON Patch

只支持：

```text
add
remove
replace
```

Schema：

```ts
type RestrictedJsonPatch =
  | { op: "add"; path: string; value: unknown }
  | { op: "remove"; path: string }
  | { op: "replace"; path: string; value: unknown }
```

不支持 `move/copy/test`。结构移动使用 `outline.move`；并发冲突由 revision/baseline 校验解决。

所有 patch：

- 按数组顺序应用。
- 任一失败整体回滚。
- 最终结果必须通过完整领域 Schema。
- 禁止修改 runtime-managed path。
- 每种 operation 使用自己的 path 白名单。

### 8.5 Scope 裁剪

- Chat/Grill/Plan：不披露 `mutate_ppt`。
- Execute + Spec + slide scope：只披露绑定当前 slide ID 的 `slide.spec.write/patch`。
- Execute + PPT + slide scope：只披露绑定当前 slide ID 的 `slide.spec.*`、`slide.html.*`。
- Execute + PPT + deck scope：披露全部 operation。
- `outline.init` 只在 deck scope 且 outline 为空时进入 Schema enum。
- `read_ppt` 与 `search_refs` 是低风险只读工具；`render_slide` 仅在 Execute + PPT scope 进入 Schema；它们的资源参数同样按 scope 裁剪。
- `mutate_ppt` 的内部 capability 固定为 `ppt.mutate`、风险为 medium。Runtime 必须用同一条策略同时决定工具披露和执行；不得存在“Schema 已披露但执行时必然拒绝”的第二套规则。
- `create_plan`、`update_plan`、`ask_user`、`review_completion`、`finish` 是 Runtime control actions，按 phase/plan state 单独披露，且一次模型响应只能包含一个 control action。
- 已披露的 domain tool 若仍返回 `CAPABILITY_DENIED`，这是 Runtime 配置不变量失败，Runtime 立即终止本次 Run，不交给模型重试。

### 8.6 返回值

```json
{
  "operation": "slide.spec.write",
  "target": {
    "type": "slide",
    "slide_id": "sli_03...",
    "part": "spec"
  },
  "revisions": {
    "spec": 2
  },
  "created": {},
  "affected_slide_ids": ["sli_03..."],
  "invalidated_slide_ids": ["sli_03..."]
}
```

错误必须包含稳定 code、operation、字段 path 和可执行修复信息。

## 9. 读取和渲染工具

### 9.1 `read_ppt`

保留 `read_ppt`，但资源类型增加真实 deck：

```json
{"resource":{"kind":"deck"}}
{"resource":{"kind":"outline"}}
{"resource":{"kind":"design"}}
{"resource":{"kind":"slide","slide_id":"sli_01...","part":"spec"}}
{"resource":{"kind":"slide","slide_id":"sli_01...","part":"html"}}
```

资源定位仍是结构化对象，不接受路径、`current`、ordinal 或展示 key。

### 9.2 `render_slide`

保留 `render_slide(slide_id)`：

- 读取当前 Run staging 中的候选 HTML。
- 使用当前 deck/outline/design 构造 Runtime frame。
- 在隔离 Chromium 中截图并返回 overflow、clipping、console、font、resource 诊断。
- 产生绑定当前 HTML body source 和 frame context 的 proof。
- 不隐式修改 HTML。

## 10. 后端领域架构

### 10.1 共享命令服务

新增统一的 PPT mutation domain service，Agent 工具和 HTTP API 都调用它：

```text
PPTMutationService
  ApplyDeckPatch
  InitOutline
  InsertOutlineNode
  MoveOutlineNode
  UpdateOutlineNode
  RemoveOutlineNode
  WriteDesign
  PatchDesign
  WriteSlideSpec
  PatchSlideSpec
  WriteSlideHTML
  PatchSlideHTML
```

适配器差异：

- Agent adapter 写入 RunSession staging，并在 finish 后统一 Commit。
- HTTP/UI adapter 直接执行单次原子提交；活跃 Run 时继续返回 409 `RUN_ACTIVE`。

不能保留两套独立结构规则。

### 10.2 Outline 树工具函数

建立单一权威库：

```text
ValidateOutlineTree
FlattenOutline
FindNode
ResolveSlideOrdinal
InsertNode
MoveNode
UpdateNode
RemoveNode
SemanticSlideNodeHash
```

Context Engine、HTTP service、workflow、frontend contract tests 共享同一行为定义。

### 10.3 Commit

Commit 必须支持 outline 中已声明页面和 spec/HTML 的分阶段 staging：

- staging 期间允许 pending。
- Completion Gate 负责判断本次目标是否允许 pending。
- 成功 finish 时，完整 PPT 生成必须无 pending。
- Commit 原子写 deck、outline、design、spec、HTML、materialization、版本快照和 slide identity rows。
- 不再遍历 `outline.SlideOrder`；统一遍历 `FlattenOutline`。
- DB `ApplyPPTMutation` 删除，不能保留 no-op 权威假象。

### 10.4 Context Engine

Context Pack 增加 deck 资源，并从 outline tree 产生：

- stable ordered slide IDs。
- ordinal projection。
- section/subsection ancestry。
- slide node label/role。

Slide Summary 不再从 spec 读取 placement。页面邻居、同 subsection 页面和工作集都基于 flatten 结果和树祖先查询。

### 10.5 Completion Gate

Required Action 全部改为 `mutate_ppt` + concrete op：

```text
缺失 spec       -> slide.spec.write
spec 不合法     -> slide.spec.write / slide.spec.patch
缺失 HTML       -> slide.html.write
HTML 诊断失败   -> slide.html.patch / slide.html.write
结构非法        -> outline.init / insert / move / update / remove
design 缺失     -> design.write
```

Completion Gate 不再建议旧 `write_ppt/edit_ppt`。

### 10.6 数据库

删除任何页面顺序写入接口和测试假设。Slide table 保留：

```text
id
project_id
current_version
last_export_at
```

如 API 列表需要 ordinal，由 service 从当前 outline 快照派生，不持久化。

## 11. HTTP API

### 11.1 Canonical project content

统一返回一个自洽快照：

```ts
type ProjectContentSnapshot = {
  deck: Deck
  outline: Outline
  design: Design
  slides_by_id: Record<SlideId, {
    spec_state: "pending" | "ready"
    spec: SlideSpec | null
    html_state: MaterializationState
    html_revision: number
    materialization: Materialization | null
  }>
}
```

不返回另一个带权威顺序的 `slides[]`。前端从 outline flatten 派生 ordered slides。

### 11.2 Mutation endpoint

前端结构操作统一走一个 mutation endpoint，例如：

```text
POST /api/v1/projects/:id/mutations
```

请求使用与 `mutate_ppt` 相同的 operation contract，并额外要求 `expected_revision`/If-Match。响应返回 canonical project content snapshot。

删除旧的分裂接口：

- `/mutations`
- `/mutations`
- 单独的 section/subsection add/rename/remove 路由

内部仍可保留 handler helper，但对外不维护两套协议。

### 11.3 Pending spec

单页读取：

- 已声明但 spec 缺失：200 + `spec_state=pending`。
- slide ID 不在 outline/identity registry：404。
- 提交的 spec 不符合 Schema：422。
- revision 冲突：409。

## 12. Prompt 工程

### 12.1 资源契约

Prompt resource contract 更新为：

- Deck owns presentation intent, audience, language, requirements, prohibitions and base settings.
- Outline owns the strict two-level tree and only slide order.
- Outline slide node owns stable slide identity reference, directory label and role.
- Slide Spec owns title, key message, ordered element intents and layout direction.
- Design owns global visual system and chrome style.
- Slide HTML is the Agent-authored page implementation; Runtime owns page-number frame.

Runtime 管理字段继续从 Agent contract 中剥离。

### 12.2 Empty deck playbook

新顺序：

1. 读取 deck，确认目标、受众、语言、要求和页数范围。
2. 调用 `outline.init`，只提交 `client_ref`，不得生成正式 ID。
3. 使用返回映射读取/确认正式结构。
4. 调用 `design.write` 建立全局视觉系统。
5. 按 flatten 后 ordinal 逐页调用 `slide.spec.write`。
6. 按相同 ID 调用 `slide.html.write`。
7. 每页调用 `render_slide`，根据诊断使用 `slide.html.patch/write` 修复。
8. 通过 Completion Gate 后 finish。

### 12.3 Coordinated edit playbook

- 用户说“第 N 页”时，先从当前 outline revision 解析为 stable slide ID。
- 后续所有操作只使用 ID。
- 页面 reorder 使用 `outline.move`，不能改 spec placement。
- section/subsection 变化使用 outline operation，不整体重写 outline。
- spec 变化必须更新对应 HTML 并重新 render。
- 纯页面顺序变化不要求重写 HTML。
- deck/design 的影响范围由 mutation result 的 invalidated targets 决定。

### 12.4 HTML Prompt 约束

- 禁止在页面 HTML 中写死页码、总页数和当前 section 编号。
- 禁止使用 `sli-*`、文件路径或 ordinal 作为用户可见标题。
- HTML 只实现页面主体；Runtime frame 负责编号和共享 chrome。
- render/repair 仍在同一连续 ReAct Loop 中完成。

### 12.5 Prompt 模块和工具披露

同步修改：

- runtime core 中的工具名隔离规则。
- mode policy/tool disclosure。
- empty deck/deck edit/slide edit playbook。
- resource contracts。
- completion repair guide。
- quality rubric 中的 page-number/HTML 规则。
- Context Briefing 和 working set 的资源摘要。
- 所有 Provider 继续共用同一业务 Prompt，不复制供应商版本。

## 13. 前端架构

### 13.1 Store

`projectStore` 保存每项目唯一 `ProjectContentSnapshot`：

```text
contentByProjectId[projectId]
  deck
  outline
  design
  slides_by_id
```

不再分别保存：

- `slidesByProjectId` 的权威顺序。
- `specByProjectId` 的另一份 outline。
- placement map。

所有快照更新必须一次 Zustand set 原子发布。

`deckStore` 继续只保存：

- `currentSlideId`。
- preview mode。
- global view。

不得恢复 `currentPage`。

### 13.2 Selectors

建立纯函数 selector：

```text
flattenOutline(outline)
directorySections(outline, slidesById)
ordinalBySlideId(outline)
selectedSlide(snapshot, currentSlideId)
adjacentSlideIds(outline, currentSlideId)
```

目录、缩略图、上一页/下一页、overview、页码和活动文案使用同一 selector 语义。

### 13.3 URL

URL 继续保存 stable slide ID：

```text
?slide=sli_01...
```

页面移动不改 URL。删除当前页时选择 flatten 结果中的确定性相邻页。

### 13.4 目录结构操作

DeckNavigator 不再在前端构造完整 `ordered_ids + placements`：

- 拖拽/上下移动发 `outline.move`。
- 新增 section/subsection/slide 发 `outline.insert`。
- 重命名发 `outline.update`。
- 删除发 `outline.remove`。
- 请求期间禁用其他结构操作。
- 成功后直接原子应用后端 canonical snapshot，不额外并发刷新。
- 活跃 Run 期间继续禁止人工结构编辑。

### 13.5 Pending 页面

outline 初始化完成后，目录立即展示所有页面 skeleton：

- `pending spec`：显示“等待生成设计稿/正在生成”。
- spec ready、HTML missing：设计稿可查看，幻灯片视图显示“页面未物化”。
- HTML fresh：显示缩略图和画布。
- render error：保留上一版可用 HTML，并显示本次运行错误状态。

### 13.6 Runtime frame 与页码

`IsolatedSlidePreview`、缩略图和导出使用同一 frame builder：

```text
stored HTML body
+ current ordinal/total
+ deck numbering policy
+ design chrome
= final presented slide
```

移动页面后只更新 frame props，不重新请求或重写 HTML。

## 14. 运行反馈与文案

### 14.1 审批后反馈

- approve：`正在启动执行`。
- revise：`正在调整计划`。
- cancel：`正在停止任务`。
- 进入具体 mutation 后由具体 operation 覆盖通用文案。

### 14.2 单页反馈

使用 Runtime 从 active outline overlay 解析的 ordinal：

```text
正在创建第 3 页设计稿
已创建第 3 页设计稿

正在更新第 3 页设计稿
已更新第 3 页设计稿

正在创建第 3 页幻灯片
已创建第 3 页幻灯片
```

不追加页面标题：

```text
✅ 已创建第 3 页设计稿
❌ 已创建第 3 页设计稿 · 什么是好 SKILL
```

### 14.3 批量折叠

- 同类：展示数量和具体对象，例如 `已创建 10 个页面设计稿`、`已创建 10 张幻灯片`。
- 不同类：才使用 `已完成 10 项`。
- 不向用户展示 `sli-*`、工具名、resource key 或本地机器字段作为主文案。

## 15. 失效与重新渲染规则

| 变化 | Spec | HTML body | Runtime frame |
|---|---|---|---|
| 纯页面 reorder | 不变 | 不变 | 立即重算 |
| 移动 section/subsection 子树 | 不变 | 不变 | 立即重算 |
| 页面跨 section/subsection | 不变 | 默认不变 | 立即重算 ancestry/chrome |
| outline slide label/role 更新 | 视语义影响 | 可能需更新 | 立即重算 |
| spec 更新 | 当前页更新 | 当前页 stale | 当前 frame 可复用 |
| design 更新 | 不变 | 受影响页 stale | 立即重算 chrome |
| deck 目标/受众/语言更新 | 由 Agent 判断受影响页 | 相应页面 stale | 设置变化时重算 |
| numbering 更新 | 不变 | 不变 | 立即重算 |

Mutation result 必须返回 `invalidated_slide_ids` 和原因，前端只展示状态，Agent 根据 Completion Gate 修复。

## 16. 删除的旧概念

一次性删除，不保留双读/双写：

- `outline.outline_order`。
- `slide.spec.section/subsection`。
- `slide.spec.role`。
- DB/page API 中作为权威字段的 position/order。
- `ApplyPPTMutation`。
- Agent 工具 `write_ppt/edit_ppt`。
- 旧 Resource 的 `deck:outline/deck:design` 输入契约。
- 旧 reorder/restructure/section/subsection 分裂写接口。
- Prompt 中旧工具名和旧资源职责。
- 前端 `slides[]` 作为独立权威顺序。

旧开发数据可以清空或由项目启动脚本重新生成；不要实现迁移器。

## 17. 分阶段实施与提交建议

允许分阶段多次提交，但每阶段结束应保持当前阶段约定下的测试可运行。不要在代码中保留长期兼容桥。

### Phase 1：持久化与领域模型

建议 Commit：

```text
refactor: introduce deck and tree-owned outline

- Add deck.json as the presentation-level authoring resource.
- Make the outline tree the only owner of hierarchy and slide order.
- Remove placement ownership from slide specs and derive ordinals from the tree.
```

范围：

- 新 deck/outline/spec/design/materialization Schema。
- Go/TypeScript 类型。
- Project 初始化。
- Outline flatten/validate/node hash。
- Context Engine loader/assembler。
- 文件读取和版本目标。
- 删除旧迁移兼容代码或直接提升 layout version 并重建开发数据。

### Phase 2：统一 Mutation 领域服务和 API

建议 Commit：

```text
refactor: centralize PPT mutations behind typed commands

- Add typed deck, outline, design, spec, and HTML mutation commands.
- Reuse one validation and persistence path for Agent and HTTP adapters.
- Replace reorder and placement APIs with canonical mutation snapshots.
```

范围：

- `PPTMutationService`。
- outline init/insert/move/update/remove。
- Runtime ID 分配和 provisional identities。
- canonical snapshot API。
- pending spec 200 语义。
- 删除旧 structure endpoints/service paths。

### Phase 3：Agent 工具和 Prompt

建议 Commit：

```text
refactor: replace PPT write tools with mutate_ppt

- Expose a scoped discriminated mutation schema to execute runs.
- Remove write_ppt and edit_ppt from runtime, prompts, events, and repair guidance.
- Update empty-deck generation to initialize structure before page authoring.
```

范围：

- `mutate_ppt` Tool Schema/Execute。
- 受限 JSON Patch、HTML exact patch。
- tool disclosure/scope/idempotency/batching。
- Prompt modules/playbooks/contracts。
- Completion Gate/required actions。
- Public events/tool enum/error mapping。

### Phase 4：前端快照和目录

建议 Commit：

```text
refactor: derive workspace navigation from canonical outline snapshots

- Store one atomic project-content snapshot and derive every page ordinal from the outline.
- Send typed outline mutations for directory editing and apply canonical responses directly.
- Render pending pages and active-run overlay progress without intermediate 422 errors.
```

范围：

- API types/client。
- `projectStore` 归一化。
- DeckNavigator selectors/drag/move/add/remove/rename。
- pending skeleton。
- active Run overlay refresh。
- 稳定 `currentSlideId` 和 URL 回归测试。

### Phase 5：HTML frame、物化和收尾

建议 Commit：

```text
refactor: decouple runtime page numbering from slide HTML

- Inject page numbering and shared chrome from the current outline and deck settings.
- Separate HTML body freshness from deterministic frame context.
- Align preview, thumbnails, render evidence, export, and user-facing activity labels.
```

范围：

- Runtime frame builder。
- Preview/thumbnail/export 共用。
- Materialization/proof/source hash。
- public copy 和同类批量聚合。
- 删除所有旧字段、旧测试 fixture、旧文档引用。

## 18. 测试要求

### 18.1 Schema/领域测试

- deck 正负 Schema。
- outline direct/grouped 正负 Schema。
- flatten 顺序和 ordinal。
- ID 重复、跨项目、非法父子和循环移动。
- insert/move/update/remove 全部边界。
- pending slide 合法，最终完成时 pending 被 Gate 拒绝。
- spec 不再接受 section/subsection/role。
- patch path 白名单和原子回滚。

### 18.2 Workflow 测试

- empty project -> outline.init -> ID mapping -> spec/html -> render -> commit。
- AI 不能提交正式新 ID。
- scope 裁剪后的 `mutate_ppt` op enum。
- 多 Tool Calls 的串行/并行规则。
- Completion repair 返回新 op。
- Run 取消不提交 provisional identities。
- Run overlay 始终返回自洽 pending 快照。

### 18.3 Materialization/Render 测试

- spec 变化使当前页 HTML stale。
- design 变化使受影响页面 stale。
- 纯 reorder 不改变 HTML revision/body freshness。
- reorder 后 frame 页码立即变化。
- 封面隐藏页码但第二页仍显示 2。
- render proof 拒绝 stale body 或 stale frame context。

### 18.4 前端测试

- outline flatten 是唯一页面顺序。
- move section/subsection 时内部页面整体跟随。
- move selected slide 时 selection/URL 保持 ID。
- pending 页面目录和画布状态。
- mutation response 原子更新，无双请求竞态。
- 活跃 Run overlay 进度。
- 单页活动文案无标题后缀。
- 同类批量文案使用具体对象，不同类才使用“项”。

### 18.5 最终验证命令

```bash
cd backend && go test ./...
cd frontend && npm test
cd frontend && npm run lint
cd frontend && npm run tsc
cd frontend && npm run build
```

还需运行仓库现有的 render worker/integration tests；如测试依赖本机 Chromium，应记录实际执行结果和未运行原因。

## 19. 验收标准

1. 新项目创建后没有 slide ID，仍能完整读取 deck/outline/design 和空快照。
2. `outline.init` 原子创建结构并返回所有正式 ID；Agent 不再生成 `sli-*`。
3. spec 缺失的已声明页面返回 pending 200，不出现中间态 422。
4. outline tree 是唯一顺序来源，代码库不存在 `outline_order` 和 spec placement 双写。
5. 移动 page/section/subsection 只修改 outline，并返回 canonical snapshot。
6. 工作区 ordinal、目录页码和画面页码一致。
7. 页码不写入 Agent HTML；重排不重写 HTML。
8. `write_ppt/edit_ppt` 从工具、Prompt、Gate、事件、测试中完全删除。
9. `mutate_ppt` 只接受 12 个封闭 op，并按 scope 裁剪。
10. `deck.patch/design.patch/slide.spec.patch` 只支持 add/remove/replace。
11. `slide.html.patch` 保留唯一锚点 exact replacement。
12. 前端以单一原子项目快照为源，稳定 `currentSlideId` 不回退为 index。
13. 运行反馈显示真实状态；单页完成文案为“已创建第 N 页设计稿”，不追加标题。
14. 同类批量展示具体对象，不同类才使用“项”。
15. 后端、前端、Prompt snapshot、Schema、Render 和构建测试全部通过。

## 20. 给执行 Agent 的启动 Prompt

```text
请在 /Users/bytedance/Desktop/ByteDance/PPT_Agent 中完整实施：
docs/discuss/2026-08-26-deck-outline-mutate-ppt-architecture-design.md

这是一次 breaking refactor。严格遵循仓库 AGENTS.md；不要为旧 Schema、旧工具、旧接口或旧开发数据保留兼容层、迁移层、双读或双写。先审计当前代码与最新文档，再按设计贯通后端领域模型、deck/outline/spec/design/materialization 文件、mutate_ppt 工具、Prompt/Context/Completion、HTTP API、前端 store/目录/预览/运行反馈和测试。

可以按设计文档 Phase 1-5 分阶段实施并多次提交，每个提交使用规范 commit message，避免混入无关改动。实现过程中以“稳定 ID、outline 树唯一顺序、Runtime 页码、单一前端快照、Agent 不生成正式 ID”为不可破坏的不变量。不要只修改类型或表面文案，必须删除旧 write_ppt/edit_ppt、outline_order、spec placement 和旧结构 API 的所有生产/测试/Prompt 引用。

完成后运行全部 Go 测试、前端 test/lint/tsc/build 以及可运行的 render integration tests。最终回复只输出改动总结：分阶段提交列表、核心架构变化、前后端与 Prompt 变化、测试结果，以及任何明确偏离设计的地方；不要输出过程日志。
```
