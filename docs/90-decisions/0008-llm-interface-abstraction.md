---
id: ADR-0008
title: LLM 客户端 interface 抽象，假设 DeepSeek function calling 良好
status: accepted
owner: backend
depends_on: [ADR-0007]
verifies: []
---

# ADR-0008：LLM 客户端 interface 抽象，假设 DeepSeek function calling 良好

## 背景

Harness 重度依赖 function calling。DeepSeek 的 function calling 稳定性历史上弱于 GPT/Claude。
用户指示：当前**假设 DeepSeek 表现良好**，不做复杂容错；多 LLM 供应商切换后续再考虑。
同时项目定位「严格专用」于 PPT 业务。

## 决策

- LLM 客户端以 **interface 抽象**（`Chat`/`Stream`/`CallTool`），当前仅 DeepSeek 一个实现。
- **不做** function call 智能纠偏（JSON 修复/重提示）。
- **保留最小失败退出**：function call 无法解析时，记录 → 发 `error` → Run 转 `failed`，不进入重试死循环。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| interface 抽象 + 最小容错（选中） | 不焊死 DeepSeek、不过度设计、循环不卡死 | 坏返回会失败（可接受，后续增强） |
| 焊死 DeepSeek 具体类型 | 最省事 | 未来换/加供应商需大改 harness |
| 现在就做完整多供应商 + 容错 | 健壮 | 过度设计，违背用户指示 |

## 后果

- 「严格专用」约束 PPT 业务，不约束 LLM 厂商——未来加 OpenAI/Claude 仅新增实现类。
- 见 [llm-integration](../20-architecture/llm-integration.md) 的 `ARCH-LLM-FC-001/002`。
- 高级容错与多供应商切换列入 backlog。

## 状态

accepted（2026-06-30）。
