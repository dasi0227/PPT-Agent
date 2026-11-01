# 左栏目录：结构模型与视觉重构设计

## 适用范围

本设计覆盖左栏目录（[DeckNavigator.tsx](../../frontend/src/features/deck/DeckNavigator.tsx)）的两个层面：

1. 结构模型：section / subsection / slide 的挂载规则、编号、折叠、人工增删。
2. 视觉呈现：编号权重、层级视觉、选中态。

实现分两期（见"交付分期"）。第一期为纯前端视觉重构，随本设计落地；第二期为结构模型与人工增删，需前后端改动，单独规划。

本设计在"页面挂载规则"上**取代** [2026-08-07-deck-navigator-redesign-design.md](2026-08-07-deck-navigator-redesign-design.md) 第 95 行"页面允许没有 subsection，是该 section 的直属页"这一条。其余（缩略图、移除机器字段、结构化 restructure 接口）继续有效。

## 背景

左栏目录当前呈现三套割裂编号（section `01`、subsection `3.2`、页码 `02`），选中态叠加四种强调信号，且数据模型允许 section 下同时存在直属页与 subsection，导致直属页夹在 subsection 标题之前的别扭排布。用户提出六点疑问，本设计逐一定论。

## 结构模型决策

### D1 挂载模型：严格两层

采用**严格两层**模型，替代当前的混合模型：

- section 是第一层，subsection 是第二层，slide 是叶子。
- 一个 section 处于两种互斥形态之一：
  - **直属形态**：没有任何 subsection，页面直接挂在 section 下。
  - **归组形态**：有一个或多个 subsection，页面必须挂在某个 subsection 下。
- **不允许**一个 section 同时拥有 subsection 和直属页。

### D2 对应六问的直接结论

| 问题 | 结论 |
|------|------|
| Q1 页面能否直挂 section | **能**，仅当该 section 无 subsection（直属形态）。 |
| Q2 有 subsection 时能否存在游离直属页 | **不能**。section 进入归组形态后，不存在直属页。 |
| Q3 subsection 是否支持独立折叠 | **不支持**。只有 section 级折叠。理由：目录树只应一层可折叠，多层手风琴的交互噪音大于收益。 |
| Q4 三套编号割裂 | **重排视觉权重**：页码为主角，结构编号为背景。见 D5。 |
| Q5 蓝色选中过重 | **降到两个信号**：左侧 accent 竖条 + 极浅底色，去掉文字变色与加粗。见 D6。 |
| Q6 人工增删 section/subsection | **支持**，作为轻量结构编辑，复用 restructure 快照协议，单独一期实现。见 D3。 |

### D3 人工增删 section/subsection

- 目录提供入口：section 区末尾"新增章节"，归组形态 section 内"新增子节"。
- 前端派生新 outline 结构后，通过 restructure 快照协议提交；后端返回权威 `{slides, spec}` 快照，前端原子应用。
- 需要后端扩展 outline 结构变更能力（当前 [ApplyPPTMutation](../../backend/internal/service/slide_content.go) 只改 `outline_order` 与 placement，不建 / 删 section/subsection）。
- Agent 写 outline 与用户手动编辑共享同一份 outline；活跃 run 期间禁用人工结构编辑（沿用现有 `runActive` 门控）。

### D4 校验与 Agent 约束（严格两层的落地保障）

- 后端 restructure 校验新增一条：当 placement 的 `section` 对应 section 拥有 subsection 时，该 placement **必须**带合法 `subsection`；否则拒绝。
- 反之，section 无 subsection 时，placement 不得带 `subsection`。
- Agent 写 outline 时遵守同一互斥规则：section 的 `subsections` 为空 ⟺ 其页面为直属页。
- 迁移：开发期不为历史混合数据保留兼容分支；若存在旧混合数据，由结构变更能力统一归位（具体归位策略在实现期确定）。

## 视觉决策

### D5 编号：阿拉伯层级，垂直对齐，靠权重区分

- 编号体系：section=`1 2 3`，subsection=`1.1 1.2`，页码=两位补零 `02 03`。三层均为阿拉伯数字。
- 垂直对齐：三层记号落在同一条最左记号栏，全部**左对齐**、宽度一致，不缩进、无竖向引导线。
- 靠**字号 / 字重 / 颜色**区分层级权重，而非缩进或颜色抢眼度：
  - 页码：最重（`text-text-600` 级、`font-semibold`），是导航主角。
  - section 号：中性中灰、`font-semibold`，不再使用 accent 蓝。
  - subsection 号：最浅（`text-text-400`）、小字。
- 去除 section 号补零（`1` 而非 `01`）。
- 视觉权重排序：页码 ≥ section > subsection。当前 section 号用 accent 蓝最抢眼，需扭转。

### D6 选中态：两个信号

- 保留：左侧 accent 竖条（定位锚点）。
- 保留但减弱：极浅底色（`bg-accent-soft` 调更淡或 `bg-accent/5` 量级）。
- 去掉：文字变 accent 色、`font-medium` 加粗。
- 目标：选中态表达"当前位置"，而非"高亮内容"，风格向 Keynote 缩略图选中的克制感靠拢。

## 交付分期

- **第一期（视觉，低风险，纯前端）＝本次落地**：D3 折叠维持现状 + D5 编号阿拉伯层级与垂直对齐 + D6 选中降噪。只改 [DeckNavigator.tsx](../../frontend/src/features/deck/DeckNavigator.tsx)，不动契约、不改后端。
- **第二期（结构模型，高风险，前后端）**：D1/D2 严格两层 + D4 校验与 Agent 约束 + D3 人工增删。动后端 restructure 校验、outline 写路径与 Agent outline 约束。

## 非目标

- 本次（第一期）不改动数据结构、后端校验与 outline 写路径；不实现人工增删入口。
- 不引入三层及以上 outline 结构。
- 不改变缩略图、页面删除、拖拽落点的既有交互。
- 不为历史混合结构数据设计兼容层（开发期原则）。

## 验收标准

1. section 只呈现两种形态之一（全直属 / 全归组），UI 与数据均不出现"subsection 与直属页混排"。
2. 后端 restructure 拒绝违反 D4 互斥规则的 placement。
3. 只有 section 级可折叠；subsection 恒平铺。
4. section=`1`、subsection=`1.1`、页码=`02` 三层记号左对齐、垂直对齐、不缩进无竖线；靠字号/字重/颜色区分权重，页码最重、section 号不再用 accent 蓝。
5. 选中态仅由左侧竖条 + 极浅底色表达，文字不变色不加粗。
6. 存在人工新增 / 删除 section 与 subsection 的入口，且经 restructure 快照协议提交并原子应用。（第二期）
7. 活跃 run 期间人工结构编辑被禁用。（第二期）
