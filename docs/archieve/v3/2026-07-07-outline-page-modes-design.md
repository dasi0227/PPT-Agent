# 大纲 / 页面多模态区分与切换 — 设计文档

- **日期**：2026-07-07
- **状态**：设计已获用户逐节确认，待写实现计划
- **范围**：前端预览双视图 + 大纲编辑（AI + 手动）+ 结构操作（加/删/重排）+ 页身份重构
- **关联**：`docs/v2/`（v2 增量事实源）、`docs/v1/`（v1 架构契约）

---

## 1. 背景与问题

当前产品（v2，M1–M5 已完成）存在一个体验断层：**大纲阶段是视觉黑洞**。

- `kind=outline` 生成后，磁盘上每页只有 `slide.json`（结构化内容意图），没有 `index.html`。
- 中间预览区靠 `hasSlides`（其实是靠 html_path）决定显示 iframe；只有 json 的页无从预览。
- "整套生成"（把大纲逐页渲染成 html + 建 `common/tokens.css` + `design-spec.json`）在前端**没有明确入口**。
- 后端 `GET /slides/{id}` 只返回元数据（title/layout/html_path），**前端拿不到 bullets/subtitle/content_intent/chart_intent**，无法美观呈现大纲。
- `kind=outline` 只做首次生成，**没有"改大纲"的能力**；有大纲时前端临时退化成 Overview 编辑。

用户目标：让**大纲**和**页面**成为可自由切换的两种呈现，每页都能在"大纲视图"与"HTML 视图"间切换；大纲可用 AI 或手动编辑；支持加/删/重排页；改了大纲后已生成的 html 有清晰的"待更新"提示。

## 2. 目标与非目标

### 目标
- G1：预览区每页可独立在**大纲视图 / HTML 视图**间切换，智能默认。
- G2：大纲内容可通过**AI 自然语言**与**手动就地编辑**两条路径修改（互斥并发）。
- G3：支持**加页 / 删页 / 重排序**（AI + 手动双入口），且磁盘零迁移。
- G4：改大纲后，已生成的 html 有 **outline_dirty 脏标记**，由用户显式重生成同步。
- G5：页身份用**稳定 slide_id 命名目录 + order 字段排序**，支撑结构操作。

### 非目标
- 不做 slide.json 的版本快照 / 回收站（html 产物版本沿用现有体系）。
- 不写旧数据迁移脚本（**开发期直接清库重来**）。
- 不改单机无鉴权定位；不改 SSE 帧格式与三层架构（Run→Harness→Tools）。

## 3. 核心决策（用户逐条确认）

| # | 决策点 | 选定 |
|---|---|---|
| 1 | 预览显示大纲/html 由什么决定 | **每页独立视图偏好** + 智能默认（有 html→HTML，仅 json→大纲） |
| 2 | 大纲怎么改 | **AI + 手动就地编辑并存**；AI 运行中禁止手动改 |
| 3 | 手动编辑保存方式 | **字段级即时 PATCH** |
| 4 | 改大纲后旧 html 处理 | **outline_dirty 脏标记 + 用户显式重生成**（不自动） |
| 5 | 第一版结构操作范围 | **支持加 / 删 / 重排序** |
| 6 | 页身份锚定 | **稳定 slide_id 命名目录 + 独立 order 字段** |
| 7 | 结构操作触发方式 | **AI + 手动 UI 双入口** |
| 8 | slide.json 编辑版本快照 | **不做**（大纲编辑不留版本） |
| 9 | 删页 | **二次确认，无回收站** |
| 10 | 现有数据 | **直接清库重来**，不写迁移脚本 |
| 11 | AI 删页 | **走 needs_input 二次确认**；加页/改字段/重排 AI 直接执行 |
| 12 | AI 大纲编辑粒度 | **支持单页与跨多页**操作 |

## 4. 详细设计

### 4.1 页身份重构（地基）

**数据模型**
- `slides.id`：稳定身份（已有 UUID），**用作磁盘目录名**。
- `slides.order`：新增列，展示顺序；采用**间隔分配**（0,10,20…），插入取中值，间隔耗尽时该 project 一次性重排规整（后台，用户无感）。
- `slides.idx`：**弃用作为路径与身份来源**。保留列以免破坏历史结构，但所有定位改走 id/order；新建页不再依赖 idx 计算路径。
- `slides.outline_dirty`：新增布尔列（见 4.4）。

**磁盘布局**
```
旧：slides/000/{slide.json,index.html}   versions/slide-000/vN.html
新：slides/<slide_id>/{slide.json,index.html}   versions/slide-<slide_id>/vN.html
    html 对 ../../common/tokens.css 的相对引用层级不变
```

**影响面（约 10 个生产文件，`%03d`/idx 拼路径改为按 slide_id）**
`generate/runner.go`、`generate/write_tool.go`、`edit/runner.go`、`edit/patch_tool.go`、`edit/read_tool.go`、`overview/fanout_tool.go`、`assetops/mount_tool.go`、`outline/submit_tool.go`、`service/slide.go`、`model/version_target.go`；DB 迁移新增 `order`/`outline_dirty` 列；相关测试同步。

**零迁移语义**
- 重排 = 只改 order 数字，磁盘不动。
- 加页 = 新建 `slides/<新id>/` + 分配 order，其它页不动。
- 删页 = 删 `slides/<该id>/` 目录，其它不动。

**前端不受影响的部分**：预览 `postMessage({type:'goto',index})` 与翻页仍按"已排序数组下标"工作；只有后端定位文件从 idx 改为 slide_id。

### 4.2 slide-json 读写契约

**读（扩展现有端点带 content）**
```
GET /slides/{id}
GET /projects/{id}/slides   （列表同样带 content，前端一次拿全）
响应新增：
  order, outline_dirty,
  content: { subtitle, bullets[], content_intent, chart_intent, steps, notes }
后端从 slides/<id>/slide.json 读出装配。
```

**写（字段级即时 PATCH）**
```
PATCH /slides/{id}
body: { title?, subtitle?, bullets?, content_intent?, chart_intent?, layout?, steps? }
语义：
  - 局部更新传入字段，回写 slide.json；若传了 title/layout 则同步 DB 对应列（order 不经此端点改，走 reorder）
  - 若该页有 html → 置 outline_dirty=true
  - project 有活跃 run → 409 RUN_ACTIVE
  - 走 project 锁串行化
  - 不产版本快照
```

**结构操作端点**
```
POST   /projects/{id}/slides            { after_slide_id?, layout? } 加页→分配 order，空白 slide.json
DELETE /slides/{id}                     删页（删目录+versions+DB 行）
POST   /projects/{id}/mutations    { ordered_ids:[...] } 批量重写 order
（均受 RUN_ACTIVE 互斥 + project 锁）
```

### 4.3 预览区双视图（每页独立）

**deckStore 扩展**
```
保留：previewMode: 'main' | 'overview' | 'single'
新增：viewByPage: Record<slideId, 'outline' | 'html'>
派生：effectiveView(slideId) =
        viewByPage[slideId]
        ?? (slide.html_path ? 'html' : 'outline')
```

**主视图工具栏**：右侧新增 `[大纲 | HTML]` 段控，只切当前页的 `viewByPage[slideId]`。当前页无 html 时"HTML"侧置灰不可选并提示"该页尚未生成"。切页按 effectiveView 决定，每页记忆独立。

**OutlineCard 组件**：把 slide.json 渲染成接近成品排版的 16:9 卡片——title（大字号）、subtitle、bullets 列表、chart_intent 占位说明、content_intent、steps 标记、layout 徽标、右上角 outline_dirty 徽标。与 HTML 视图共享同一 16:9 尺寸，切换不跳版。

**总览网格**：有 html 的页显示 html 缩略 iframe；无 html 的页显示**紧凑版 OutlineCard**（大纲缩略卡），避免空白格。

### 4.4 大纲编辑与脏标记

**两条路径（并发互斥）**
- 路径一 · AI：Outline 模式自然语言 → **新增大纲编辑 runner**（有大纲时走它，不再退化 Overview）。支持单页与跨多页。
- 路径二 · 手动：OutlineCard 就地编辑 → 失焦 `PATCH /slides/{id}`。
- 互斥：project 有活跃 run 时 OutlineCard 只读（灰显+锁提示）；手动 PATCH 撞活跃 run → 409 RUN_ACTIVE；run 结束自动解锁。

**outline_dirty（每页布尔）**
- 含义：slide.json 已改但 html 未同步。
- 置 true：任何改 slide.json 的操作（AI/手动）且该页当前有 html。
- 置 false：该页重新生成 html 成功后。
- 无 html 的页不涉及脏。

**UI 呈现**：Deck 列表该页 ⚠️"待更新"；OutlineCard 右上角徽标；HTML 视图顶部"大纲已更新，当前为旧版" + [重新生成本页]。

**消脏（显式）**：单页 [重新生成本页] → 单页 generate(slide_id) → 成功置 false；Deck 顶部 [重新生成所有待更新页] 批量逐页。绝不自动重渲染。

### 4.5 结构操作（AI + 手动）

**手动 UI**
- 加页：Deck 底部 [+ 加页] 或某页"在此后插入" → `POST /projects/{id}/slides`，新页空白 slide.json（默认 layout=bullets），order 落相邻页之间，无 html→自动大纲视图。
- 删页：悬浮/右键 [删除] → **二次确认弹窗**（无回收站）→ `DELETE /slides/{id}`。
- 重排：Deck 拖拽 → `POST /projects/{id}/mutations`，批量重写 order，磁盘零迁移。

**AI 工具（大纲编辑 runner 内，与手动共享 service 底层）**
```
patch_outline_slide(slide_id, {字段...})        改现有页字段（可跨页多次）
add_outline_slide({after_slide_id?, layout?, ...}) 加页
delete_outline_slide(slide_id)                    删页 → 发 needs_input 二次确认后才真删
reorder_outline_slides([slide_id,...])            重排
finish
```
- 全部经 slidejson 校验 + 写盘 + 置脏（有 html 的页）。
- **AI 删页**：不直接删，发 `needs_input`（复用 HITL）:"即将删除第 N 页《标题》，确认？[确认删除/取消]"，用户 `/runs/{id}/input` 确认后才删。加页/改字段/重排 AI 直接执行。

**并发与一致性护栏（统一）**
- 所有结构/内容写（REST 手动 + AI 工具）走 project 锁串行化。
- 手动写撞活跃 run → 409 RUN_ACTIVE（前端置只读）。
- order 间隔分配 + 中值插入；间隔耗尽后台规整。

## 5. 端到端流程（对齐后的 SOP）

```
新建 project
  → Outline：生成大纲（kind=outline，落 slide.json）
       每页可切"大纲视图"查看/手动编辑；AI 也可改/加/删/重排
  → 【整套生成】（kind=generate 不带 page_index）  ← 建 tokens.css + design-spec，逐页出 html
       生成后各页默认 HTML 视图，可切回大纲视图看意图
  → Page：单页编辑；Overview：全局（依赖 tokens.css，须先整套生成）；Repo：资产
  → 回大纲改内容/结构 → 相关页 outline_dirty=true → 用户显式 [重新生成] 消脏
```

> 注：本设计聚焦大纲↔页面的多模态切换与编辑。"整套生成入口"的具体触发方式（显式按钮 vs 自动接续）作为紧邻的独立议题，不在本 spec 强制，但双视图与脏标记已为其预留位置。

## 6. 影响的模块边界

| 模块 | 变更 |
|---|---|
| DB schema | slides 新增 order、outline_dirty；清库重来 |
| 磁盘布局 | slides/<slide_id>/、versions/slide-<slide_id>/ |
| 后端 service/slide | 读带 content、字段 PATCH、加/删/重排、脏位置/清位 |
| 后端 agent/outline | 新增大纲编辑 runner + 4 个结构/内容工具 + AI 删页 HITL |
| 后端 路径拼接 | ~10 文件 idx→slide_id |
| 前端 deckStore | viewByPage + effectiveView |
| 前端 PreviewWorkspace | 视图段控 + OutlineCard 渲染 + 总览缩略卡 |
| 前端 DeckNavigator | 真实 title、脏标记、加/删/拖拽重排入口 |
| 前端 api/slides | content 类型、PATCH、结构操作端点 |

## 7. 测试策略

- 后端：slide_id 路径拼接全链路回归；PATCH 局部更新 + 脏位；加/删/重排 order 正确性 + 零迁移；RUN_ACTIVE 互斥；AI 删页 needs_input 流程；大纲编辑 runner 单页/跨页。
- 前端：effectiveView 派生（含手动覆盖/智能默认）；OutlineCard 渲染各字段；视图段控禁用态；脏标记展示；拖拽重排调用。
- e2e：大纲→切视图→手动编辑→AI 改→加/删/重排→整套生成→脏标记→重生成消脏。

## 8. 风险与缓解

| 风险 | 缓解 |
|---|---|
| slide_id 路径改造面广、易漏 | 集中一个 `slidePath(slideID)` helper，全部改调它；靠回归测试兜底；清库重来降低历史包袱 |
| AI 跨多页编辑误伤 | 删页强制 needs_input 确认；改字段可见即可改回；重排可逆 |
| 手动与 AI 并发写冲突 | project 锁 + RUN_ACTIVE 互斥（前端只读 + 后端 409）双保险 |
| order 间隔耗尽 | 后台一次性规整，用户无感 |
| 列表带 content 变大 | 页数量级小（通常 <30），可接受；必要时后续分页 |
</content>
