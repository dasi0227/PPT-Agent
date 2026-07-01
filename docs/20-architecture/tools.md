---
id: ARCH-TOOLS
title: 工具集（function calling schema）
status: approved
owner: backend
depends_on: [ARCH-HARNESS, DATA-MODEL, ASSET-PROTOCOL]
verifies: []
---

# 工具集（Harness Tools / function calling schema）

Harness 的每个动作都是一个**工具**：带 JSON 参数 schema 的确定性脚本。LLM 通过 function calling 选择工具，
脚本执行并返回 observation。这是 opaque-blob 理念的落地——**LLM 只做认知决策（选工具、给参数），文件 IO/校验/版本化全是脚本**。

## 工具总表

| 工具 | 类别 | 作用 | 可用 scope |
|---|---|---|---|
| `read_outline` | 只读 | 读大纲 slide-json[] | all |
| `read_slide` | 只读 | 读某页当前 html | current/page/overview |
| `patch_slide` | 写·页 | **锚定文本替换**某页 html（old→new） | current/page/overview |
| `write_slide` | 写·页 | 整页写入（仅生成阶段/子代理） | generate(子代理) |
| `read_common_style` | 只读 | 读公共层 tokens.css | overview |
| `patch_common_style` | 写·公共层 | 锚定替换公共层 token | overview |
| `apply_theme` | 写·公共层 | 应用 theme 资产到公共层 | overview |
| `search_assets` | 只读 | 检索个人仓库资产 | all |
| `read_asset` | 只读 | 读某资产载荷 | all |
| `mount_asset` | 写·页 | 移植资产进某页（layout/component/fx） | current/page/overview |
| `create_asset` | 写·仓库 | 新增资产 | repo |
| `patch_asset` | 写·仓库 | 修改资产 | repo |
| `delete_asset` | 写·仓库 | 删除资产 | repo |
| `validate_slide` | 校验 | 跑 html-output-spec，返回 observation | 写页后自动/手动 |
| `validate_asset` | 校验 | 校验资产载荷 schema | repo |
| `finish` | 控制 | 声明完成，退出循环 | all |

> 动态门控：实际注册给 LLM 的子集由 scope/mode 决定（见 [agent-harness](agent-harness.md) 门控表）。

## 核心工具 schema

### patch_slide（编辑核心，等同 edit_file）

```json
{
  "name": "patch_slide",
  "description": "对指定页 html 做锚定文本替换。old_text 必须在该页唯一出现，否则报错要求补充上下文。",
  "parameters": {
    "type": "object",
    "required": ["slide_idx", "edits"],
    "properties": {
      "slide_idx": { "type": "integer", "minimum": 0 },
      "edits": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["old_text", "new_text"],
          "properties": {
            "old_text": { "type": "string", "description": "锚点：页内唯一的原文片段" },
            "new_text": { "type": "string", "description": "替换后的新文本" }
          }
        }
      }
    }
  }
}
```
执行语义：脚本校验每个 `old_text` 在该页**唯一** → 替换 → 跑 `validate_slide` → 落盘 + 版本；任一锚点不唯一/不存在则**整体失败**，返回错误 observation（LLM 据此补上下文重试）。

### write_slide（生成阶段）

```json
{
  "name": "write_slide",
  "description": "整页写入 html（生成阶段或子代理使用）。",
  "parameters": {
    "type": "object",
    "required": ["slide_idx", "html"],
    "properties": {
      "slide_idx": { "type": "integer", "minimum": 0 },
      "html": { "type": "string" }
    }
  }
}
```

### apply_theme / patch_common_style（/overview 核心）

```json
{
  "name": "apply_theme",
  "description": "将仓库中某 theme 资产的 token 全集写入公共样式层，一次性全局换肤。",
  "parameters": {
    "type": "object",
    "required": ["theme_asset_id"],
    "properties": { "theme_asset_id": { "type": "string" } }
  }
}
```
```json
{
  "name": "patch_common_style",
  "description": "锚定替换公共层 tokens.css（用于微调单个 token，而非整体换主题）。",
  "parameters": {
    "type": "object",
    "required": ["edits"],
    "properties": {
      "edits": { "type": "array", "items": {
        "type": "object", "required": ["old_text","new_text"],
        "properties": { "old_text": {"type":"string"}, "new_text": {"type":"string"} } } }
    }
  }
}
```

### mount_asset（移植资产进页）

```json
{
  "name": "mount_asset",
  "description": "按资产 manifest 的挂载约定，将 layout/component/fx 资产注入指定页。",
  "parameters": {
    "type": "object",
    "required": ["slide_idx", "asset_id"],
    "properties": {
      "slide_idx": { "type": "integer", "minimum": 0 },
      "asset_id": { "type": "string" },
      "params": { "type": "object", "description": "按资产 params schema 填参；缺省走默认" }
    }
  }
}
```

### search_assets

```json
{
  "name": "search_assets",
  "description": "按 kind 与语义 query 检索个人仓库资产，返回资产索引（不含完整载荷）。",
  "parameters": {
    "type": "object",
    "required": ["query"],
    "properties": {
      "kind": { "type": "string", "enum": ["layout","component","theme","fx"] },
      "query": { "type": "string" }
    }
  }
}
```

### create_asset / patch_asset / delete_asset（/repo）

```json
{
  "name": "create_asset",
  "description": "在个人仓库新增一个资产（layout/component/theme/fx），manifest 须符合资产协议 schema。",
  "parameters": {
    "type": "object",
    "required": ["manifest", "payload"],
    "properties": {
      "manifest": { "type": "object", "description": "统一信封字段，见 asset-protocol" },
      "payload": { "type": "object", "description": "按 kind 对应的载荷（html/css/js/tokens）" }
    }
  }
}
```
（`patch_asset` 用 asset_id + 锚定 edits；`delete_asset` 用 asset_id。）

### finish

```json
{
  "name": "finish",
  "description": "声明任务完成，退出 ReAct 循环。",
  "parameters": {
    "type": "object",
    "required": ["summary"],
    "properties": { "summary": { "type": "string" } }
  }
}
```

## 约束

| ID | 约束 |
|---|---|
| `ARCH-TOOLS-001` | 每个工具 MUST 有 JSON 参数 schema，参数 MUST 校验后才执行 |
| `ARCH-TOOLS-002` | 写类工具 MUST 在成功后落版本（[versioning](../30-data-model/versioning.md)） |
| `ARCH-TOOLS-003` | `patch_*` 的 `old_text` 不唯一/不存在 MUST 整体失败并返回可操作错误 observation |
| `ARCH-TOOLS-004` | 写页工具 MUST 在落盘前隐式跑 `validate_slide`，不合规则拒绝落盘 |
| `ARCH-TOOLS-005` | 工具执行 MUST 串行化共享状态写入（state.json/SQLite），防竞态 |
| `ARCH-TOOLS-006` | **路径边界（正确性护栏）**：所有工具的文件读写 MUST 限定在当前 project 的 work_dir 或全局 `_assets/` 内。路径 MUST 规范化后做前缀校验，拒绝 `..`/绝对路径/符号链接逃逸；越界 MUST 整体失败并返回错误 observation，不写任何文件 |

> `ARCH-TOOLS-006` 不是"安全剧场"，而是防止 LLM 生成异常路径（如 `../../../`）把 work_dir 之外的本机文件写坏的**正确性边界**。单机无登录场景下，用户级权限/RBAC/网络攻击面防护均不做（见 [ADR-0011](../90-decisions/0011-security-posture.md)）。

## 验收标准（Given-When-Then）

- **AC-TOOLS-003**（`ARCH-TOOLS-003`）
  - GIVEN `patch_slide` 的 old_text 在页内出现两次
  - WHEN 执行
  - THEN 整体失败，observation 提示「锚点不唯一，请补充上下文」，文件未变

- **AC-TOOLS-004**（`ARCH-TOOLS-004`）
  - GIVEN 一次会破坏 html-output-spec 的 patch
  - WHEN 执行
  - THEN validate 失败 → 拒绝落盘 → 返回错误 observation

- **AC-TOOLS-006**（`ARCH-TOOLS-006`）
  - GIVEN 一个带越界路径（如 `../../etc/x` 或绝对路径）的工具调用
  - WHEN 执行
  - THEN 路径校验失败，整体拒绝，work_dir 外无任何文件被写入

## 校验方式

```bash
go test ./internal/harness/tools -run 'TestPatchAnchorUnique|TestValidateBeforeWrite|TestToolSchema|TestPathBoundary'
# 工具 schema 合法性：逐个 validate 为合法 JSON Schema
```

## 依赖

- [ARCH-HARNESS](agent-harness.md)、[DATA-MODEL](../30-data-model/data-model.md)、[ASSET-PROTOCOL](../60-design-system/asset-protocol.md)
