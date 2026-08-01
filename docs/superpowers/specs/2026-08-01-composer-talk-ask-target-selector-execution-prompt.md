# Composer Talk、Ask 与目标选择器改造执行 Prompt

## 使用方式

将本文件作为本次前端修改的唯一需求来源。实施 Agent 必须先阅读当前代码，再自主规划并一次性完成实现、测试和交付，中途不询问普通实现细节。

## 任务背景

当前 Composer 将以下控件同时平铺：

- 蓝图 / 页面。
- 单页 / 整份。
- 讨论。
- 执行前确认。
- 物化整份。

这些控件实际属于三个不同维度：

1. 目标产物：`blueprint / presentation`。
2. 作用范围：`slide / deck`。
3. 交互方式：默认执行、`talk`、`ask`。

当前呈现方式层级不清晰、控件过多，而且“物化整份”与“HTML 页面 + 整份”重复。

本次需要将 Composer 重组为：

```text
┌──────────────────────────────────────────────┐
│ 描述你想制作或修改的内容                     │
│                                              │
│ [只讨论] [先讨论]       [当前页 HTML⌄] [发送] │
└──────────────────────────────────────────────┘
```

左侧是 `talk / ask` 状态按钮，右侧是目标选择器和发送按钮。

## 一、实施前检查

开始前至少阅读：

- `frontend/src/features/agent/CommandComposer.tsx`
- `frontend/src/features/agent/ModeSwitcher.tsx`
- `frontend/src/stores/composerStore.ts`
- `frontend/src/features/agent/modeMapping.ts`
- `frontend/src/api/types.ts`
- 上述模块相关测试

要求：

1. 检查 git 状态，不覆盖用户已有的无关修改。
2. 制定一次内部实施计划，然后连续执行到完成。
3. 修改前运行相关测试并记录基线。
4. 不要只输出分析或计划，必须完成实现和验证。

## 二、不可破坏的边界

1. 不修改顶部项目 Tab。
2. 不修改左、中、右三栏布局。
3. 不修改左右栏展开、收起、拖拽缩放和持久化。
4. 不修改 Thread Tabs 和 Timeline 结构。
5. 不改变 Composer 位于右栏底部的位置。
6. 不重构 Agent Runtime。
7. 不新增 kind。
8. 不删除 `/talk` 和 `/ask` 快捷指令。
9. 不改变现有 `WorkSpec`、`RunTarget` 和 `RunInteraction` 协议。
10. 不引入新的 UI 框架、图标库或状态管理库。
11. 不重新加入“物化整份”的其他同义快捷按钮。
12. 不做与 Composer 无关的大范围视觉修改。

## 三、重构 Talk 与 Ask 的呈现

删除当前 ModeSwitcher 中独立分散的“讨论”和“执行前确认”呈现方式。

在输入框底部操作栏最左侧增加两个状态按钮：

1. `只讨论`
2. `先讨论`

### 3.1 按钮视觉

- 图标在左，文本在右。
- 使用现有 Lucide 图标库。
- “只讨论”建议使用 `MessageCircle` 或语义相近的交流图标。
- “先讨论”建议使用 `MessageCircleQuestion`、`MessagesSquare` 或语义相近的需求澄清图标。
- 图标大小为 14px 到 16px。
- 未选中时使用透明背景、弱边框和次级文字。
- hover 时使用轻微中性背景。
- 选中时使用 `accent-soft` 背景、accent 图标和文字。
- 不使用大面积实心蓝色。
- 视觉形式参考 Codex 中“替我审批”一类状态启用按钮。
- 按钮具有 `aria-pressed`、中文 `aria-label` 和 tooltip。
- 两个按钮必须互斥。
- 再次点击已选中的按钮，恢复默认自动执行状态。

### 3.2 协议映射

两个按钮均未选中时：

```json
{
  "intent": "apply",
  "clarification": "when_blocked"
}
```

含义：

- Agent 默认直接执行。
- 只有无法安全继续时才询问用户。

选择“只讨论”：

```json
{
  "intent": "consult",
  "clarification": "when_blocked"
}
```

对应之前的 `/talk`：

- 基于当前项目进行只读分析。
- 可以解释、评价、回答问题和给建议。
- 不允许修改蓝图或 HTML。
- tooltip 文案建议为：“只分析和交流，不修改项目内容”。

选择“先讨论”：

```json
{
  "intent": "apply",
  "clarification": "before_apply"
}
```

对应之前的 `/ask`：

- 进入设计前讨论和执行前确认路径。
- 先围绕需求进行澄清、分析和方案设计。
- 形成执行方案并等待用户确认后，再进入修改。
- 产品语义参考 brainstorming / grilling，但本次不重构 Agent Runtime。
- 前端继续使用现有 `before_apply` 协议。
- tooltip 文案建议为：“先澄清需求并形成方案，确认后再执行修改”。

必须保留 `/talk` 和 `/ask` 文本快捷指令及其现有映射。

### 3.3 切换规则

```text
默认执行
  点击只讨论 -> talk
  点击先讨论 -> ask

talk
  再次点击只讨论 -> 默认执行
  点击先讨论 -> ask

ask
  再次点击先讨论 -> 默认执行
  点击只讨论 -> talk
```

不要在 Store 中增加第三套重复的 mode 字段。显示状态必须由 `intent + clarification` 派生。

## 四、删除“物化整份”

彻底删除 Composer 中的“物化整份”按钮。

同时清理：

- `WandSparkles` 等不再使用的 import。
- `submit(true)` 或 `deckMaterialization` 相关特殊分支。
- 空指令下自动发送“基于当前蓝图物化整份 HTML 演示”的逻辑。
- 与该按钮有关的测试、文案和死代码。

“物化整份”与以下目标组合完全等价：

```text
整份 HTML
= artifact: presentation
+ level: deck
```

用户应通过目标选择器表达该意图。

## 五、将产物和范围合并为目标选择器

删除当前两组始终展开的 Segment：

- 蓝图 / 页面。
- 单页 / 整份。

改成一个紧凑目标选择器，放在发送按钮正左侧。

触发按钮形式参考 Codex 模型选择器：

```text
当前页 HTML  ˅
```

### 5.1 触发按钮

- 使用中性或透明背景。
- 左侧显示当前目标名称。
- 右侧显示 `ChevronDown`。
- 高度与发送按钮所在操作栏协调。
- 不使用蓝色实心底。
- hover 使用轻量中性背景。
- focus-visible 使用克制的可访问性样式。
- 不抢过发送按钮的视觉权重。

### 5.2 菜单形式

点击后打开紧凑的 DropdownMenu 或 Popover，不使用 Dialog。

优先复用项目现有 Radix DropdownMenu 封装，不增加依赖。

菜单直接提供四个业务目标，不再要求用户分别理解 artifact 和 level：

#### 当前页 HTML

```json
{
  "artifact": "presentation",
  "level": "slide"
}
```

- 携带当前 `slide_id`。
- 描述：“生成或修改当前页面的 HTML”。

#### 整份 HTML

```json
{
  "artifact": "presentation",
  "level": "deck"
}
```

- 描述：“生成或统一修改整份演示”。

#### 当前页蓝图

```json
{
  "artifact": "blueprint",
  "level": "slide"
}
```

- 携带当前 `slide_id`。
- 描述：“调整当前页的标题、内容和视觉意图”。

#### 整份蓝图

```json
{
  "artifact": "blueprint",
  "level": "deck"
}
```

- 描述：“设计整份演示的结构和内容蓝图”。

### 5.3 菜单视觉

- 推荐宽度为 260px 到 300px。
- 每个选项包含主标题和一行简短描述。
- 当前选项显示 Check 图标。
- HTML 与蓝图可以使用分组标题或分割线。
- 不使用嵌套的第二层菜单。
- 菜单与触发按钮右对齐，避免超出右栏。
- 支持键盘上下选择、Enter 确认和 Escape 关闭。

只有四个目标组合，直接选择四项比多级菜单更高效。

### 5.4 默认值

- 有页面时默认“当前页 HTML”。
- 没有页面时默认“整份蓝图”。
- 不存在当前页面时，“当前页 HTML”和“当前页蓝图”禁用，或者自动回落到对应的整份目标。
- 切换项目时恢复合理默认值。
- 用户手动选择后，不得被普通数据刷新覆盖。

Composer Store 内部仍然只保存：

```ts
artifact
level
```

不要额外保存重复的 `selectedTarget`。目标选择器文案由 `artifact + level` 派生。

## 六、Composer 底部布局

输入框下方只保留一行操作栏。

左侧：

```text
[只讨论] [先讨论]
```

右侧：

```text
[当前页 HTML⌄] [发送]
```

要求：

- 使用 `justify-between` 将左右两组分开。
- 空间不足时优先压缩目标选择器文本，不压缩发送按钮。
- talk、ask 与目标选择器不得换成多行。
- 在常见右栏宽度和右栏最小宽度下保持单行。
- 不改变 Composer 在右栏底部的位置。

删除当前单独显示的：

```text
本次作用于 当前页 HTML | 自动执行 | 仅阻塞时询问
```

目标选择器与 talk / ask 状态已经能够表达完整状态，继续保留会造成信息重复。

## 七、去除输入框聚焦边框

移除输入框聚焦后明显的蓝色边框或外环。

要求：

- 聚焦前后 Composer 外层边框颜色保持一致。
- 删除 `focus-within:ring-*`、动态 accent border、蓝色 outline 或等价样式。
- textarea 保持 `focus:outline-none`。
- 不出现整圈高亮蓝框。
- 可以通过输入光标、轻微背景变化或克制阴影表示正在输入。
- 不允许使用明显的聚焦边框变化。
- 操作按钮仍然保留 `focus-visible`，不得全局关闭键盘焦点。

## 八、组件边界

建议将原 ModeSwitcher 拆分或重命名为：

```text
InteractionModeButtons
TargetSelector
```

`InteractionModeButtons` 负责：

- 默认执行。
- talk。
- ask。
- 互斥切换。
- 协议字段映射。

`TargetSelector` 负责：

- 四种目标组合。
- 菜单展示。
- 当前选择文案。
- `slide_id` 可用性。

`CommandComposer` 负责：

- 输入内容。
- 组合请求。
- 提交状态。
- 排列子组件。

不要为了本次修改重构其他 Timeline 组件。

## 九、测试要求

必须更新或新增测试，覆盖：

1. 默认状态下两个按钮均未选中。
2. 默认请求为 `apply + when_blocked`。
3. 点击“只讨论”后映射为 `consult`。
4. 再次点击“只讨论”恢复默认执行。
5. 点击“先讨论”后映射为 `apply + before_apply`。
6. talk 和 ask 不能同时选中。
7. talk 与 ask 相互切换时协议正确。
8. `/talk` 与 `/ask` 快捷指令继续有效。
9. 目标选择器正确展示当前组合。
10. 四个目标选项映射正确。
11. 当前页目标正确携带 `slide_id`。
12. 没有当前页时正确禁用或回落。
13. 页面中不存在“物化整份”按钮。
14. 页面中不存在重复的“本次作用于……”状态行。
15. 输入框聚焦后不会出现蓝色外框。
16. DropdownMenu 支持键盘操作。
17. 右栏最小宽度下操作栏不换行、不溢出。
18. 中文输入法和 `Cmd/Ctrl + Enter` 继续有效。
19. 创建 Run 失败时输入内容不会丢失。
20. 现有前端测试保持通过。

## 十、验证

必须运行：

```bash
cd frontend
pnpm test
pnpm tsc
pnpm lint
pnpm build
```

如本地服务可启动，还需要检查：

- 正常右栏宽度。
- 右栏最小宽度。
- 1440 x 900。
- 1280 x 800。

重点检查：

- talk / ask 是否清楚但不过度抢眼。
- 目标选择器是否紧邻发送按钮。
- 菜单是否不会超出右栏。
- 输入框聚焦后是否没有蓝色边框。
- 底部操作栏是否保持单行。
- “物化整份”是否已经完全删除。
- 默认自动执行是否不需要额外按钮。

## 十一、完成汇报

完成后一次性汇报：

1. talk / ask 的最终交互和字段映射。
2. 目标选择器的四个选项。
3. 删除的旧逻辑和死代码。
4. 主要修改文件。
5. 测试、类型检查、lint 和构建结果。
6. 正常宽度与最小右栏宽度的验收结果。

不要只输出分析或计划。直接完成实现、测试、浏览器验收和最终交付。
