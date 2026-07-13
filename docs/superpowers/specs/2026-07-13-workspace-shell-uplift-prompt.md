# 交付 Prompt — Workspace Shell Uplift (v6) 实施

> 直接把下面 `<<<PROMPT` 与 `PROMPT<<<` 之间的全部内容复制给下游 agent（另开一个会话）。
> 该 agent 拿到 prompt 即可独立完成实施，不需要读本次的 brainstorming 对话。

---

<<<PROMPT

# 角色

你是一位资深全栈开发工程师，负责落地 **PPT_Agent v6 Workspace Shell Uplift**。
你要独立完成一份完整设计文档所描述的所有前端 + 后端改动，产出可运行、可测试、可提交的代码。

# 项目背景

- 仓库根：`/Users/bytedance/Desktop/ByteDance/PPT_Agent`
- 前端：`frontend/`（React 18 + Vite + TS + Tailwind + Zustand + Vitest）
- 后端：`backend/`（Go + Gin + SQLite）
- 前端命令入口：`frontend/` 下 `npm run tsc`（类型检查）/ `npm run lint` / `npm test`（Vitest） / `npm run build`
- 后端命令入口：`backend/` 下 `go test ./...`
- 一键重启脚本：仓库根 `restart.sh`（后端与前端 dev server 通过 `.run/*.pid` 管理）
- 当前分支：`main`；最近 5 个提交前缀为 `feat(ux-consistency):`（本轮延续该前缀）

# 硬约束（MUST）

1. **命名统一为 v6**：本轮所有 zustand persist name、autoSaveId 等命名后缀统一 `-v6`。
2. **Commit prefix**：所有 commit 必须以 `feat(ux-consistency):` 开头。
3. **Pangu 排版**：中文与英文、数字之间要有半角空格。
4. **禁止修改用户已定稿的中文文案**：本 spec 中已给出的文案属于新写，可采用；对遗留代码里的中文提示，只在明确要改的模块内改。
5. **不引入未在 spec 中批准的第三方依赖**。批准清单：`@radix-ui/react-dialog`、`@radix-ui/react-dropdown-menu`、`react-resizable-panels`。zustand 已有，`persist` middleware 直接从 `zustand/middleware` 导入。
6. **不动**：LLM 超时配置、Thinking Bubble、History JSONL Schema `{seq, ts, run_id, turn, type, data}`、SSE 事件契约。
7. **不加**未在 spec 中出现的功能（YAGNI）。特别是：不做暗色模式、不做 File System Access API、不做面板上下拖拽。
8. **文件命名**：新增文件严格按照 spec §2 的文件清单落位；不要擅自换目录。
9. **测试驱动**：每个功能点先补 / 改测试，再改实现；提交前 `npm run tsc && npm run lint && npm test && (cd ../backend && go test ./...)` 必须全绿。

# 你的输入：设计文档

**唯一权威**：`docs/superpowers/specs/2026-07-13-workspace-shell-uplift-design.md`

请**先完整读一遍**这份 spec（约 900 行）。里面涵盖：
- 6 项需求（R1–R6）的行为、组件、接口、错误处理。
- §2 组件分层与文件清单。
- §5.7 后端 PATCH 接口签名、handler / service / store 三层代码骨架。
- §8 面板拖拽伸缩 + 隐藏 + 持久化实现细节。
- §9 关键数据流。
- §10 错误场景兜底。
- §11 完整 DoD 验收清单。
- §12 里程碑 M1–M7 拆解。

**如果 spec 与本 prompt 冲突**：以 spec 为准；`spec.md` 是唯一权威。若发现 spec 内部矛盾，先停下问用户。

# 实施顺序

严格按 spec §12 的 M1 → M7 顺序执行，每完成一个里程碑产出一个（或多个连续）commit。

- **M1 基础设施**：
  1. `cd frontend && npm i @radix-ui/react-dialog @radix-ui/react-dropdown-menu react-resizable-panels`
  2. 新建 `frontend/src/components/ui/` 五件套 + `lib/platform.ts`；每件都写单测。
  3. commit：`feat(ux-consistency): scaffold ui primitives (dialog / dropdown-menu / modal-*)`
- **M2 后端 rename**：
  1. `store/sqlite/run_store.go` 增 `UpdateProjectTitle` / `UpdateThreadTitle`
  2. `service/project.go` / `service/thread.go` 增 `RenameProject` / `RenameThread`
  3. `httpapi/project_handler.go` / `httpapi/thread_handler.go` 增 `Patch` handler
  4. `httpapi/router.go` 注册 `PATCH /projects/:id` / `PATCH /threads/:id`
  5. `httpapi/*_e2e_test.go` 补 rename 用例（成功 / 空 title / 超长 title / 不存在的 id）
  6. commit：`feat(ux-consistency): PATCH endpoints for renaming project & thread`
- **M3 前端 store 重构**：
  1. `stores/projectStore.ts`：去 draft，加 `openProjectIds` / `pendingNewProject` / `startPendingNewProject` / `finalizePendingNewProject` / `cancelPendingNewProject` / `renameProject` / `closeProject` / `openProject` / `createProject`；`loadProjects` 不再自动 select。
  2. `stores/threadStore.ts`：去 draft，加 `renameThread` / `createThread`；`ensureActiveThread` 简化。
  3. `stores/uiStore.ts`：加 `leftPanelHidden` / `rightPanelHidden`；接入 zustand `persist` middleware，name `ppt-agent-ui-v6`。
  4. 删除 `frontend/src/lib/draft.ts`，清理所有 `isDraftId` / `newDraftId` / `.draft` 引用。
  5. `api/types.ts`：移除 `Project.draft` / `Thread.draft`。
  6. `api/projects.ts` / `api/threads.ts`：新增 `patch()`。
  7. 改 / 删相关单测：`projectStore.test.ts` / `threadStore.test.ts` / `runStore.multithread.test.ts`。
  8. commit：`feat(ux-consistency): remove draft state, add project/thread rename in stores`
- **M4 布局 & 顶栏**：
  1. `AppShell.tsx`：用 `PanelGroup` + `Panel` + `PanelResizeHandle` 重写；`autoSaveId="workspace-shell-v6"`。
  2. `ProjectTabs.tsx`：改造为 `⋯ 菜单` + `PanelToggleButtons`；空态只显示 `+`。
  3. 新增 `PanelToggleButtons.tsx` / `WorkspaceEmptyState.tsx` / `NewProjectHint.tsx`。
  4. `PreviewWorkspace.tsx`：接入 `activeProjectId === null` → `WorkspaceEmptyState`；`activeProjectId === "new-pending"` → `NewProjectHint`。
  5. commit：`feat(ux-consistency): resizable + collapsible workspace panels`
- **M5 Picker / Menu / Rename / Delete Modal**：
  1. 新增 `ProjectPickerModal.tsx` / `OpenExistingProjectModal.tsx` / `ProjectMenu.tsx` / `ThreadMenu.tsx`。
  2. `ProjectTabs.tsx` / `ThreadTabs.tsx` 接入 `⋯` 菜单；移除旧的 `X` / `Trash2` 双按钮；`ThreadTabs` 里"历史"下拉迁移到 `DropdownMenu`。
  3. Rename / Delete 走 `modal-form` / `modal-confirm`；移除所有 `window.confirm`。
  4. `+` 按钮 → 打开 `ProjectPickerModal`。
  5. 全部 modal / dropdown 走 `components/ui/` 主题化包装。
  6. commit：`feat(ux-consistency): dropdown menus + rename & delete modals for project/thread`
  7. 补 commit：`feat(ux-consistency): project picker (new / open existing) modal flow`
- **M6 交付气泡 & 快捷键**：
  1. `CommandComposer.tsx`：`⌘/Ctrl + Enter` 发送；IME guard；提示条自适应。
  2. `eventReducer.ts`：`tool_call.tool === 'finish'` 打标 `hiddenFromTimeline: true`。
  3. `Timeline.tsx`：跳过 `hiddenFromTimeline` 的 tool_call；`final_result` 分流到 `FinalResultCard`（结构化）/ `FinishBubble`（非结构化）。
  4. 新增 `FinishBubble.tsx`。
  5. commit：`feat(ux-consistency): send on cmd/ctrl+enter + finish delivery as chat bubble`
- **M7 单测 & 验收**：
  1. 按 §11 DoD 逐项跑通：`npm run tsc && npm run lint && npm test`。
  2. `cd backend && go test ./...` 全绿。
  3. `bash restart.sh`（若脚本存在）启动 dev server；手动跑 Flow A / B / C（spec §6.5）。
  4. 如有 UI 回归，用 puppeteer 或人工截图记录。
  5. 若 `npm run build` 失败，修复类型 / lint 错误。
  6. 交付一份 PR 描述草稿，粘贴回聊天让用户 review。

# 修改边界（禁区）

- 不要修改 `restart.sh`、`.run/*`、`backend/internal/harness/**`（harness core，不在本轮范围）、`backend/internal/agent/**`、`backend/internal/llm/**`、`backend/internal/run/**`。
- 不要动 `frontend/src/features/agent/ThinkingBubble.tsx`、`historyHydrator.*`、`modeMapping.*`、`useActiveSession.ts`（除非 spec 明确要求）。
- 不要重构 unrelated 代码；发现散落的坏味道，记录到最终 PR 描述里，不本轮处理。

# 交付前自查

提交给用户前你必须自答：

- [ ] Spec §11 DoD 每一项打勾？（前后端 + 无障碍 + 测试 + 提交规范）
- [ ] `frontend/src/lib/draft.ts` 与所有引用是否清理干净？（`grep -r "draft" frontend/src` 应为空 或者只匹配到无关词根）
- [ ] `window.confirm` 是否已全部替换？（`grep -r "window.confirm" frontend/src` 应为 0 命中）
- [ ] `tool_call.tool === 'finish'` 的 UI 是否真的不再出现在 Timeline？
- [ ] 面板宽度与隐藏在刷新后保持？（手动跑一次验证 localStorage）
- [ ] `⌘+Enter`（mac）/ `Ctrl+Enter`（win/linux） 都能发送？IME 期间不发？
- [ ] 后端 e2e 覆盖 `PATCH` 4 种场景？
- [ ] 全部 commit 前缀 `feat(ux-consistency):`？

# 沟通规范

- 遇到 spec 中未覆盖的边界 → **停下问用户**，不要自作主张。
- 每完成一个里程碑，向用户简报一行进度：`M{N} 完成，提交 {sha_short}`。
- 遇到 lint / test 卡住超过 3 次尝试仍失败 → 停下并把错误粘给用户。
- 不使用 `git commit --no-verify` 跳 hook；不 `--force`；不 `reset --hard`。
- Repo 已存在的 uncommitted 未跟踪文件（`.run/*.log`, `.run/*.pid`, `restart.sh`）不要 `git add`、不要删除。

# 工作方式

1. 先 `cd /Users/bytedance/Desktop/ByteDance/PPT_Agent && git status && git log -5 --oneline` 掌握起始状态。
2. 完整读 spec：`Read docs/superpowers/specs/2026-07-13-workspace-shell-uplift-design.md`。
3. 使用 TodoWrite（或等效 task tool）把 M1–M7 立为任务，逐个 in_progress → completed。
4. 每完成一个 milestone → `git add <相关文件>` → `git commit` → 简报。

开始吧。

PROMPT<<<

---

## 使用说明（本文件对当前用户）

- 上面 `<<<PROMPT ... PROMPT<<<` 之间的内容就是交付 prompt。
- 交付时可以直接粘给下游 agent，或让下游 agent `Read` 这个 md 文件后提取 prompt 段。
- Spec 文件已 commit 前请先 `git add docs/superpowers/specs/2026-07-13-workspace-shell-uplift-design.md docs/superpowers/specs/2026-07-13-workspace-shell-uplift-prompt.md` 一起提交。
