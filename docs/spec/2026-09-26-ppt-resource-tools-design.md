# PPT 资源编辑与读取工具设计

日期：2026-09-26。

状态：已于 2026-09-26 按本文完成代码重构，模型工具、运行提示词与前端事件识别已同步切换。实现细节与验证范围见第 8 节；真实模型及界面操作仍待手动验收。

## 1. 目标与范围

将 Manifest、Design、单页 Spec 的编辑从 `mutate_ppt` 拆成三个独立工具：`edit_manifest`、`edit_design`、`edit_spec`。直接用各对象的业务字段表达修改，减少模型理解操作名、JSON Pointer、Patch 数组与值类型之间组合关系的负担。

已确认七个编辑工具，按操作方式划分如下；其中两个 Outline 工具按文件存在状态择一暴露：

| 工具 | 操作方式 |
| --- | --- |
| `edit_manifest`、`edit_design`、`edit_spec` | 按业务字段更新结构化对象 |
| `write_html` | 首次创建或完整替换一页 HTML 源码 |
| `patch_html` | 对已有页面源码应用精确文本替换 |
| `init_outline` | Outline 文件不存在时，以完整 JSON 源码初始化目录 |
| `arrange_outline` | Outline 文件已存在时，对其 JSON 源码应用精确文本替换 |

`edit` 与 `patch` 在这里区分业务字段编辑和源码补丁；三个 JSON 编辑工具同样支持局部更新。HTML 全量与局部操作分别由 `write_html` 和 `patch_html` 表达。

读取统一使用 `read_resource`：通过 `resource` 选择类型，单页 Spec 和 HTML 额外通过 `slide_id` 定位。

工具按逻辑资源划分，存储继续沿用现有结构：

| 逻辑资源 | 物理存储 | 操作范围 |
| --- | --- | --- |
| Manifest | `artifacts/.manifest.json` | 整份演示的内容要求 |
| Design | `artifacts/.design.json` | 整份演示的视觉要求 |
| Outline | `artifacts/.outline.json` | 整份演示的章节、页面成员、标题与顺序 |
| 单页 Spec | `artifacts/.spec.json` 中以 `slide_id` 为键的一项 | 一页的设计稿 |
| 单页 HTML | `artifacts/<slide_id>.html` | 一页的实际源码 |

Spec 属于稳定页面身份，不以 HTML 已存在为前提。集中保存全部 Spec 不代表工具可以一次覆盖整个集合。

Outline 仍负责页面成员、标题、归属与顺序，改用源码初始化与编辑，不向模型暴露 `action`、`node`、`parent` 等结构操作协议。Runtime 负责解析最终结构、分配新 ID、维护关联内容和原子提交。具体合同及工具可见性见第 5 节。

## 2. 已确认：三个 JSON 工具的共同规则

### 输入语义

- 全部可编辑顶层业务字段直接作为工具参数，不再嵌套在 `patch`、`data` 或 `content` 中。
- 业务字段在编辑请求中均可省略；省略表示保持原值，提交表示修改。`edit_spec` 另需必填的 `slide_id`。
- 每次至少提交一个可编辑字段；只提供页面身份不构成有效编辑。
- 字符串整体替换；数组整体替换，不追加、不按下标合并，`[]` 表示清空数组。
- `decorations` 按其固定子字段更新，未提交的子字段保持原值。这是明确的字段规则，不扩展为任意对象的递归合并协议。
- 拒绝未知字段、错误类型和非法枚举，不将字符串自动转换为数组或对象。
- 必填业务字段不能通过 `null` 删除。可选 Spec 字段的显式清除方式见第 8 节，不能把省略解释成删除。

编辑参数的可选性不改变最终文档 Schema 的必填要求。执行流程为：定位资源并检查权限与版本 → 合并本次字段 → 校验完整结果 → 原子保存 → 返回保存后的完整业务对象。任一字段无效时，整次调用失败，不保存局部结果。

项目由当前运行上下文确定，模型不传 `project_id` 或文件路径。页面通过稳定 `slide_id` 定位，不用页码定位。

### 输出语义

成功返回 `ok`、`changed_fields` 和修改后的完整资源；`edit_spec` 额外返回 `slide_id`。资源字段分别为 `manifest`、`design`、`spec`。

`changed_fields` 只列出实际发生变化的顶层字段，提交了相同值时不列入；例如只修改 `decorations.page_number` 时列出 `decorations`。没有实际变化时返回空数组和当前完整对象，不伪造一次内容变更。

完整对象是成功保存后的权威值，包含未修改的现有字段，不只是输入的回显。可选字段未设置时仍省略，不补 `null` 或默认角色。

仅修改这些参考数据不自动生成或重写 HTML，也不推进 HTML 生成参考快照。持久化、冲突保护、页面权限、失败回滚和历史能力继续由 Runtime 承担。

## 3. 已确认：各 JSON 工具

### 3.1 `edit_manifest`

修改项目初始化时已有的 Manifest，不单独设置 `create_manifest`。

| 参数 | 类型 | 编辑时必填 | 语义 |
| --- | --- | --- | --- |
| `title` | string | 否 | 演示标题 |
| `language` | string | 否 | 内容语言 |
| `pages` | string | 否 | 期望页数，如 `10`、`11-12`、`约10`；不直接增删页面 |
| `audience` | string | 否 | 受众及必要背景 |
| `goal` | string | 否 | 希望听众达成的结果 |
| `requirements` | string[] | 否 | 完整替换额外内容要求列表，不重复基本信息 |
| `prohibitions` | string[] | 否 | 完整替换禁忌列表 |

例如只修改目标和要求：

```json
{
  "goal": "帮助团队判断何时应该编写 Skill",
  "requirements": ["说明适用边界", "提供具体示例"]
}
```

成功返回示例：

```json
{
  "ok": true,
  "changed_fields": ["goal", "requirements"],
  "manifest": {
    "title": "Skill 设计分享",
    "language": "zh-CN",
    "pages": "11-12",
    "audience": "研发团队",
    "goal": "帮助团队判断何时应该编写 Skill",
    "requirements": ["说明适用边界", "提供具体示例"],
    "prohibitions": []
  }
}
```

最终 Manifest 要求上述七个字段全部存在。字符串长度、数组数量及条目约束沿用业务 Schema，不在工具层另建一套不同规则。期望页数与字段归属详见 [批量加载组件与演示页数](2026-09-26-batch-components-and-manifest-pages-design.md)。

### 3.2 `edit_design`

修改项目已有的 Design。

| 参数 | 类型 | 编辑时必填 | 语义 |
| --- | --- | --- | --- |
| `direction` | string | 否 | 整体视觉方向 |
| `layout_preferences` | string[] | 否 | 完整替换排版偏好列表 |
| `decorations` | object | 否 | 修改给出的公共装饰位置 |

`decorations` 接受四个可选子字段：`page_number`、`section_title`、`deck_title`、`key_message`。各字段取值沿用现有位置枚举，其中 `page_number` 不接受 `none`，其他三个允许 `none` 表示不展示。工具接受部分子字段，最终 Design 仍保存全部四个子字段。

例如只修改页码位置：

```json
{
  "decorations": {
    "page_number": "bottom-center"
  }
}
```

成功返回示例：

```json
{
  "ok": true,
  "changed_fields": ["decorations"],
  "design": {
    "direction": "通过留白和对齐组织信息",
    "layout_preferences": ["优先采用清晰的两栏对照"],
    "decorations": {
      "page_number": "bottom-center",
      "section_title": "top-left",
      "deck_title": "none",
      "key_message": "none"
    }
  }
}
```

`direction` 可为空字符串，`layout_preferences` 可为空数组，具体合法性沿用当前 Schema。没有可更新子字段的 `decorations: {}` 应按无有效编辑内容处理。

### 3.3 `edit_spec`

创建或局部修改指定页面的 Spec。每次只处理一页，返回该页完整 Spec。

| 参数 | 类型 | 编辑时必填 | 语义 |
| --- | --- | --- | --- |
| `slide_id` | string | 是 | Outline 中已存在的稳定页面 ID |
| `key_message` | string | 否；首次创建时是 | 本页核心信息 |
| `elements` | object[] | 否；首次创建时是 | 完整替换本页内容元素列表 |
| `role` | enum | 否 | 可选页面角色 |
| `layout` | string | 否 | 可选布局建议 |

每个元素包含 `type` 与 `intent`。`type` 沿用 `text`、`list`、`metric`、`quote`、`table`、`chart`、`diagram`、`code`、`asset`；`role` 沿用现有 12 种页面角色。不把页面标题、顺序或 ID 写入 Spec 正文。

已有 Spec 时，传入字段合并到原对象；没有 Spec 时，以传入内容创建，必须同时具备 `key_message` 和 `elements`。按当前 Schema，`elements: []` 合法。创建失败不能留下占位 Spec，页面不存在时也不能顺便创建 Outline 页面。

修改示例：

```json
{
  "slide_id": "sli_example",
  "key_message": "优秀 Skill 必须有明确的适用边界"
}
```

成功返回示例：

```json
{
  "ok": true,
  "slide_id": "sli_example",
  "changed_fields": ["key_message"],
  "spec": {
    "key_message": "优秀 Skill 必须有明确的适用边界",
    "elements": [
      {"type": "diagram", "intent": "展示适用与不适用的场景"}
    ],
    "role": "definition",
    "layout": "左右对比"
  }
}
```

写入物理 `.spec.json` 时必须保留其他页面的项；冲突判断、权限和变化识别均以目标页面为单位。不能要求模型读取或重写整个集合才能修改一页。

## 4. 已确认：HTML 分为 `write_html` 与 `patch_html`

HTML 是自由文本源码，不适合按 JSON 业务字段编辑。现有实现已经区分 `slide.html.write` 与 `slide.html.patch`；将它们直接暴露为两个含义明确的工具，不再通过一个操作枚举切换不同输入形状。局部修改工具确定命名为 `patch_html`，其补丁格式采用原文精确替换，不引入 unified diff 或 JSON Patch 语法。

| 工具 | 用途 | 核心输入 | 输出 |
| --- | --- | --- | --- |
| `write_html` | 首次生成，或完整替换一页源码 | `slide_id`, `html` | `ok`, `slide_id`, `changed`, `content_hash` |
| `patch_html` | 对已有源码作局部文本替换 | `slide_id`, `edits: [{old_text, new_text}]` | `ok`, `slide_id`, `changed`, `content_hash` |

表中为业务输入；版本检查元信息见第 8 节，不因精简接口而取消覆盖保护。

### `write_html` 的语义

接收该页完整 HTML，页面必须已存在于 Outline。没有 HTML 时创建，已有时整体替换；工具描述须明确写出覆盖语义。它不接受完整 Spec、不隐式调用模型生成源码，也不创建页面身份。

`create_html` 无法准确涵盖整页重写；`update_html` 无法区分全量与局部；`edit_file` 则要求模型面对路径和额外文件范围。当前产品采用 `write_html` 更直接。若未来同时支持多种 HTML 资源，再考虑 `write_slide_html`，目前页面参数已足以明确范围。

### `patch_html` 的语义

```json
{
  "slide_id": "sli_example",
  "edits": [
    {
      "old_text": "<h1>适用场景</h1>",
      "new_text": "<h1>明确适用边界</h1>"
    }
  ]
}
```

- HTML 必须存在，`edits` 非空；`old_text` 非空，`new_text` 可为空以表达删除。
- 在内存中的候选源码上按顺序应用替换，每项 `old_text` 必须精确匹配一次。
- 匹配零次或多次时失败，返回失败项与匹配数量，不猜测目标、不默认全局替换。
- 整批替换完成并校验通过后一次提交；中途失败不保存前面的替换。
- 初版不引入正则表达式、CSS 选择器修改、行号补丁或 `replace_all`，避免增加定位与失败语义。

两种工具共享现有 HTML 校验和提交链路。写入成功只说明源码保存成功，视觉检查仍通过渲染与截图完成。真实 HTML 字节变化且提交成功后才更新生成参考快照；同内容写入不推进。

HTML 编辑结果默认不回传整页源码：整页写入时模型已经提供全文，局部编辑时主要需要知道是否成功与新版本。后续需要全文时显式读取。三个 JSON 工具仍返回完整对象，因为这些对象结构明确、规模受限，返回值有助于确认合并后的状态。

## 5. 已确认：Outline 源码工具与动态可见性

Outline 是多级 JSON 目录树。模型直接编写或修改源码，后端处理结构与关联一致性；不再另设新增、移动、修改、删除节点的工具，也不使用包含这些操作的 `operations` 数组。

### 5.1 文件生命周期决定工具可见性

| 当前状态 | 模型可见的 Outline 编辑工具 |
| --- | --- |
| `artifacts/.outline.json` 不存在 | 仅 `init_outline` |
| `artifacts/.outline.json` 已存在 | 仅 `arrange_outline` |

此规则只决定两个 Outline 编辑工具中的可用项，不影响其他工具按各自任务权限暴露。模型不需要自行判断应该选择哪一种 Outline 操作。

`ProjectService.initWorkDir` 已取消预先写入 `{"sections":[]}`：新项目不创建 `.outline.json`，由首次成功的 `init_outline` 创建。文件路径和正式存储格式保持不变，不额外维护“已初始化”状态字段。

- `init_outline` 校验与提交成功后文件才存在；失败不得留下占位文件或部分内容。
- 下一次模型请求根据最新已提交状态暴露 `arrange_outline`，不再暴露 `init_outline`。运行中的资源视图与实际提交状态必须保持一致。
- 后续即使删除全部章节、得到 `{"sections":[]}`，也保留文件并继续暴露 `arrange_outline`。不以章节数或页面数判断是否已初始化。
- 工具执行时再次检查文件状态与权限；过期的 `init_outline` 调用不能覆盖已有文件，`arrange_outline` 也不能隐式创建不存在的文件。
- 文件存在但读取失败或内容损坏时明确报告错误，不能将其当成“不存在”并重新初始化覆盖。

### 5.2 `init_outline`

| 参数 | 类型 | 必填 | 语义 |
| --- | --- | --- | --- |
| `content` | string | 是 | 完整初始 Outline 的 JSON 源码 |

固定操作当前项目的 `.outline.json`，不接受文件路径或项目 ID。仅在文件不存在时执行，已有文件即使内容为空目录也不能使用此工具。

`content` 包含完整根对象与 `sections` 数组。章节、小节和页面的业务字段沿用现有 Outline 结构；新节点省略 `id` 或 `slide_id`，由 Runtime 生成正式 ID。初始化不接受模型自行指定的节点 ID。

后端解析输入草稿、补齐 ID、校验完整正式结构，并通过统一事务提交。成功后返回实际保存的完整 JSON 源码，其中包含所有新生成的 ID。创建目录只创建页面身份，不自动生成占位 Spec 或 HTML。

### 5.3 `arrange_outline`

| 参数 | 类型 | 必填 | 语义 |
| --- | --- | --- | --- |
| `edits` | object[] | 是 | 按顺序应用的原文替换列表，不能为空 |
| `edits[].old_text` | string | 是 | 必须非空且精确匹配一次的原文 |
| `edits[].new_text` | string | 是 | 替换后的文本，可为空以删除原文 |

```json
{
  "edits": [
    {
      "old_text": "\"title\": \"项目背景\"",
      "new_text": "\"title\": \"背景与目标\""
    }
  ]
}
```

固定操作当前已存在的 Outline。新增、删除、重排、分组和改名均通过 JSON 源码修改表达，不增加 `action`、`mode`、`node_id`、`parent_id`、`cascade` 或临时节点引用等操作参数。

在内存中的候选文本上依次匹配和替换，任一项匹配零次或多次即整批失败，并反馈失败项及匹配数量。全部替换完成后才解析并校验最终 JSON；不要求每个中间文本都构成有效目录。最终结构合法且关联处理成功后一次提交，不保存中间结果。

### 5.4 节点身份、关联内容与返回值

- 已有节点使用原 ID。相同 ID 表示同一节点，改名、移动或重新分组不生成新的页面身份，也不重写其 Spec、HTML。
- 新节点省略相应的 `id` 或 `slide_id`，由后端生成；已提供的 ID 必须来自当前 Outline，类型必须匹配且最终结构中不得重复。不凭标题或位置猜测节点身份。
- 省略 ID 明确表示创建新节点，不用这种方式表达已有节点的移动。旧页面 ID 未出现在最终结构中时，视为删除该页。
- 后端对比前后完整目录，统一处理被删除页面的 Spec、HTML 与关联状态。目录、关联内容和状态作为整体提交，任一处理失败全部回滚；不能仅写 Outline 后再独立清理文件。
- 输入草稿允许新节点缺少 ID；补齐后的正式文件仍遵守原 Outline Schema 与结构约束，包括章节不能同时包含直属页面和小节。源码编辑接口不放宽最终持久化规则或任务权限。
- 两个工具均返回实际保存后的完整 JSON 源码，包含后端补齐的 ID，供下一次编辑直接使用。输出正文与已保存文本一致；外层结果封装和版本元信息按统一合同收敛。
- 目录变更不自动创作 HTML，也不推进 HTML 生成参考快照；页序、章节或标题变化引起的渲染依赖变化继续按现有规则处理。

### 5.5 读取配套

`read_resource` 使用 `{"resource":"outline"}` 读取实际保存的完整 JSON 源码字符串，与 `arrange_outline` 的匹配文本保持一致。不能把解析后重新排版的对象、附加 `ordinal` 等展示字段的投影或带行号的文本当作源文件正文返回。

Outline 尚不存在时明确返回“尚未初始化”，不创建文件，也不返回可被误认为已保存内容的空目录。工具读取已从 `ModelOutline` 展示对象切换为源码读取；其他上下文展示不应改变用于文本编辑的源文。

## 6. 已确认：`read_resource` 的名称与资源定位规则

读取统一命名为 `read_resource`，每次定位一个逻辑资源并返回内容。项目仍由当前运行上下文确定，无需传入文件路径或 `project_id`。

已确认参数：

| 参数 | 类型 | 规则 |
| --- | --- | --- |
| `resource` | enum | `manifest`、`design`、`spec`、`html`、`outline` |
| `slide_id` | string | `spec`、`html` 必填；全局资源不接受 |

`resource` 每次取一个枚举值。`spec`、`html` 必须提供稳定 `slide_id`；`manifest`、`design`、`outline` 直接定位全局内容，不接受 `slide_id`。缺少必填页面身份、传入无关页面身份或使用未知资源类型时返回参数错误。

用扁平资源名替代当前嵌套的 `kind/part` 组合，工具输入和输出采用同一种资源标识。

| 读取对象 | 输入示例 | 返回内容 |
| --- | --- | --- |
| Manifest | `{"resource":"manifest"}` | 完整 Manifest 对象 |
| Design | `{"resource":"design"}` | 完整 Design 对象 |
| Outline | `{"resource":"outline"}` | 实际保存的完整 Outline JSON 源码字符串，可直接用于文本替换 |
| Spec | `{"resource":"spec","slide_id":"sli_example"}` | 指定页的完整 Spec 对象，不是 `.spec.json` 的全部页面集合 |
| HTML | `{"resource":"html","slide_id":"sli_example"}` | 指定页实际保存的完整 HTML 源码字符串 |

页面身份归属 Outline，读取 Spec 不要求该页已生成 HTML。物理存储布局继续由 Runtime 处理，Spec 集中存储不改变单页定位和读取权限。

### 读取结果与上下文处理建议

建议统一读取结果的外层结构：

```json
{
  "ok": true,
  "resource": "spec",
  "slide_id": "sli_example",
  "content_hash": "<当前资源版本>",
  "content": {
    "key_message": "优秀 Skill 必须有明确的适用边界",
    "elements": [
      {"type": "diagram", "intent": "展示适用与不适用的场景"}
    ],
    "role": "definition",
    "layout": "左右对比"
  }
}
```

全局资源省略 `slide_id`。Manifest、Design、Spec 返回 JSON 对象；Outline 返回原始 JSON 源码字符串，保持其源码编辑合同；HTML 返回原始创作源码字符串，不混入主题、公共装饰或预览交互代码。工具读取应看到当前运行中此前成功提交的修改。

目标页不存在、资源尚未生成、内容损坏、无权读取应分别反馈，不能统一返回空对象或空字符串。读取不创建任何资源，也不推进生成参考快照。

目前 `read_ppt` 会在相同版本已存在于可见上下文时省略正文，返回 `already_available`。建议显式读取成功时始终返回请求内容，使读取合同稳定；自动上下文注入仍可单独去重。两者不必使用同一种正文省略策略。

首版保持一次读取一个资源，不增加批量读取或字段投影。HTML 超过读取或上下文预算时必须明确报错，不静默截断并声称全文；如果长 HTML 成为实际常见问题，再增加仅适用于 HTML 的行区间读取，并明确返回区间及全文版本。

## 7. 实施边界

三个 JSON 工具实施时，替代模型可见的 `manifest.patch`、`design.write`、`design.patch`、`slide.spec.write` 和 `slide.spec.patch` 入口。后端可以复用现有校验与事务逻辑，但不让模型同时面对两套等价编辑协议。

HTML 工具实施时，以 `write_html`、`patch_html` 替代模型可见的 `slide.html.write`、`slide.html.patch` 入口，同样不保留两套等价编辑协议。

Outline 工具实施时，以 `init_outline`、`arrange_outline` 替代模型可见的 `outline.init`、`outline.insert`、`outline.move`、`outline.update`、`outline.remove`，不保留另一套目录动作协议。统一提交层继续负责节点身份与关联一致性。

读取工具实施时，以 `read_resource` 和第 6 节确定的扁平定位参数替代模型可见的 `read_ppt` 入口，不同时暴露两个名称或保留旧参数兼容。Outline 正文切换为原始 JSON 文本，Manifest、Design、Spec 仍返回对象。

全部资源切换完成后移除模型可见的 `mutate_ppt`。同步调整工具注册、按状态生成的工具列表、执行时状态检查、权限映射、提示词、上下文资源版本、工具结果展示和相关合同检查。

取消新项目的空 Outline 预创建，并让内容加载、上下文、管理入口、历史与恢复等相关流程识别合法的“尚未初始化”状态。加载和读取不能为了维持旧假设自动补建文件。既有 Outline 文件即使为空，也按已存在处理，不增加旧数据迁移或额外初始化状态标记。

本次不改变文件路径和正式 Outline 数据结构；变化包括工具协议、Outline 创建时机与源码读写方式。不重新设计业务管理 UI，不引入通用任意文件写入权限，也不修改 `DESIGN.md`。

后续验收重点是：省略字段保留、数组整替换、装饰子字段保留、非法请求原子失败、首次 Spec 的必填约束、单页修改不覆盖其他页、返回真实完整对象、冲突时不覆盖新值。HTML 另检查唯一匹配和整批失败回滚；读取检查单页范围、原始源码以及缺失与错误的区分。

Outline 另需验收：新项目没有文件且只暴露初始化工具，初始化失败不留下文件，成功后下一次请求只暴露编排工具；文件存在但为空时仍可编排；过期初始化不能覆盖已有内容；读取和工具返回的文本可直接用于下一次精确替换；新节点 ID 由后端生成，已有节点移动保留 Spec、HTML；删除页面联动清理且失败整体回滚；读取失败不能当成未初始化覆盖。实现与自动检查结果见下节。

## 8. 实现细节与验证记录

### 8.1 合同细节

- `edit_spec` 的 `role: null`、`layout: null` 删除相应可选字段，保存后省略；其他字段不接受 `null`。省略参数仍表示保留原值。
- 除 `init_outline` 外，编辑工具均提供可选 `expected_hash`，接收读取或上次编辑返回的 `content_hash`。Outline、HTML 使用原始字节哈希，其他资源使用规范化 JSON 哈希，Spec 只比较目标页。保留底层事务提交的冲突保护；未传此参数时，不宣称已校验模型读取版本。
- `read_resource` 每次显式调用均返回完整正文，封装为 `ok`、`resource`、`content_hash`、`content`，单页资源另含 `slide_id`。Outline、HTML 的 `content` 为源码字符串，其余为完整对象。
- JSON 编辑返回 `ok`、`changed`、`content_hash`、`changed_fields` 和以资源名命名的完整对象；Spec 另含 `slide_id`。Outline 编辑返回完整源码 `content`；HTML 编辑只返回状态、`slide_id` 与哈希。
- 每次模型请求前重新按 `.outline.json` 是否存在过滤工具列表；执行时再次检查。初始化成功提交后下一次请求只暴露 `arrange_outline`；已有空文件同样只暴露编排工具。
- 原子提交、节点身份同步、删除关联清理、HTML 生成参考快照仍由现有事务链路处理。仅编辑参考内容不会推进 HTML 的生成基线。

### 8.2 实施范围

已替换模型可见的 `mutate_ppt`、`read_ppt`，删除旧联合编辑 Schema，同步提示词、错误引导、公开事件名称、前端活动展示与刷新逻辑。管理界面 HTTP API 使用的内部节点操作仍保留，以服务现有界面行为；它们不是模型工具，也不构成模型旧协议兼容入口。

新项目不再创建空 Outline；上下文加载与管理快照识别未初始化状态且不补写文件。仅调整参考内容的运行可以在页面 Spec 尚未生成时完成，页面交付的验证要求继续由完成检查负责。

### 8.3 自动检查与待验收项

- 后端所有包及测试文件编译检查通过（`go test ./... -run '^$'`）；此命令不执行测试用例。
- 后端针对性测试通过：字段合并与可选字段删除、单页 Spec 隔离、冲突拒绝、Outline 身份保留与删除回滚、源码读取、工具动态切换、权限、事务提交、幂等性、HTML 参考快照，以及新项目无 Outline 的初始化流程。
- 完整 Runtime 流程测试通过：初始化提交成功后下一次模型请求切换到 `arrange_outline`，新建页面 ID 已持久化并进入全页操作范围。
- 前端 TypeScript 检查通过；事件解析、运行状态、历史还原、上下文面板与活动展示等 7 个相关测试文件共 93 项通过。
- 扩展后端测试未全绿：上下文注入／压缩、Briefing 策略、日志回放身份校验、Polish 上下文及公开文本替换仍有失败。这些检查不属于本次工具协议验收已通过的范围，不能据此宣称全量回归通过。
- 未运行浏览器自动化。真实模型调用与界面交互按项目规定交由用户手动验收。

## 9. 依据

- [创作目录与管理 UI](2026-09-25-authoring-files-and-management-ui-design.md)：文件布局、Spec 集中存储与单页操作边界。
- [页面角色归属 Spec 并改为可选](2026-09-26-optional-slide-role-in-spec-design.md)：角色归属、可选字段的持久化语义。
- [HTML 生成参考快照](2026-09-25-html-generation-reference-snapshots-design.md)：参考修改、HTML 修改与基线推进之间的关系。
- 当前业务 Schema：`backend/schemas/manifest.schema.json`、`backend/schemas/design.schema.json`、`backend/schemas/slide-spec.schema.json`、`backend/schemas/outline.schema.json`。
- 当前工具与执行逻辑：`backend/internal/workflow/ppt_tools.go`、`backend/internal/workflow/ppt_helpers.go`、`backend/internal/pptmutation/mutation.go`；当前项目初始化：`backend/internal/service/project.go`；当前 Outline 展示投影：`backend/internal/contextengine/model_view.go`。

本文优先替代旧文档中通过 `mutate_ppt` 编辑这些资源、通过 `read_ppt` 读取资源，以及新项目预创建空 Outline 的约定。Outline 的正式存储结构、其他资源的数据职责和未被本文调整的业务与持久化约束继续有效。
