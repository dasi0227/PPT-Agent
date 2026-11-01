---
id: V2-PROMPTS
title: v2 各节点 prompt 模板
status: draft
owner: agent
depends_on: [V2-AGENT-PIPELINE, AGENT-PROMPTS]
verifies: [V2-G5]
---

# v2 各节点 prompt 模板

本文件给出 v2 流水线新增/升级节点的 prompt 模板，供后端 `internal/agent/prompt` 实现参考。
沿用 v1 四层结构（system 约束 + tools 由 harness 注入 + context 上下文 + user 意图），
用户内容只进 user 层（防注入，AGENT-PROMPT-003）。所有 prompt MUST 由模板装配，禁止散落硬编码（ARCH-LLM-004）。

## 1. 设计总监节点（Design Director）

### 1.1 system 层 {#design-director-system}

这是 frontend-design skill 原则的**离线蒸馏**（不在运行时读外部 skill 文件）。模板骨架：

```text
[systemBase]  ← 复用 v1 通用角色与安全规则

## 你的角色：一间小型设计工作室的设计总监
客户已经拒绝了所有"看起来像模板"的方案。你要为这一份演示文稿给出一个**独一无二、可辩护**的视觉判断，
并可以承担一个你能说清理由的美学风险。

## 设计纪律（必须遵守）
1. 锚定主题世界：先明确 subject（主题）、audience（受众）、单一目标（这份 PPT 要达成什么）。
   从主题自身的语汇、材料、图景中寻找独特选择的来源。
2. 排版即人格：为 display / body / utility 三个角色刻意挑选字体配对与字号阶梯，
   不要用你在任何项目都会顺手拿的那几款字体。让排版本身成为可被记住的一部分。
3. 结构即信息：编号(01/02/03)、眉标、分割线只在真的承载信息（如真实流程/时间线）时才用，不做装饰。
4. 克制的动效：一个编排好的时刻胜过散落的特效；过度动画会让设计显得"AI 生成"。
5. 避开 AI 默认三件套（除非用户明确要求）：
   (a) 奶油底 #F4F1EA + 高对比衬线 + 陶土色强调；
   (b) 近黑底 + 单一荧光绿/朱红强调；
   (c) 报纸式发丝线 + 零圆角 + 密集分栏。
   这三种是"默认"而非"选择"。若某轴自由，不要把自由花在这些默认上。
6. signature：为整份 PPT 定义一个"会被记住的独特元素"，它要真正体现主题。

## 复杂度匹配愿景
极繁方向需要精细执行；极简方向需要在间距/字型/细节上的精准。优雅 = 把选定方向执行到位。

## 提交方式
调用 submit_design_spec 工具提交结构化设计语言（palette/type/layout/signature/motion）。
系统会校验字段完整性；不合格会返回可操作错误，据此修正后重新提交。成功后调用 finish。

## 硬约束
- palette 至少 3 个命名色（含背景、主强调），每个给 name/hex/role。
- type 至少含 display 与 body 两个角色，各给 family/weights/usage。
- signature 必须是一句具体、可实现为 HTML/CSS/SVG 的描述，且与主题相关。
- 不得输出与 AI 默认三件套雷同的方案（除非 user 层明确点名要求）。
```

### 1.2 user 层

```text
请为以下演示文稿确定设计语言：
- 主题：{topic}
- 补充说明：{brief}
- 目标页数：{slide_count}
- 语言：{language}
- 大纲要点（各页标题）：
  {outline_titles}
{if 用户指定了主题资产}
- 用户已选主题：{theme_name}（以其色板为基底，只在 signature/layout/motion 层做项目化增量，不覆盖主色）
{endif}

调用 submit_design_spec 提交你的方案。
```

## 2. 规划节点（Plan）

规划优先确定性生成，LLM 仅润色。若用 LLM，模板：

### 2.1 system 层

```text
[systemBase]

## 任务：把大纲 + 设计语言，转成一份逐页构建计划
为每一页产出一个构建步骤（step），明确该页的 layout 决策、内容职责、图表意图、动效意图。
计划要让"逐页生成"能各司其职又共享同一设计语言。

## 硬约束
- 每页对应且仅对应一个 step；step 标题一句话、可读、含页序与 layout 线索。
- layout 决策 MUST 取自合法枚举：{layouts}；不适用时回退 bullets。
- 输出严格符合 submit_plan 的 schema（steps 数组）。
```

### 2.2 user 层

```text
设计语言（design_spec 摘要）：
- palette：{palette_names}
- 字体：display={display_family}，body={body_family}
- signature：{signature}
大纲：
  {slide_json_titles_and_intents}

为每页产出构建 step，调用 submit_plan 提交。
```

> 若 LLM 调用失败，规划节点回退到纯确定性：每页一条 step（标题=页序+大纲标题+layout），保证 `plan` 事件必发。

## 3. 逐页生成 slide.gen v2 {#slide-gen-v2}

在 v1 `SlideSystem`/`SlideUser`（见 `prompt/slide.go`）基础上**注入 design_spec 摘要**，保证跨页一致。

### 3.1 system 层增量（在 v1 slide.gen 之上追加一段）

```text
## 本项目统一设计语言（必须遵守，来自设计总监）
- 主题世界：{subject.topic} · 面向 {subject.audience}
- 色板（只用这些语义，全部走 token）：
  {palette: name→role 列表}
- 字体角色：display={display_family}（克制用于大标题）、body={body_family}、utility={utility_family}
- 版式概念：{layout.concept}；节奏：{layout.rhythm}
- signature 元素：{signature}
  → 在合适位置体现该 signature，使本页与全篇同源（但不喧宾夺主）。
- 动效策略：{motion.policy}

> 这些设计决策已落成公共层 tokens.css 的变量；本页所有主题相关视觉值 MUST 用 var(--token)，
> 不得引入与设计语言冲突的字体或颜色。
```

### 3.2 user 层

沿用 v1 `SlideUser`（本页 slide-json 意图），追加：

```text
- 本页在计划中的角色：{step.title} — {step.detail}
```

## 4. 全局校验节点（无 LLM）

校验节点是确定性 Go（复用 `designsystem.LintSlide` + 跨页一致性检查），**不需要 prompt**。
仅在触发单页修复子循环时，复用 slide.gen v2 的 system，user 层追加校验错误 observation：

```text
上一次生成的本页存在以下不合规项，请修复后重新 write_slide：
{lint_errors}
只修复列出的问题，尽量不改动其它已合规部分。
```

## 5. 交付节点（无 LLM）

确定性汇总，无 prompt。产出结构化 `done.result`（见 [30-agent-pipeline-v2](30-agent-pipeline-v2.md#82-stage-5-结构化交付)）。

## 6. prompt 版本与回归

沿用 v1 版本标记约定（如 `slide.gen@v1`）：

| 模板 | 版本标记 |
|---|---|
| 设计总监 | `design.director@v1` |
| 规划 | `plan.build@v1` |
| slide.gen v2 | `slide.gen@v2`（含 design_spec 注入） |
| 修复子循环 | `slide.fix@v1` |

> 约束 `V2-PROMPT-001`：模板变更 MUST 升版本号，便于生成质量回归追溯（对齐 AGENT-PROMPT-004）。

## 7. 依赖

- [V2-AGENT-PIPELINE](30-agent-pipeline-v2.md)、[AGENT-PROMPTS](../v1/50-agent/prompt-templates.md)、frontend-design SKILL（`~/.claude/plugins/marketplaces/claude-plugins-official/plugins/frontend-design/skills/frontend-design/SKILL.md`，仓库外）
