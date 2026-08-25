# PPT Resource、字符串工具与 Spec 命名重构设计

> 日期：2026-08-02
>
> 状态：设计确认稿
>
> 范围：PPT 领域资源模型、`read_ppt/write_ppt/edit_ppt` 参数、文件与字段命名、
> 多工具调用、渲染 Worker 和业务 System Prompt
>
> 文档性质：后续实现与验收的权威规格，不包含兼容性双写方案
>
> 设计来源：基于产品经理交接基线及 2026-08-02 三份权威设计稿，经后续产品讨论确认

## 1. 决策摘要

本设计将模型可见的 PPT 操作界面进一步收敛为接近 General Agent 文件工具的形式：

```text
read_ppt(resource)
write_ppt(resource, content: string)
edit_ppt(resource, edits: text replacements)
```

模型只能寻址四类受控 PPT 资源：

```text
deck:outline
deck:design
slide:<slide_id>:spec
slide:<slide_id>:html
```

关键决定：

1. 模型工具层删除 `global`，只保留 `deck | slide`。
2. 公开业务术语和代码字段将 `blueprint` 改为 `spec`。
3. 文件重命名为 `outline.json`、`design.json`、`spec.json` 和 `index.html`。
4. 三个 PPT 读写工具不再暴露 `model`、`include`、文件路径、staging 或存储 Artifact Kind。
5. `read_ppt` 向 Agent 返回目标资源的原始内容字符串。
6. `write_ppt.content` 对所有资源统一为字符串。
7. `edit_ppt` 对所有资源统一使用唯一锚点文本替换。
8. Tool Schema 只约束资源寻址和操作外壳；Outline、Design、Slide Spec 的字段由独立领域
   Schema 校验。
9. Prompt Compiler 从同一份领域 Schema 生成给 Agent 的资源契约，避免 Prompt 与 Runtime 漂移。
10. Provider 和 Runtime 支持一次模型响应携带多个 Tool Calls；写操作按序执行，独立只读与渲染
    可限流并发。
11. Render Worker 改为常驻服务并复用 Browser 进程，不再每页冷启动 Chromium。
12. 仍然保持单一连续 ReAct Loop、动态 Plan、显式 `finish`、Completion Gate 和 11 种公共事件。

## 2. 与既有权威设计的关系

本设计只替换以下既有决定：

| 旧设计 | 新设计 |
| --- | --- |
| `target.artifact = blueprint \| presentation` | `target.artifact = spec \| presentation` |
| 工具资源 `global \| slide` | 工具资源 `deck:outline / deck:design / slide:*:spec / slide:*:html` |
| `deck.json` | `outline.json` |
| `design/design-spec.json` | `design.json` |
| `slides/{id}/slide.json` | `slides/{id}/spec.json` |
| `content.model` | 删除，统一为 `content: string` |
| `read_ppt.include = model/html` | 删除，资源地址已经唯一确定读取内容 |
| JSON Patch 与 HTML Text Edit 两种模型可见编辑 | 统一为精确文本替换 |
| 单个 `ToolCall` Provider/Runtime 契约 | `ToolCalls[]` |

以下既有决定保持不变：

- 产品是 HTML PPT 业务 Agent，不是通用 Agent。
- 公开任务仍由 `artifact + level + interaction` 三个正交维度组成。
- `chat/simple/complex` 是 Runtime strategy。
- 所有 strategy 使用一个连续 ReAct Loop；Complex 只额外拥有动态 Plan。
- Plan 不是 Workflow DAG。
- 所有正常成功退出必须显式调用 `finish` 并通过 Completion Gate。
- 模型业务工具仍只有 `read_ppt/write_ppt/edit_ppt/search_refs/render_slide`。
- Runtime 控制动作仍只有 `update_plan/ask_user/finish`。
- 公共前端事件仍只有 11 种。
- staging、evidence、context、strategy、phase 和 trace 默认不暴露给普通用户。
- Repo 资产写入、多 Agent、通用 Shell、打字机流和复杂智能 Verifier 仍不属于 MVP。

本设计经确认后，应同步更新以下三份 2026-08-02 设计稿中的对应章节，不能保留相互矛盾的权威定义：

- `2026-08-02-adaptive-react-execution-strategy-design.md`
- `2026-08-02-ppt-agent-tool-surface-design.md`
- `2026-08-02-agent-public-events-and-timeline-design.md`

## 3. 产品领域模型

### 3.1 Run 公开目标

新的公开 Run 目标为：

```text
target.artifact = spec | presentation
target.level    = deck | slide
interaction     = talk | ask | execute
```

四种组合：

| artifact | level | 用户语义 |
| --- | --- | --- |
| `spec` | `deck` | 整份 PPT 的 Outline、Design 和各页 Spec |
| `spec` | `slide` | 指定页面的 Slide Spec |
| `presentation` | `deck` | 整份 Spec 及所有页面 HTML |
| `presentation` | `slide` | 指定页面的 Spec 和 HTML |

`spec` 的产品定义是：

> 可指导 HTML Presentation 生成的结构化内容与设计规格。

普通用户界面可以继续显示“设计稿”，不需要展示英文 `spec`。

### 3.2 模型可寻址资源

模型不直接操作 Run 的 `artifact`，也不直接操作磁盘文件。Runtime 根据当前 WorkSpec、interaction、
strategy、phase 和 scope 决定哪些资源可以读写。

资源联合：

```text
Deck Resource
├── outline
└── design

Slide Resource
├── spec
└── html
```

规范地址：

```json
{
  "type": "deck",
  "part": "outline"
}
```

```json
{
  "type": "deck",
  "part": "design"
}
```

```json
{
  "type": "slide",
  "slide_id": "slide-03",
  "part": "spec"
}
```

```json
{
  "type": "slide",
  "slide_id": "slide-03",
  "part": "html"
}
```

唯一资源键：

```text
deck:outline
deck:design
slide:slide-03:spec
slide:slide-03:html
```

资源键用于 scope、staging、revision、evidence、并发冲突、Trace 和恢复。模型不能传入项目路径、
绝对路径、相对路径或数据库 ID。

### 3.3 Deck 与 Slide 的边界

`deck` 是整份 PPT 的全局资源层，但不是包含所有页面正文和 HTML 的巨型文档。

- `deck:outline` 保存整份目标、受众、核心命题、叙事、章节和 `slide_order`。
- `deck:design` 保存整份画布、色彩、字体、间距、布局、签名视觉和动效规范。
- `slide:*:spec` 保存一页的语义内容与视觉意图。
- `slide:*:html` 保存一页的最终 HTML。

一次 `deck-level` Run 可以操作多个 Deck/Slide Resource；一次 Deck Resource 写入不能承载所有页面
HTML。

## 4. 文件布局与命名

目标项目布局：

```text
<project>/
├── outline.json
├── design.json
├── common/
│   ├── tokens.css
│   └── base.css
└── slides/
    ├── slide-01/
    │   ├── spec.json
    │   └── index.html
    └── slide-02/
        ├── spec.json
        └── index.html
```

重命名：

| 旧路径 | 新路径 |
| --- | --- |
| `deck.json` | `outline.json` |
| `design/design-spec.json` | `design.json` |
| `slides/{slide_id}/slide.json` | `slides/{slide_id}/spec.json` |
| `slides/{slide_id}/index.html` | 保持不变 |

内部类型与存储 Target 建议同步改名：

| 旧名称 | 新名称 |
| --- | --- |
| `ArtifactDeck` / `blueprint_deck` | `ArtifactOutline` / `outline` |
| `ArtifactDesign` / `design_spec` | `ArtifactDesign` / `design` |
| `ArtifactSlide` / `blueprint_slide` | `ArtifactSlideSpec` / `slide_spec` |
| `ArtifactPresentation` / `presentation_slide` | `ArtifactSlideHTML` / `slide_html` |
| `BlueprintRevision` | `SpecRevision` |
| `SourceBlueprintRevision` | `SourceSpecRevision` |
| `SlideBlueprintCard` | `SlideSpecCard` |
| `blueprintStore` | `specStore` |

禁止只修改 UI 文案而继续在新代码中传播 `blueprint/global/model` 旧名称。

### 4.1 数据迁移

实现需要提供一次性、原子的数据迁移：

1. 识别旧项目布局版本。
2. 在同一项目目录内完成文件重命名。
3. 更新数据库中的路径、Target Type 和 revision 字段。
4. 更新版本快照路径与引用。
5. 迁移成功后标记新的布局版本。
6. 迁移失败时回滚本次迁移，不留下新旧文件混合状态。

目标 Runtime 只读写新路径；不保留长期双读、双写、别名工具或旧字段兼容分支。

## 5. 统一字符串工具原则

三个工具模拟 General Agent 的文件读写心智模型：

```text
read_file(path)
write_file(path, content)
edit_file(path, old_text, new_text)
```

PPT 版本为：

```text
read_ppt(resource)
write_ppt(resource, content)
edit_ppt(resource, edits)
```

差异是：`resource` 是受控的 PPT 领域地址，不是任意文件路径。

所有资源的模型可见内容统一为字符串：

| Resource | String 内容 |
| --- | --- |
| `deck:outline` | JSON 文本 |
| `deck:design` | JSON 文本 |
| `slide:*:spec` | JSON 文本 |
| `slide:*:html` | HTML 文本 |

外层 Function Calling 参数仍然是 JSON Object；`write_ppt.content` 是该 Object 中的 String 字段。

## 6. 共享 Resource Schema

规范 JSON Schema：

```json
{
  "oneOf": [
    {
      "type": "object",
      "required": ["type", "part"],
      "additionalProperties": false,
      "properties": {
        "type": {
          "const": "deck"
        },
        "part": {
          "type": "string",
          "enum": ["outline", "design"]
        }
      }
    },
    {
      "type": "object",
      "required": ["type", "slide_id", "part"],
      "additionalProperties": false,
      "properties": {
        "type": {
          "const": "slide"
        },
        "slide_id": {
          "type": "string",
          "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$"
        },
        "part": {
          "type": "string",
          "enum": ["spec", "html"]
        }
      }
    }
  ]
}
```

Provider Tool Schema 中应内联该结构，避免依赖 Provider 对 `$ref/$defs` 的兼容程度。

## 7. `read_ppt`

### 7.1 参数

```json
{
  "name": "read_ppt",
  "description": "Read the latest authorized PPT resource as its raw source text, preferring this run's staged version.",
  "parameters": {
    "type": "object",
    "required": ["resource"],
    "additionalProperties": false,
    "properties": {
      "resource": {
        "oneOf": [
          {
            "type": "object",
            "required": ["type", "part"],
            "additionalProperties": false,
            "properties": {
              "type": {
                "const": "deck"
              },
              "part": {
                "type": "string",
                "enum": ["outline", "design"]
              }
            }
          },
          {
            "type": "object",
            "required": ["type", "slide_id", "part"],
            "additionalProperties": false,
            "properties": {
              "type": {
                "const": "slide"
              },
              "slide_id": {
                "type": "string",
                "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$"
              },
              "part": {
                "type": "string",
                "enum": ["spec", "html"]
              }
            }
          }
        ]
      }
    }
  }
}
```

### 7.2 返回

成功时，Agent Observation 的主要内容直接是资源原始字符串：

- JSON 资源返回当前 staged/committed 文件中实际保存的 JSON 文本。
- HTML 资源返回当前 staged/committed 文件中实际保存的 HTML 文本。
- 不附加本地路径、staging 路径、hash、完整 Manifest 或 Runtime phase。

Runtime 内部的 `DomainToolResult` 仍保留：

- `ok`；
- resource key；
- source view；
- revision/hash；
- issues；
- token/context budget 信息。

这些内部字段用于 Trace、Evidence 和恢复，不等于模型必须看到的正文。

读取优先级：

```text
当前 Run staged resource
→ committed resource
→ TARGET_NOT_FOUND
```

MVP 继续执行资源大小上限。为保证后续精确编辑基于完整正文，`read_ppt` 不返回伪装成完整内容的
截断字符串；资源超过模型可读上限时直接返回 `CONTENT_TOO_LARGE`，而不是允许读取任意路径。

## 8. `write_ppt`

### 8.1 参数

```json
{
  "name": "write_ppt",
  "description": "Create or fully replace one authorized PPT resource with raw JSON or HTML source text in the run staging transaction.",
  "parameters": {
    "type": "object",
    "required": ["resource", "content"],
    "additionalProperties": false,
    "properties": {
      "resource": {
        "oneOf": [
          {
            "type": "object",
            "required": ["type", "part"],
            "additionalProperties": false,
            "properties": {
              "type": {
                "const": "deck"
              },
              "part": {
                "type": "string",
                "enum": ["outline", "design"]
              }
            }
          },
          {
            "type": "object",
            "required": ["type", "slide_id", "part"],
            "additionalProperties": false,
            "properties": {
              "type": {
                "const": "slide"
              },
              "slide_id": {
                "type": "string",
                "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$"
              },
              "part": {
                "type": "string",
                "enum": ["spec", "html"]
              }
            }
          }
        ]
      },
      "content": {
        "type": "string",
        "minLength": 1,
        "maxLength": 2097152
      }
    }
  }
}
```

### 8.2 语义

- `write_ppt` 完整创建或替换一个资源。
- Agent 不提交 revision、schema version、project ID、timestamps、staging 路径或 commit 信息。
- Runtime 根据 resource 解析字符串、注入内部字段、校验、规范化并写入 staging。
- JSON 资源成功写入后，staged view 保存规范化 JSON。
- HTML 保存为原始文本，但必须通过 HTML Contract。
- 大范围重建使用 `write_ppt`，不使用大量脆弱的 `edit_ppt` 替换。

### 8.3 Runtime 处理

```text
validate tool disclosure / interaction / strategy / phase / scope
        ↓
resolve resource key
        ↓
JSON resource → parse JSON string
HTML resource → parse HTML string
        ↓
inject or preserve Runtime-managed metadata
        ↓
domain validation
        ↓
stage normalized candidate
        ↓
invalidate affected evidence
        ↓
return concise observation
```

禁止 Agent 修改的内部字段包括：

```text
schema_version
revision
project_id
slide_id
created_at
updated_at
source revisions
materialization state
```

## 9. `edit_ppt`

### 9.1 参数

```json
{
  "name": "edit_ppt",
  "description": "Atomically apply one or more uniquely anchored text replacements to an authorized PPT resource.",
  "parameters": {
    "type": "object",
    "required": ["resource", "edits"],
    "additionalProperties": false,
    "properties": {
      "resource": {
        "oneOf": [
          {
            "type": "object",
            "required": ["type", "part"],
            "additionalProperties": false,
            "properties": {
              "type": {
                "const": "deck"
              },
              "part": {
                "type": "string",
                "enum": ["outline", "design"]
              }
            }
          },
          {
            "type": "object",
            "required": ["type", "slide_id", "part"],
            "additionalProperties": false,
            "properties": {
              "type": {
                "const": "slide"
              },
              "slide_id": {
                "type": "string",
                "pattern": "^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$"
              },
              "part": {
                "type": "string",
                "enum": ["spec", "html"]
              }
            }
          }
        ]
      },
      "edits": {
        "type": "array",
        "minItems": 1,
        "maxItems": 32,
        "items": {
          "type": "object",
          "required": ["old_text", "new_text"],
          "additionalProperties": false,
          "properties": {
            "old_text": {
              "type": "string",
              "minLength": 1
            },
            "new_text": {
              "type": "string"
            }
          }
        }
      }
    }
  }
}
```

### 9.2 语义

- 每个 `old_text` 必须在当前候选字符串中唯一匹配。
- 多个 edits 按参数顺序应用到同一个内存候选。
- 任一 edit 匹配零次或多次时，整个调用失败，候选不进入 staging。
- 全部替换完成后，对最终完整资源重新解析和校验。
- JSON 资源也使用文本编辑，不向模型披露 JSON Patch 分支。
- Agent 应先 `read_ppt` 获得精确内容，再构造唯一锚点。

稳定错误：

```text
EDIT_ANCHOR_NOT_FOUND
EDIT_ANCHOR_AMBIGUOUS
CONTENT_INVALID
TARGET_OUT_OF_SCOPE
REVISION_CONFLICT
```

## 10. 领域 Schema 与校验

Tool Schema 不展开 Outline、Design 和 Slide Spec 的业务字段。字段契约由独立、版本化的领域
Schema 管理：

```text
schemas/outline.schema.json
schemas/design.schema.json
schemas/slide-spec.schema.json
```

校验路由：

```text
deck:outline    → JSON parse → Outline Schema → reference validation
deck:design     → JSON parse → Design Schema
slide:*:spec    → JSON parse → Slide Spec Schema → outline reference validation
slide:*:html    → HTML parse → deterministic lint
```

### 10.1 字段策略

领域 Schema 固定：

- 字段名；
- 数据类型；
- 必填字段；
- 稳定枚举；
- 数组和文本上限；
- ID 和引用关系；
- 禁止 Agent 写入的 Runtime 字段。

领域 Schema 不固定：

- 实际标题和文案；
- 章节和页面数量；
- 核心句表达；
- 视觉描述；
- 配色值和字体选择；
- 页面 HTML 结构。

已被产品和消费者理解的 JSON Object 默认使用：

```json
{
  "additionalProperties": false
}
```

Agent 不能通过发明无人消费的字段来绕过产品模型。新增正式字段必须升级对应领域 Schema、Prompt 和
消费者。

### 10.2 三层校验

```text
格式层
JSON / HTML 是否可解析
        ↓
领域层
字段、类型、必填、枚举和上限
        ↓
项目一致性层
outline 引用、section 归属、slide_id、spec/html 完整性
```

视觉 overflow、clipping、资源失败、字体和 console error 不进入 JSON Schema，由 `render_slide`
处理。

### 10.3 Prompt 与校验器共源

领域 Schema 是唯一事实来源：

```text
领域 Schema
├── Runtime Validator：权威校验
└── Prompt Compiler：生成紧凑资源契约和合法示例
```

禁止人工维护一份与 Runtime Schema 独立的 Prompt 字段清单。

## 11. 权限与 Scope

### 11.1 Interaction

```text
talk / ask → 所有写入拒绝
execute    → 在当前 WorkSpec scope 内写入
```

### 11.2 Artifact

```text
artifact=spec
→ 可写 outline / design / slide spec
→ 禁止写 slide html

artifact=presentation
→ 可写完成 Presentation 所需的 outline / design / slide spec / slide html
```

### 11.3 Level

```text
level=deck
→ 可写 deck resources
→ 可写该 Deck scope 内的 slide resources

level=slide
→ 只可写指定 slide_id 的 spec/html
→ deck resources 只读
```

所有工具在实际执行时必须再次校验，不依赖 Provider 已收到的 Schema。

## 12. Evidence 与 Completion Gate

Evidence 绑定唯一资源键和 source hash。

最低完成要求：

| ChangeSet | Completion 要求 |
| --- | --- |
| `deck:outline` 改变 | Outline Schema + 引用完整性 |
| `deck:design` 改变 | Design Schema；使受影响 Slide render evidence stale |
| `slide:*:spec` 改变 | Slide Spec Schema + Outline 引用；按影响使 HTML/render stale |
| `slide:*:html` 改变 | HTML lint + 最新 source hash 的 render evidence |
| 新建 Presentation Slide | Spec + HTML + lint + render |
| 整份 Presentation | Outline、Design、全部声明 Slide 的 Spec/HTML 和最新渲染证据 |

Completion Gate 拒绝继续返回同一个 ReAct Loop，不引入独立 Verify/Repair Stage。

## 13. 多 Tool Calls 与并发

### 13.1 Provider 契约

Provider 层将单数：

```text
ToolCall *ToolCall
```

改为：

```text
ToolCalls []ToolCall
```

必须完整解析和保留 Provider 返回的全部调用，禁止只取第一项或静默丢弃其余调用。

### 13.2 Runtime 执行策略

模型仍在同一个 ReAct turn 中返回一组 Tool Calls。Runtime 按以下规则执行：

| 调用集合 | 执行方式 |
| --- | --- |
| 独立 `read_ppt/search_refs` | 限流并发 |
| 不同 Slide 的 `render_slide` | 限流并发 |
| `write_ppt/edit_ppt` | 按模型返回顺序串行 |
| 同一资源键的任何调用 | 串行 |
| 包含写入与后续依赖读取/渲染 | 按原顺序串行 |
| `update_plan` | 单独执行 |
| `ask_user` | 必须是该响应唯一控制动作 |
| `finish` | 必须是该响应唯一完成动作 |

写调用批次 fail-fast：一个写/编辑失败后，不执行后续可能依赖该结果的调用，把已产生 Observation
返回模型。

只读并发批次不因一个调用失败取消其他独立调用，除非 Run 被取消或预算耗尽。

默认并发上限：

```text
read/search: 4
render: 3
```

并发上限属于 Runtime 配置，不是工具参数。禁止向模型暴露 `parallel=true`。

每个调用仍然：

- 单独计入 Tool Budget；
- 单独进行 disclosure、scope 和 capability 校验；
- 拥有唯一 `call_id`；
- 产生配对的 `tool.started/tool.completed`；
- 产生独立 Observation 和 Evidence。

多 Tool Calls 不创建 Workflow DAG，不创建 Step 子 Loop，也不等于多 Agent。

## 14. 常驻 Render Worker

当前每次渲染启动 Node 和 Chromium 的实现应替换为常驻 Worker：

```text
Backend startup
→ start render worker
→ launch one Browser process
→ health ready

render_slide
→ acquire render slot
→ create isolated Browser Context/Page
→ render
→ collect diagnostics/screenshot
→ close Context/Page
→ retain Browser process
```

要求：

- Browser 进程跨页面复用；
- 每个页面使用隔离 Browser Context；
- 外部网络继续默认阻断；
- resource path 继续限制在当前 Project；
- 支持取消、超时和 Worker 异常重启；
- Worker 不健康时创建 Run 前给出明确产品错误；
- 默认最多并发渲染 3 页；
- 单页目标渲染时间为 2～3 秒级，不应重复冷启动浏览器；
- `render_slide` 工具名和参数保持不变。

## 15. 业务 System Prompt

### 15.1 Prompt 分层

```text
Runtime Policy
  权限、Loop、工具、finish、Plan、staging 隐藏规则

PPT Business Policy
  Outline、Design、Slide Spec、HTML 的职责与质量标准

Current Resource Contracts
  从领域 Schema 编译的字段说明和最小合法示例

Context Pack
  项目、当前资源、历史决定、参考和受控 refs

Current Runtime State
  Strategy、Phase、Plan、ChangeSet、Evidence 和 Observation
```

### 15.2 业务能力要求

Prompt 必须明确：

- Outline 如何表达目标、受众、核心命题、章节、叙事弧和页面顺序；
- Design 如何表达全局视觉语言而不是页面内容；
- Slide Spec 如何表达标题、核心句、内容摘要、要点、页面角色和视觉意图；
- HTML 如何遵守 `slide-stage`、公共 tokens/base、16:9、CJK、可访问性和资源限制；
- 单页信息密度、标题层级、内容取舍和跨页一致性；
- 写 HTML 后按需 `render_slide`，根据 Observation 自主修正；
- 小修改优先 `edit_ppt`，大重建使用 `write_ppt`；
- Agent 调用工具时只传原始 String，不传文件路径或 Runtime 元数据；
- 一次可返回多个无依赖 Tool Calls；
- 有依赖的调用必须按因果顺序分 turn 或让 Runtime 顺序执行；
- 最终交付必须显式 `finish(message=...)`。

Prompt 只提供专业原则和资源契约，不能固化成页面生成 Playbook、逐 Step Workflow 或独立 Verify/
Repair Stage。

### 15.3 Tool Call 示例

Prompt 应包含少量、准确示例：

- 读取 `deck:outline`；
- 完整写入 `slide:*:spec` JSON String；
- 完整写入 `slide:*:html` HTML String；
- 对 JSON String 做唯一锚点编辑；
- 对 HTML String 做唯一锚点编辑；
- 批量写入多个 Slide；
- 批量渲染多个独立 Slide；
- Gate 拒绝后继续修正并再次 `finish`。

示例必须从当前 Tool Schema 和领域 Schema 生成或测试，禁止长期手工漂移。

## 16. 公共事件与前端

公共事件名称仍为 11 种，不新增事件。

需要更新的 payload：

- `run.started.target.artifact` 使用 `spec | presentation`。
- Tool Event 的安全 target 使用 Resource 的业务投影。
- `tool.started/tool.completed` 不展示 raw content。
- Public Projector 根据资源生成：
  - “读取整份结构”
  - “更新全局设计”
  - “生成第 3 页设计稿”
  - “生成第 3 页 HTML”
- 不向普通 UI 展示磁盘文件名、路径、String 转义内容、Schema 错误全文或 staging。

前端命名：

```text
代码/API：spec
普通 UI：设计稿
```

三栏、顶部 Project Tabs、侧栏拖动与收起、Thread Tabs、Composer 和现有低噪声 Timeline 保持不变。

公共事件 payload 发生不兼容变化时，`schema_version` 升级为 `2`；不做 v1/v2 双写。

## 17. 错误语义

核心错误：

| Code | 含义 |
| --- | --- |
| `RESOURCE_INVALID` | Resource 联合不合法 |
| `RESOURCE_NOT_DISCLOSED` | 当前轮次不允许操作该资源 |
| `TARGET_OUT_OF_SCOPE` | Resource 超出当前 Run scope |
| `RESOURCE_NOT_FOUND` | 资源不存在 |
| `CONTENT_TOO_LARGE` | 资源超过模型可安全读取或写入的大小上限 |
| `CONTENT_INVALID` | JSON/HTML 字符串无法解析或不符合领域契约 |
| `EDIT_ANCHOR_NOT_FOUND` | `old_text` 匹配零次 |
| `EDIT_ANCHOR_AMBIGUOUS` | `old_text` 匹配多次 |
| `REVISION_CONFLICT` | committed baseline 已变化 |
| `RENDER_FAILED` | 页面渲染失败 |
| `STAGING_REQUIRED` | 写入缺少 staging transaction |

`CONTENT_INVALID` Observation 必须提供：

- resource key；
- 失败 JSON Pointer 或 HTML 检查项；
- 简短原因；
- 可执行修正建议。

禁止只返回“schema invalid”。

## 18. 实施边界

本次实施包括：

- 领域命名和文件布局重构；
- 数据一次性迁移；
- WorkSpec、Context、Store、Service、Workflow、API、Frontend 类型适配；
- 三个字符串工具；
- 领域 Schema 共源；
- 多 Tool Calls；
- 安全并发调度；
- 常驻 Render Worker；
- 业务 System Prompt；
- 相关测试和文档同步。

本次不包括：

- 新业务工具；
- 通用文件系统或 Shell；
- Repo 资产写入；
- 多 Agent；
- Workflow DAG；
- 智能视觉 Verifier；
- Eval Harness 或版本评测平台；
- Trace UI；
- PPTX/PDF 导出；
- 页面整体视觉重构。

## 19. 测试与验收

### 19.1 命名与迁移

- 新项目只创建 `outline.json/design.json/spec.json/index.html`。
- 代码和 API 的公开字段不再出现 `blueprint`。
- 模型工具参数不再出现 `global/model/include`。
- 一次性迁移保留旧项目内容、revision、slide order 和版本关系。
- 迁移后 Runtime 不再读取旧路径。

### 19.2 工具

- `read_ppt` 对四类资源返回准确原始 String。
- `write_ppt.content` 只接受 String。
- 三类 JSON String 分别经过对应领域 Schema。
- HTML String 经过 HTML parse/lint。
- `edit_ppt` 唯一锚点成功，零匹配和多匹配原子失败。
- 多 edit 任一失败时不产生部分 staged changes。
- talk/ask 不能写。
- spec Run 不能写 HTML。
- slide-level Run 不能写 Deck 或其他 Slide。
- Agent 不能通过 Resource 推导或访问任意路径。

### 19.3 多 Tool Calls

- Provider 返回的所有 Tool Calls 均被保留。
- 多个写调用按序执行。
- 一个写失败后后续依赖调用不执行。
- 独立 read/search 可并发。
- 不同 Slide render 可并发且不超过配置上限。
- 同一资源不并发写。
- 每个 Tool Call 的公共事件严格成对。
- 控制动作不能与普通工具混合。
- Completion Gate、预算、取消和 Trace 在批量调用下仍正确。

### 19.4 Render Worker

- Backend 启动时验证 Worker 和 Browser。
- 多页渲染复用同一个 Browser 进程。
- 每页使用独立 Context。
- Worker 超时、崩溃和取消不会泄漏 Browser/Context。
- 外部网络和项目路径越界继续被阻断。
- 真实 Chromium 集成测试在 CI 或明确的集成测试任务中运行。

### 19.5 Prompt

- Prompt 中资源名称与 Tool Schema 一致。
- Prompt 的领域字段说明来自同一份 Schema。
- Agent 能从空项目写出合法 Outline、Design、Slide Spec 和 HTML。
- Agent 能读后使用精确字符串编辑 JSON/HTML。
- Agent 能生成多个 Tool Calls，且不会混合 `finish/ask_user`。
- 普通文本仍不会隐式结束 Run。

### 19.6 产品主路径

真实模型完成：

```text
创建空项目
→ 写 Outline
→ 写 Design
→ 写全部 Slide Spec
→ 写全部 Slide HTML
→ 并发渲染
→ 修正失败页面
→ finish
→ Completion Gate
→ 原子 Commit
→ 前端刷新并可重新打开
```

同时完成：

- 修改单页 Spec；
- 修改单页 HTML；
- 修改整份 Outline；
- 修改整份 Design 并重新验证受影响页面；
- talk/ask/execute 在同一 Thread 中连续协作。

## 20. 完成定义

只有同时满足以下条件才算完成：

1. 新资源、命名和文件布局真实落地，不只是增加别名。
2. 三个工具使用统一 String 协议，旧参数不再披露。
3. 领域 Schema 是 Runtime 和 Prompt 的共同事实来源。
4. 公开 `artifact` 已从 `blueprint` 完整迁移为 `spec`。
5. 多 Tool Calls 不丢失，写顺序和只读并发规则正确。
6. Render Worker 复用 Browser，并通过真实 Chromium 集成测试。
7. 业务 Prompt 能指导真实模型完成主路径。
8. Completion Gate、staging、evidence、scope、事件顺序和原子 Commit 继续成立。
9. 后端、前端、集成、迁移和真实主路径测试通过。
10. 当前工作区中与本实施无关的用户修改没有被覆盖、删除或 reset。
