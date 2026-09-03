# 输入框触发符迁移设计

## 背景

Agent 输入框（`PromptComposerEditor`）支持以特殊字符唤起快捷菜单。当前映射为：

| 功能 | 当前触发符 |
| --- | --- |
| 提示词 Prompt | `$`（兼容 `¥`） |
| 组件 Component | `#` |
| 页面 Page | `@` |
| 汇总 Summary（三列面板） | `/` |

> 说明：用户记忆中的「¥ 提示词」实际上是 `$` 的输入法兼容字符——中文输入法下敲 `$` 常被自动转成 `¥`，故 `findPromptTrigger` 用 `[$¥]` 同时接受两者。

现需为「命令」预留 `/`。命令通常由 `/` 唤起，因此整体迁移一次触发符：

| 功能 | 新触发符 | 备注 |
| --- | --- | --- |
| 提示词 Prompt | `%`（兼容全角 `％`） | 半角/全角均触发 |
| 组件 Component | `¥`（兼容 `$`） | 沿用原提示词的输入法双字符容错 |
| 页面 Page | `#` | |
| 汇总 Summary | `@` | 三列面板内容不变 |
| 命令 Command | `/` | **暂不绑定任何效果，完全静默**，后期实现 |

## 需求决策（已与用户确认）

1. **`$` 归组件**：`$` 与 `¥` 继续绑定为一组，一起指向「组件」。中文输入法下敲 `$` 变 `¥` 仍能唤起组件。
2. **`%` 兼容全角**：提示词同时接受半角 `%` 与全角 `％`，与现有 `[$¥]` 的处理手法一致。
3. **停用现有斯杠命令**：当前 `CommandComposer` 里的 `/talk`、`/ask`、`/plan`、`/overview`、`/current`（提交时改 `mode`/`scope`）一并停用，让 `/` 彻底空出给后期命令体系。
4. **`/` 完全静默**：现阶段敲 `/` 与普通字符无异，不弹菜单也不提示。
5. **汇总面板内容不变**：`@` 唤起的三列面板仍是「页面 / 组件 / 提示词」三列，仅更换唤起符号，内部各列复用原有 matcher。

## 现状实现要点

触发符**没有集中常量**，各 `findXxxTrigger` 用正则内联字面量匹配，全部位于 [promptMatching.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/promptMatching.ts)：

- [findPromptTrigger](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/promptMatching.ts#L36-L46)：`/(?:^|[ \n])([$¥])([^ \n$¥]*)$/`
- [findComponentTrigger](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/promptMatching.ts#L48-L58)：`/(?:^|[ \n])(#)([^ \n#]*)$/`
- [findPageTrigger](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/promptMatching.ts#L60-L70)：`/(?:^|[ \n])(@)([^ \n@]*)$/`
- [findSlashTrigger](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/promptMatching.ts#L72-L82)：`/(?:^|[ \n])(\/)([^ \n/]*)$/`

触发符的消费在 [PromptComposerEditor.tsx](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/PromptComposerEditor.tsx)：`updateTrigger`（L225-268）依次调用四个 finder，并按 `prompt > component > page > slash` 决定当前触发类型。

斯杠命令在 [CommandComposer.tsx](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/CommandComposer.tsx)：`applyShortcut`（L136-154）解析 `/talk` 等命令，调用点在 L375。它与编辑器的 `findSlashTrigger`（汇总面板）无关，只是共用 `/` 前缀。

> 结论：本次迁移仅需改动正则字面量与斯杠命令；后端字段 `component_names` / `mentioned_slide_ids` 与触发符无关，无需改动；也不存在向用户展示「符号→功能」的图例文案需要同步（placeholder 仅为「输入你的想法与目标」）。

## 详细方案

### 1. 触发匹配（promptMatching.ts）

按新映射调整四条正则的字符类。为可维护性，建议将四个触发字符抽为文件级常量，正则由常量拼装，避免字面量继续散落（可选优化，若不做则直接改字面量）。

| finder | 现正则字符类 | 新正则字符类 | 对应功能 |
| --- | --- | --- | --- |
| `findPromptTrigger` | `[$¥]` | `[%％]` | 提示词 |
| `findComponentTrigger` | `#` | `[¥$]` | 组件（含输入法容错） |
| `findPageTrigger` | `@` | `#` | 页面 |
| `findSlashTrigger` | `\/` | `@` | 汇总面板 |

注意正则的排除字符集也要同步：例如 `findPromptTrigger` 的 `([^ \n$¥]*)` 需改成 `([^ \n%％]*)`，`findComponentTrigger` 的 `([^ \n#]*)` 改成 `([^ \n¥$]*)`，`findPageTrigger` 的 `([^ \n@]*)` 改成 `([^ \n#]*)`，`findSlashTrigger` 的 `([^ \n/]*)` 改成 `([^ \n@]*)`。排除集保证连续同符号（如 `##`）不会被当作 query 内容。

迁移后不再有任何 finder 匹配 `/`，因此 `/` 自然完全静默，无需额外处理。

### 2. 命名（可选但推荐）

`findSlashTrigger` / `kind: 'slash'` 在迁移后语义变为「汇总（`@`）」，名称成为误导。推荐同步重命名为 `findSummaryTrigger` / `kind: 'summary'`，涉及：

- [promptMatching.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/promptMatching.ts)：`findSlashTrigger`、`SlashTrigger` 类型。
- [PromptComposerEditor.tsx](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/PromptComposerEditor.tsx)：`ComposerTrigger` 的 `kind` 联合类型（L44）、`SLASH_COLUMNS`（L46-50）、`updateTrigger` 中的分支（L237-249）、键盘处理 `handleEditorKeyDown` 中 `trigger.kind === 'slash'` 分支（L490-524）、`applySlashActive`（L472-480）、`slashCol`/`slashRow`/`slashColumns`/`slashCellData`/`slashColumnEmptyText` 等一系列 `slash*` 命名，以及菜单渲染分支（L598-671）。

若为控制改动面，可保留 `slash` 命名仅改正则；文档标注该命名此后指代「汇总」。**推荐做重命名**以符合项目整洁约定，但需在同一次提交内一致切换。

### 3. 触发优先级（PromptComposerEditor.tsx L241-249）

新触发字符互不重叠（`% ％` / `¥ $` / `#` / `@`），优先级链在功能上不再产生冲突。保持现有链式判断即可；若做了重命名，将 `slashTrigger` 分支改名为 `summaryTrigger` 并置于链尾。

### 4. 停用斯杠命令（CommandComposer.tsx）

- 删除 `applyShortcut` 函数（L136-154）。
- 删除调用点 `request = applyShortcut(raw, request);`（L375）。

删除后，以 `/` 开头的输入将作为普通 `instruction` 正文原样提交，不再改写 `mode`/`scope`。这与「`/` 完全静默、留给后期命令」一致。

### 5. 汇总面板（无需改内容）

`SLASH_COLUMNS`（[PromptComposerEditor.tsx L46-50](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/PromptComposerEditor.tsx#L46-L50)）仍为「页面 / 组件 / 提示词」三列，`applyPage` / `applyComponent` / `applyPrompt` 的分派逻辑不变。仅唤起符号从 `/` 变为 `@`（由 `findSummaryTrigger` 承接）。列头文案、`aria-label="汇总检索候选"`、空列「暂无配置」提示均保持不变。

## 影响面与不改动项

- **不改动**：后端全部逻辑；`component_names` / `mentioned_slide_ids` 字段；页面片段序列化格式 `Page N · Title⟨slide_id⟩`；彩色片段样式（`promptComposer.css`）；三列面板结构与交互。
- **改动**：`promptMatching.ts` 四条正则；`CommandComposer.tsx` 删除斯杠命令；（可选）`slash → summary` 重命名。

## 测试调整

- [promptMatching.test.ts](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/promptMatching.test.ts)
  - prompt matching：把用例中的 `$` / `¥` 改为 `%` / `％`，并新增全角 `％` 触发用例。
  - component matching：`#` 用例改为 `¥` / `$`，覆盖输入法双字符容错。
  - page matching：`@` 用例改为 `#`。
  - slash trigger：`/` 用例改为 `@`（若重命名，`describe` 名同步为 summary trigger），并新增「`/` 不再触发任何菜单」的用例。
- [PromptComposerEditor.test.tsx](file:///Users/bytedance/Desktop/ByteDance/PPT_Agent/frontend/src/features/agent/PromptComposerEditor.test.tsx)
  - 「opens an empty configuration menu for @, #, $, and ¥」（L212-229）：遍历数组更新为 `[['#','页面'],['¥','组件'],['$','组件'],['%','提示词'],['％','提示词']]`。
  - 组件插入用例：触发符 `#` → `¥`。
  - 页面插入/重排/首字符用例：触发符 `@` → `#`。
  - 三列面板用例（L247-262）：唤起符 `/` → `@`。
  - 新增：敲 `/` 不弹任何菜单的断言。
- 后端测试（`page_mention_test.go`、`run_command_test.go` 等）无需改动。
- 若删除斯杠命令，检查并移除 `CommandComposer` 相关测试中对 `/talk` 等命令的断言（如存在）。

## 验证

- `cd frontend && npm run typecheck`（或项目既有 TS 检查命令）。
- `cd frontend && npx vitest run src/features/agent/promptMatching.test.ts src/features/agent/PromptComposerEditor.test.tsx`。
- 手动核对：`%`/`％` 唤起提示词、`¥`/`$` 唤起组件、`#` 唤起页面、`@` 唤起三列汇总面板、`/` 无任何反应且原样进入正文。
