---
id: AGENT-CONTEXT
title: 上下文装配
status: approved
owner: agent
depends_on: [AGENT-PROMPTS, DATA-MODEL, DS-TOKENS]
verifies: []
---

# 上下文装配

装配「喂给 LLM 的上下文层」：根据 scope/mode，选取恰当的目标产物、设计系统切片、选区与插件，
保证 LLM 拿到**足够且不过量**的信息（渐进式、按需）。

## 装配输入

| 来源 | 内容 |
|---|---|
| Run 参数 | kind/scope/mode/page_index/instruction/selection |
| **线程历史** | 当前 thread 的对话历史（`threads/<id>.jsonl`），提供跨 Run 连续性；长会话按预算滚动压缩 |
| 目标产物 | page scope → 当前页 html + slide-json；overview → 公共层 tokens.css + 各页标题索引；repo → 目标资产载荷 |
| 设计系统切片 | 相关版式/图表/动效目录条目（不全量塞入） |
| 个人仓库 | 用户提及或相关的资产 manifest |
| 选区（backlog） | `selection`：{ screenshot_crop, dom_selector_range, bbox }（MVP 预留） |
| 历史摘要 | 最近相关版本/run 摘要（供 /recap 与编辑连续性） |

## 按 scope 的装配策略

| scope | 注入 | 不注入 |
|---|---|---|
| current / page | 仅目标页 html + slide-json + 相关 token 名 | 其它页 html、公共层全文（仅给 token 清单） |
| overview | 公共层 tokens.css 全文 + token 语义说明 + 各页标题/版式（轻量索引） | 各页 html 全文（逐页 patch 时由子代理各自注入单页） |
| repo | 目标资产载荷 + 资产协议 schema 摘要 | 任何 PPT 项目的页/公共层 |

> 装配出的上下文与 harness 动态工具集是一对：注入什么 = 该 scope 工具能操作什么。

## 设计系统切片（渐进式）

- 先给「目录索引」（版式名/图表名/动效名 + 一句话），仅在选定后再展开具体规格。
- 借鉴参考项目的渐进式披露：给地图，再按需给细节，控制 token 预算。

## 选区上下文（backlog 预留）

- `selection` 字段存在时（未来框选编辑），装配为：裁剪截图引用 + DOM 选择器范围 + bbox。
- MVP 阶段该字段可为空；装配器 MUST 容忍缺省。

| ID | 约束 |
|---|---|
| `AGENT-CTX-001` | page scope MUST NOT 注入其它页 html（隔离 + 控量） |
| `AGENT-CTX-002` | overview scope MUST 注入公共层全文，MUST NOT 注入单页 html |
| `AGENT-CTX-003` | 设计系统 MUST 渐进式注入（先索引后细节） |
| `AGENT-CTX-004` | 上下文装配 MUST 容忍 `selection` 缺省（backlog 兼容） |
| `AGENT-CTX-005` | 注入总量 MUST 受 token 预算约束，超限时按优先级裁剪（目标产物 > 设计系统细节 > 历史） |

## 验收标准（Given-When-Then）

- **AC-CTX-001**（`AGENT-CTX-001`）
  - GIVEN page scope 编辑第 3 页
  - WHEN 装配上下文
  - THEN 上下文不含第 0/1/2/4… 页的 html 全文

- **AC-CTX-004**（`AGENT-CTX-004`）
  - GIVEN 编辑请求未带 selection
  - WHEN 装配
  - THEN 正常装配，不报错

## 校验方式

```bash
go test ./internal/agent -run 'TestContextScopeIsolation|TestContextBudget'
```

## 依赖

- [AGENT-PROMPTS](prompt-templates.md)、[DATA-MODEL](../30-data-model/data-model.md)、[DS-TOKENS](../60-design-system/design-tokens.md)
