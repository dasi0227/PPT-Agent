---
id: OVERVIEW-FLOWS
title: 用户画像与核心流程
status: approved
owner: shared
depends_on: [OVERVIEW-VISION, OVERVIEW-GLOSSARY]
verifies: []
---

# 用户画像与核心流程

## 用户画像

| 画像 | 描述 | 核心诉求 |
|---|---|---|
| 非设计师内容创作者 | 有内容、无设计/前端能力 | 输入主题就能得到漂亮、可演示的 PPT |
| 技术分享者 | 会写一点前端，想要代码块/架构图/终端风 | 精确控制单页、复用自己收藏的特效 |
| 高频复用者 | 反复做相似风格的 PPT | 个人仓库沉淀样式/特效作为插件 |

## 核心流程（Happy Path）

```
输入主题
  │
  ▼
[大纲生成] 主题 → slide-json[]   ◄── 可 /talk 对齐、/ask 澄清
  │
  ▼
[风格选择] 从预置主题中选 / Agent 推荐（Show-don't-tell）
  │
  ▼
[整套生成] slide-json[] + 设计系统 → slide html[]   ◄── SSE 流式 + 进度
  │
  ▼
[在线预览] 上下页 / 步骤 / 缩放总览 / 点击跳转
  │
  ▼
[引导编辑] 自然语言 + 指令逐页打磨
  ├── /page x      锁定单页编辑
  ├── /overview    改公共样式层（全局）
  ├── /prompt      改写用户输入
  ├── /recap       回顾进度
  ├── /talk        只说不做（对齐颗粒度）
  └── /ask         必须提问（对齐颗粒度）
  │
  ▼
[沉淀] 收藏满意的样式/特效 → 个人仓库插件
```

## 关键交互时刻（Human-in-the-loop）

- **生成中途注入**：整套生成过程中，用户可随时 `POST /runs/{id}/input` 追加要求（如「第 3 页改成深色」），Agent 在下一个 checkpoint 消费。
- **Agent 主动提问**：`/ask` 模式或遇到不确定时，Agent 发 `needs_input` 暂停，等用户回答后继续。
- **只说不做**：`/talk` 模式下 Agent 只输出分析，不改任何文件，用于对齐颗粒度。

## 流程对应的规格

| 流程节点 | 规格文档 |
|---|---|
| 大纲生成 | [feat-outline-generation](../10-spec/feat-outline-generation.md) |
| 整套生成 | [feat-slide-generation](../10-spec/feat-slide-generation.md) |
| 在线预览 | [feat-online-viewer](../10-spec/feat-online-viewer.md) |
| 引导编辑 | [feat-nl-editing](../10-spec/feat-nl-editing.md) |
| 指令 | [50-agent/commands](../50-agent/commands/) |
| 个人仓库 | [feat-personal-repo](../10-spec/feat-personal-repo.md) |

## 依赖

- [OVERVIEW-VISION](vision.md)
- [OVERVIEW-GLOSSARY](glossary.md)
