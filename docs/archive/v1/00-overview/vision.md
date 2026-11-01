---
id: OVERVIEW-VISION
title: 产品愿景
status: approved
owner: shared
depends_on: []
verifies: []
---

# 产品愿景

## 一句话

一个**专注 frontend 的 PPT Coding Agent**：用户用自然语言驱动，Agent 用纯前端技术（HTML/CSS/JS）生成、编辑、在线预览 PPT。

## 我们在做什么

把「写一份漂亮的 PPT」变成「和一个懂前端、懂设计的 Agent 对话」。用户不需要懂 CSS、不需要打开 PowerPoint：
输入一个主题 → Agent 产出大纲 → 套用设计系统生成整套 HTML slides → 用户用自然语言或自定义指令逐页打磨 → 在线预览翻页。

## 为什么用前端技术做 PPT

- **表现力**：CSS 动画、Canvas 特效、SVG 图表远超传统 PPT 的能力上限。
- **零依赖可移植**：产出是 HTML/CSS/JS，可在任意浏览器打开、可部署成链接、十年后仍能渲染。
- **可被 Agent 编辑**：HTML 是文本，天然适合 LLM 读写与精确局部修改。
- **token 驱动换肤**：一套 design tokens + 公共 CSS 层，换一处即全局换肤——这正是 `/overview` 指令的根基。

## 它「不是」什么

- **不是通用 Coding Agent**：它只做 PPT/slide 这一件事，所有 prompt、上下文、工具都为此收敛。
- **不是模板填充器**：不是把内容塞进固定模板，而是按设计系统**生成**定制化前端。
- **不是 PowerPoint 的在线克隆**：不追求 PPTX 兼容（PPTX 转换列入 backlog）。

## 区别于「通用 Agent + SKILL」的定位

业界已有把「做 HTML PPT」封装成 SKILL 交给通用 Agent 的方案（如 frontend-slides、html-ppt-skill）。
本项目把这种**能力目标**沉淀为一个**专用产品**：自带后端、持久化、在线预览、版本、个人插件仓库与一套为 PPT 收敛的指令系统与 HITL 交互。
参考项目验证了「token 驱动设计系统 + iframe 隔离预览 + 可拼装资产」这条路是对的，我们将其产品化。

## 成功标准（北极星）

- 一个非设计师用户，从输入主题到获得一份**可直接演示**的 PPT，全程只靠自然语言 + 少量指令。
- 任意单页可被精确局部编辑而不破坏其它页（`/page`）。
- 全局风格可被一次性统一调整（`/overview`）。

## 依赖

无。
