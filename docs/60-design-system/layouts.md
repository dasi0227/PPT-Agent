---
id: DS-LAYOUTS
title: 版式库目录
status: approved
owner: shared
depends_on: [DS-TOKENS, DATA-SLIDE]
verifies: []
---

# 版式库目录

版式（layout）= 单页的结构模板。slide-json 的 `layout` 字段从本目录枚举取值（与
[slide-json.schema.json](../30-data-model/slide-json.schema.json) 的 enum 保持一致）。

## 版式清单

| layout | 用途 | 关键区域 |
|---|---|---|
| `cover` | 封面 | 大标题、副标题、作者/日期 |
| `toc` | 目录 | 章节列表 |
| `section-divider` | 章节分隔 | 大号章节标题 |
| `bullets` | 要点列表 | 标题 + 项目符号 |
| `two-column` | 两栏 | 左右内容块 |
| `three-column` | 三栏 | 三内容块 |
| `big-quote` | 大引用 | 引文 + 出处 |
| `stat-highlight` | 关键数字 | 巨大数字 + 说明 |
| `kpi-grid` | KPI 网格 | 多指标卡片 |
| `table` | 表格 | 数据表 |
| `code` | 代码 | 高亮代码块 |
| `diff` | 代码差异 | 增删对比 |
| `terminal` | 终端 | 命令行风格 |
| `flow-diagram` | 流程图 | 节点 + 箭头 |
| `timeline` | 时间线 | 时序事件 |
| `roadmap` | 路线图 | 阶段规划 |
| `mindmap` | 思维导图 | 中心 + 分支 |
| `comparison` | 对比 | A vs B |
| `pros-cons` | 优劣 | 正反两列 |
| `image-hero` | 大图主视觉 | 全幅图 + 叠字 |
| `image-grid` | 图组 | 多图网格 |
| `chart` | 图表 | 由 chart_intent 决定（见 charts） |
| `arch-diagram` | 架构图 | 分层/组件框图 |
| `process-steps` | 步骤 | 编号步骤 |
| `cta` | 行动号召 | 结尾引导 |
| `thanks` | 致谢 | 结束页 |

## 约定

| ID | 约束 |
|---|---|
| `DS-LAYOUTS-001` | layout 枚举 MUST 与 slide-json schema 的 `layout` enum 完全一致 |
| `DS-LAYOUTS-002` | 每个版式 MUST 在 16:9 舞台内自适应，引用公共层 token |
| `DS-LAYOUTS-003` | 生成遇未知/不适用 layout MUST 回退到 `bullets`（[SPEC-GEN-003](../10-spec/feat-slide-generation.md)） |
| `DS-LAYOUTS-004` | 新增版式 MUST 同步更新本表与 schema enum |

## 验收标准（Given-When-Then）

- **AC-LAYOUTS-001**（`DS-LAYOUTS-001`）
  - GIVEN 本表 layout 集合与 schema enum
  - WHEN 对比
  - THEN 两者完全一致（无遗漏/多余）

## 校验方式

```bash
node scripts/check-layout-enum.mjs   # 比对本表与 slide-json.schema.json enum
```

## 依赖

- [DS-TOKENS](design-tokens.md)、[DATA-SLIDE](../30-data-model/data-model.md)
