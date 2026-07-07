# 大纲/页面多模态切换 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让大纲与页面成为每页可独立切换的两种呈现，支持 AI+手动编辑大纲内容与加/删/重排结构，并以稳定 slide_id 命名目录为地基。

**Architecture:** 四个顺序依赖的阶段（Phase）。Phase 1 把页身份从"位置序号 idx"重构为"稳定 slide_id 命名目录 + order 排序字段"（纯重构，完成后行为不变）；Phase 2 补 slide-json 读写契约 + outline_dirty 脏标记；Phase 3 前端双视图（大纲/HTML 每页独立切换）；Phase 4 结构操作（加/删/重排，AI+手动双入口，AI 删页走 needs_input 确认）。每阶段结束都是可运行、可测的软件。

**Tech Stack:** Go 1.26（Gin + GORM + modernc sqlite + wire）、React + Vite + TS + Tailwind + Zustand、SSE。

**Spec:** `docs/superpowers/specs/2026-07-07-outline-page-modes-design.md`

## Global Constraints

- 后端在 `backend/` 下运行：`go run -ldflags=-linkmode=external ./cmd/server`；测试 `go test ./...`、`go vet ./...`。
- 前端在 `frontend/` 下：`pnpm test`、`pnpm tsc --noEmit`、`pnpm build`。
- 开发期**直接清库重来**（清空 `~/.dasi/ppt`），不写数据迁移脚本；只保留一套 slide_id 命名逻辑。
- 不改 SSE 帧格式、三层架构（Run→Harness→Tools）、单机无鉴权定位。
- slide.json 编辑**不产版本快照**；删页**无回收站**（靠二次确认）。html 产物版本沿用现有体系。
- 所有大纲写操作（REST 手动 + AI 工具）走 project 锁串行化；project 有活跃 run 时手动写返回 `409 RUN_ACTIVE`。
- DRY / YAGNI / TDD / 频繁提交；提交信息用 `feat(outline-modes):` / `fix(outline-modes):` 前缀。

---

## Phase 1 — 页身份重构（slide_id 命名目录 + order 字段）

**阶段目标（可测交付）**：所有 slide 磁盘路径从 `slides/%03d/` 改为 `slides/<slide_id>/`，versions 从 `versions/slide-%03d/` 改为 `versions/slide-<slide_id>/`；DB 新增 `order` 列；`ListSlides` 按 order 排序。完成后**全部现有功能行为不变**（generate/edit/overview/rollback），现有测试改造后全绿。

### 文件结构（Phase 1 触及）

- 新建：`backend/internal/model/slidepath.go`（集中路径拼接 helper，DRY 收口）
- 修改：`backend/migrations/0001_init.sql`（slides 加 `order`、`outline_dirty` 列）
- 修改：`backend/internal/model/slide.go`（Slide 加 `Order`、`OutlineDirty`）
- 修改：`backend/internal/model/version_target.go`（slide 版本 target 改用 slide_id）
- 修改：`backend/internal/store/sqlite/po.go`（slidePO 加列 + 映射）
- 修改：`backend/internal/store/sqlite/slide_store.go`（排序改 order；新增按 id 的版本方法）
- 修改：`backend/internal/store/store.go`（接口调整：见 Task 1.6）
- 修改：路径拼接站点（Task 1.7 枚举）：`agent/outline/submit_tool.go`、`agent/generate/runner.go`、`agent/generate/write_tool.go`、`agent/edit/runner.go`、`agent/edit/read_tool.go`、`agent/edit/patch_tool.go`、`agent/overview/fanout_tool.go`、`agent/assetops/mount_tool.go`、`service/slide.go`

### Task 1.1: slides 表加 order / outline_dirty 列

**Files:**
- Modify: `backend/migrations/0001_init.sql`（slides 建表段）
- Test: `backend/migrations/migrations_test.go`

**Interfaces:**
- Produces: slides 表含 `"order" INTEGER NOT NULL DEFAULT 0`、`outline_dirty INTEGER NOT NULL DEFAULT 0`

- [ ] **Step 1: 读现有建表语句确认列**

Run: `grep -n "CREATE TABLE" backend/migrations/0001_init.sql`
Expected: 找到 `slides` 建表段，含 `idx`、`layout`、`title`、`json_path`、`html_path`、`current_version`、`last_export_at`。

- [ ] **Step 2: 写迁移测试（先失败）**

在 `migrations_test.go` 增加：

```go
func TestSlidesHasOrderAndDirtyColumns(t *testing.T) {
	db := openInMemory(t) // 复用本包既有的建库 helper；若无则用 sqlite.Open + Migrate
	cols := tableColumns(t, db, "slides")
	for _, want := range []string{"order", "outline_dirty"} {
		if !cols[want] {
			t.Fatalf("slides table missing column %q; got %v", want, cols)
		}
	}
}
```

若本包无 `openInMemory`/`tableColumns` helper，则在测试文件内就地实现：用 `gorm.Open(sqlite.Open(":memory:"))` + `Migrate`，再 `db.Raw("PRAGMA table_info(slides)").Scan(...)` 收集列名。

- [ ] **Step 3: 运行测试确认失败**

Run: `cd backend && go test ./migrations/ -run TestSlidesHasOrderAndDirtyColumns -v`
Expected: FAIL，提示缺少 `order`/`outline_dirty` 列。

- [ ] **Step 4: 改建表语句**

在 `0001_init.sql` 的 slides 建表中，`current_version` 行后加两列（`order` 是 SQL 保留字，必须加双引号）：

```sql
  "order"        INTEGER NOT NULL DEFAULT 0,
  outline_dirty  INTEGER NOT NULL DEFAULT 0,
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd backend && go test ./migrations/ -run TestSlidesHasOrderAndDirtyColumns -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/0001_init.sql backend/migrations/migrations_test.go
git commit -m "feat(outline-modes): add order and outline_dirty columns to slides"
```

### Task 1.2: model.Slide 加 Order / OutlineDirty 字段

**Files:**
- Modify: `backend/internal/model/slide.go`
- Modify: `backend/internal/store/sqlite/po.go`（slidePO + 映射）
- Test: `backend/internal/store/sqlite/slide_store_test.go`

**Interfaces:**
- Produces: `model.Slide{ ..., Order int, OutlineDirty bool }`；slidePO 含 `Order int gorm:"column:order"`、`OutlineDirty bool gorm:"column:outline_dirty"`

- [ ] **Step 1: 写往返测试（先失败）**

在 `slide_store_test.go` 增加（用本包既有建库 helper）：

```go
func TestSlideOrderAndDirtyRoundTrip(t *testing.T) {
	st := newTestStore(t) // 复用本包既有 helper
	ctx := context.Background()
	mustCreateProject(t, st, "p1")
	err := st.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "s1", ProjectID: "p1", Order: 10, OutlineDirty: true, Layout: "cover", Title: "A",
			JSONPath: "slides/s1/slide.json", HTMLPath: "slides/s1/index.html"},
	})
	if err != nil { t.Fatal(err) }
	got, err := st.GetSlide(ctx, "s1")
	if err != nil { t.Fatal(err) }
	if got.Order != 10 || !got.OutlineDirty {
		t.Fatalf("round trip lost fields: %+v", got)
	}
}
```

若无 `newTestStore`/`mustCreateProject`，就地用 `sqlite.Open` + `Migrate` 建库并 `CreateProject`。

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestSlideOrderAndDirtyRoundTrip -v`
Expected: FAIL（编译错误：Slide 无 Order 字段）。

- [ ] **Step 3: 加字段与映射**

`model/slide.go` 的 `Slide` 结构在 `CurrentVersion` 后加：

```go
	Order        int
	OutlineDirty bool
```

`store/sqlite/po.go` 的 `slidePO` 在 `CurrentVersion` 后加（`order` 为保留字，列名加反引号转义在 gorm tag 里用双引号）：

```go
	Order        int    `gorm:"column:\"order\""`
	OutlineDirty bool   `gorm:"column:outline_dirty"`
```

`slidePO.toModel()` 增加 `Order: s.Order, OutlineDirty: s.OutlineDirty,`；`slideToPO(m)` 增加 `Order: m.Order, OutlineDirty: m.OutlineDirty,`。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestSlideOrderAndDirtyRoundTrip -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/model/slide.go backend/internal/store/sqlite/po.go backend/internal/store/sqlite/slide_store_test.go
git commit -m "feat(outline-modes): add Order and OutlineDirty to Slide model and PO"
```

### Task 1.3: 集中 slide 路径 helper

**Files:**
- Create: `backend/internal/model/slidepath.go`
- Test: `backend/internal/model/slidepath_test.go`

**Interfaces:**
- Produces:
  - `func SlideDir(slideID string) string` → `"slides/" + slideID`
  - `func SlideJSONPath(slideID string) string` → `"slides/" + slideID + "/slide.json"`
  - `func SlideHTMLPath(slideID string) string` → `"slides/" + slideID + "/index.html"`
  - `func SlideVersionSnapshot(slideID string, versionNo int) string` → `fmt.Sprintf("versions/slide-%s/v%d.html", slideID, versionNo)`

- [ ] **Step 1: 写 helper 测试（先失败）**

```go
package model

import "testing"

func TestSlidePaths(t *testing.T) {
	if SlideDir("abc") != "slides/abc" {
		t.Fatal(SlideDir("abc"))
	}
	if SlideJSONPath("abc") != "slides/abc/slide.json" {
		t.Fatal(SlideJSONPath("abc"))
	}
	if SlideHTMLPath("abc") != "slides/abc/index.html" {
		t.Fatal(SlideHTMLPath("abc"))
	}
	if SlideVersionSnapshot("abc", 2) != "versions/slide-abc/v2.html" {
		t.Fatal(SlideVersionSnapshot("abc", 2))
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/model/ -run TestSlidePaths -v`
Expected: FAIL（未定义）。

- [ ] **Step 3: 实现 helper**

```go
package model

import "fmt"

func SlideDir(slideID string) string          { return "slides/" + slideID }
func SlideJSONPath(slideID string) string      { return SlideDir(slideID) + "/slide.json" }
func SlideHTMLPath(slideID string) string      { return SlideDir(slideID) + "/index.html" }
func SlideVersionSnapshot(slideID string, versionNo int) string {
	return fmt.Sprintf("versions/slide-%s/v%d.html", slideID, versionNo)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/model/ -run TestSlidePaths -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/model/slidepath.go backend/internal/model/slidepath_test.go
git commit -m "feat(outline-modes): add slide_id-based path helpers"
```

### Task 1.4: slide 版本 target 改用 slide_id

**Files:**
- Modify: `backend/internal/model/version_target.go`
- Test: `backend/internal/model/version_target_test.go`（若无则新建）

**Interfaces:**
- Produces: `func SlideVersionTarget(projectID, slideID string) string` → `"project/" + projectID + "/slide-" + slideID`
- Consumes: 调用方（`service/slide.go`、generate/edit/mount 的版本写入）改为传 slideID

- [ ] **Step 1: 写测试（先失败）**

```go
func TestSlideVersionTargetUsesID(t *testing.T) {
	got := SlideVersionTarget("p1", "s9")
	if got != "project/p1/slide-s9" {
		t.Fatal(got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/model/ -run TestSlideVersionTargetUsesID -v`
Expected: FAIL（当前签名是 `(projectID string, idx int)`，编译错误）。

- [ ] **Step 3: 改签名**

`version_target.go`：

```go
func SlideVersionTarget(projectID, slideID string) string {
	return fmt.Sprintf("project/%s/slide-%s", projectID, slideID)
}
```

此改动会让 `service/slide.go`（两处 `SlideVersionTarget(sl.ProjectID, sl.Idx)`）编译失败——在 Task 1.7 统一修正调用方。

- [ ] **Step 4: 运行确认通过（仅本包）**

Run: `cd backend && go test ./internal/model/ -run TestSlideVersionTargetUsesID -v`
Expected: PASS（本包通过；全量编译待 1.7）。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/model/version_target.go backend/internal/model/version_target_test.go
git commit -m "feat(outline-modes): slide version target keyed by slide_id"
```

### Task 1.5: ListSlides 按 order 排序 + SetSlideVersion 改按 slide_id

**Files:**
- Modify: `backend/internal/store/sqlite/slide_store.go`
- Modify: `backend/internal/store/store.go`（接口签名）
- Test: `backend/internal/store/sqlite/slide_store_test.go`

**Interfaces:**
- Produces:
  - `ListSlides` 结果按 `"order" ASC` 排序
  - `SetSlideVersion(ctx, slideID string, versionNo int) error`（签名从 `(projectID string, idx, versionNo int)` 改为按 slideID）
- Consumes: `service/slide.go` 的 rollback 调用（1.7 修正）

- [ ] **Step 1: 写排序测试（先失败）**

```go
func TestListSlidesOrderedByOrder(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	mustCreateProject(t, st, "p1")
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "b", ProjectID: "p1", Order: 20, Layout: "content", Title: "B"},
		{ID: "a", ProjectID: "p1", Order: 10, Layout: "cover", Title: "A"},
	})
	got, _ := st.ListSlides(ctx, "p1")
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("expected order a,b got %+v", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestListSlidesOrderedByOrder -v`
Expected: FAIL（当前按 idx 排序，两页 idx 均 0，顺序不定/按插入）。

- [ ] **Step 3: 改排序与签名**

`slide_store.go`：`ListSlides` 的 `Order("idx ASC")` 改为 `Order("\"order\" ASC")`。

新增/改写 `SetSlideVersion`（若存在旧签名，替换）：

```go
// SetSlideVersion 更新某页 current_version（按 slide id 定位）。
func (s *Store) SetSlideVersion(ctx context.Context, slideID string, versionNo int) error {
	return s.db.WithContext(ctx).Model(&slidePO{}).
		Where("id = ?", slideID).
		Update("current_version", versionNo).Error
}
```

`store/store.go` 接口：`SetSlideVersion(ctx context.Context, projectID string, idx, versionNo int) error` 改为 `SetSlideVersion(ctx context.Context, slideID string, versionNo int) error`。

- [ ] **Step 4: 运行确认通过（本包）**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestListSlidesOrderedByOrder -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/store/sqlite/slide_store.go backend/internal/store/store.go backend/internal/store/sqlite/slide_store_test.go
git commit -m "feat(outline-modes): order slides by order column; SetSlideVersion by slide_id"
```

### Task 1.6: outline submit 落 order + slide_id 路径

**Files:**
- Modify: `backend/internal/agent/outline/submit_tool.go`（`writeFiles`）
- Test: `backend/internal/agent/outline/submit_tool_test.go`

**Interfaces:**
- Consumes: `model.SlideJSONPath/SlideHTMLPath`（Task 1.3）
- Produces: 每页 `slides/<id>/slide.json`；`model.Slide{ Order: i*10 }`（间隔分配）

- [ ] **Step 1: 改写测试预期（先失败）**

在 `submit_tool_test.go` 找到断言路径 `slides/000/...` 的用例，改为断言按 slide_id：即执行后从返回 metas 取每页 `ID`，断言磁盘存在 `slides/<ID>/slide.json`，且 `metas[i].Order == i*10`、`metas[i].JSONPath == "slides/"+metas[i].ID+"/slide.json"`。

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/agent/outline/ -run TestSubmit -v`
Expected: FAIL（当前写 `slides/%03d`、Order 未设）。

- [ ] **Step 3: 改 writeFiles**

`submit_tool.go` 的 `writeFiles`，把：

```go
		dir := fmt.Sprintf("slides/%03d", s.Idx)
		jsonPath := dir + "/slide.json"
		htmlPath := dir + "/index.html"
```

改为：

```go
		jsonPath := model.SlideJSONPath(s.ID)
		htmlPath := model.SlideHTMLPath(s.ID)
```

并在 `metas[i]` 构造中加 `Order: i * 10,`（保留 `Idx: s.Idx` 不影响）。确认文件顶部已 import `model`（已 import）。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/agent/outline/ -run TestSubmit -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/agent/outline/submit_tool.go backend/internal/agent/outline/submit_tool_test.go
git commit -m "feat(outline-modes): outline submit writes slide_id dirs with interval order"
```

### Task 1.7: 全站路径拼接 idx→slide_id 收口

**Files（逐一修改，全部把 `fmt.Sprintf("slides/%03d/...", idx)` / `versions/slide-%03d` 改为按 slideID 的 helper）:**
- Modify: `backend/internal/agent/generate/runner.go:222,249,458`
- Modify: `backend/internal/agent/generate/write_tool.go:72,104`
- Modify: `backend/internal/agent/edit/runner.go:62,66`
- Modify: `backend/internal/agent/edit/read_tool.go:52`
- Modify: `backend/internal/agent/edit/patch_tool.go:90,140`
- Modify: `backend/internal/agent/overview/fanout_tool.go:105,109`
- Modify: `backend/internal/agent/assetops/mount_tool.go:107,233,249`
- Modify: `backend/internal/service/slide.go:55,101,116`
- Test: 全量 `go build ./...` + 相关包测试

**Interfaces:**
- Consumes: `model.SlideHTMLPath(id)`、`model.SlideJSONPath(id)`、`model.SlideVersionSnapshot(id, no)`、`model.SlideVersionTarget(projectID, id)`、`SetSlideVersion(ctx, slideID, no)`

**说明**：这些工具目前多以 `idx int` 定位页。改造原则——凡持有 slide 的地方改为持有/传入 `slideID`。edit/generate/mount/fanout 的 runner 均能拿到目标 `model.Slide`（含 ID），把内部 `idx` 参数替换为 `slideID string`。逐文件机械替换，替换后靠编译 + 测试兜底。

- [ ] **Step 1: 先让全量编译暴露所有调用点**

Run: `cd backend && go build ./... 2>&1 | head -50`
Expected: 一批编译错误，集中在上列文件（SlideVersionTarget/SetSlideVersion 签名变更 + %03d 站点）。这就是待修清单。

- [ ] **Step 2: service/slide.go 修正**

`RollbackSlide` 内：
- `model.SlideVersionTarget(sl.ProjectID, sl.Idx)` → `model.SlideVersionTarget(sl.ProjectID, sl.ID)`（两处：`ListVersions` 前与 `target :=`）。
- `snap := fmt.Sprintf("versions/slide-%03d/v%d.html", sl.Idx, newNo)` → `snap := model.SlideVersionSnapshot(sl.ID, newNo)`。
- `svc.store.SetSlideVersion(ctx, sl.ProjectID, sl.Idx, newNo)` → `svc.store.SetSlideVersion(ctx, sl.ID, newNo)`。
- `ListVersions`（服务方法）内 `model.SlideVersionTarget(sl.ProjectID, sl.Idx)` → `...(sl.ProjectID, sl.ID)`。

- [ ] **Step 3: generate 包修正**

`generate/runner.go`：`fmt.Sprintf("slides/%03d/index.html", sl.Idx)`（222/249）→ `model.SlideHTMLPath(sl.ID)`；`fmt.Sprintf("slides/%03d/slide.json", sl.Idx)`（458）→ `model.SlideJSONPath(sl.ID)`。确认 `sl` 是 `model.Slide`（有 ID）。
`generate/write_tool.go`：该工具当前按 `idx` 写页与版本快照。把工具的 `idx int` 字段/参数改为 `slideID string`，`rel := fmt.Sprintf("slides/%03d/index.html", idx)`（72）→ `model.SlideHTMLPath(t.slideID)`；`snap := fmt.Sprintf("versions/slide-%03d/v%d.html", idx, no)`（104）→ `model.SlideVersionSnapshot(t.slideID, no)`；版本 target 与 `SetSlideVersion` 同步改 slideID。构造处（`generate/runner.go` 的 `NewWriteSlideTool(...)`）传 `sl.ID`。

- [ ] **Step 4: edit 包修正**

`edit/runner.go`：读取 `slides/%03d/index.html`、`slides/%03d/slide.json`（62/66）→ `model.SlideHTMLPath(sl.ID)` / `model.SlideJSONPath(sl.ID)`。runner 需先拿到目标 `model.Slide`（当前用 `PageIndex int`）——改为：用 `store.ListSlides` 取排序后列表，按 `PageIndex` 取 `slides[idx]` 得到 `slideID`，后续全用 id。
`edit/read_tool.go:52`、`edit/patch_tool.go:90,140`：同理把 `idx` 定位改 `slideID`（工具字段 `idx int` → `slideID string`），版本快照/ target 用 helper。构造处传 `slide.ID`。

- [ ] **Step 5: overview / mount 修正**

`overview/fanout_tool.go:105,109`：fanout 逐页时按 order 排序的 slides 列表取 `slideID`，读 `model.SlideHTMLPath(id)` / `model.SlideJSONPath(id)`。
`assetops/mount_tool.go:107,233,249`：`idx` → `slideID`；`slides/%03d/index.html`→`model.SlideHTMLPath(id)`，目录 `filepath.Join(projectRoot,"slides",fmt.Sprintf("%03d",idx))`→`filepath.Join(projectRoot, model.SlideDir(id))`，版本快照→`model.SlideVersionSnapshot(id,no)`。mount 工具的目标页参数（`*int` slideIdx）改为 `slideID string`；调用方（edit/overview 构造 mount 工具处）传 id。

- [ ] **Step 6: 全量编译 + 相关测试**

Run: `cd backend && go build ./... && go vet ./...`
Expected: 通过，无 `%03d` slide 路径残留（`grep -rn "slides/%03d" backend/internal` 只应剩测试内的构造，Step 7 处理）。

- [ ] **Step 7: 修相关测试的路径构造**

Run: `grep -rn "slides/%03d\|slide-%03d" backend/internal --include=*_test.go`
对每个测试文件：把手工构造的 `slides/%03d` 磁盘写入与 `model.Slide{Idx:i,...}` 改为使用稳定 id（如 `fmt.Sprintf("s%d", i)`）+ `model.SlideDir(id)` + `Order: i*10`，HTMLPath/JSONPath 用 helper。逐包运行修正。

- [ ] **Step 8: 全量测试**

Run: `cd backend && go test ./...`
Expected: PASS（Phase 1 完成，行为不变，仅路径与排序键改变）。

- [ ] **Step 9: Commit**

```bash
git add backend/internal
git commit -m "feat(outline-modes): route all slide file/version paths by slide_id"
```

### Phase 1 验收

Run: `cd backend && go vet ./... && go test ./...`
Expected: 全绿。手动冒烟（可选）：清库 → 新建 project → outline → 整套 generate，确认磁盘出现 `slides/<uuid>/index.html` 与 `versions/slide-<uuid>/`，预览正常。

---

## Phase 2 — slide-json 读写契约 + outline_dirty 脏标记（后端）

**阶段目标（可测交付）**：`GET /slides/{id}` 与 `GET /projects/{id}/slides` 返回 `content`（slide-json 全文）、`order`、`outline_dirty`；新增 `PATCH /slides/{id}` 字段级即时保存（有 html 的页置脏、活跃 run 时 409、走 project 锁、不产版本）；generate 成功后清脏。前端暂不改，用 curl/测试验证。

### 文件结构（Phase 2 触及）

- 修改：`backend/internal/service/slide.go`（新增 `ReadContent`、`PatchContent`、`markDirty`/`clearDirty`）
- 新建：`backend/internal/service/slide_content.go`（slide.json 读写 + 脏位逻辑，避免 slide.go 过大）
- 修改：`backend/internal/store/store.go` + `slide_store.go`（新增 `SetOutlineDirty`、`UpdateSlideMeta`）
- 修改：`backend/internal/httpapi/slide_handler.go`（响应加字段 + PatchSlide handler）
- 修改：`backend/internal/httpapi/project_handler.go`（ListSlides 响应加 content/order/dirty）
- 修改：`backend/internal/httpapi/router.go`（注册 `PATCH /slides/:id`）
- 修改：`backend/internal/agent/generate/write_tool.go`（写页成功清该页 dirty）
- 复用：`backend/internal/run` 的活跃 run 查询（RUN_ACTIVE 判定，见 Task 2.4）

### Task 2.1: store 新增 SetOutlineDirty / UpdateSlideMeta

**Files:**
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/sqlite/slide_store.go`
- Test: `backend/internal/store/sqlite/slide_store_test.go`

**Interfaces:**
- Produces:
  - `SetOutlineDirty(ctx context.Context, slideID string, dirty bool) error`
  - `UpdateSlideMeta(ctx context.Context, slideID, title, layout string) error`（PATCH 改 title/layout 时同步 DB 元数据）

- [ ] **Step 1: 写测试（先失败）**

```go
func TestSetOutlineDirtyAndUpdateMeta(t *testing.T) {
	st := newTestStore(t); ctx := context.Background()
	mustCreateProject(t, st, "p1")
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{{ID:"s1",ProjectID:"p1",Order:10,Layout:"cover",Title:"A"}})
	if err := st.SetOutlineDirty(ctx, "s1", true); err != nil { t.Fatal(err) }
	if err := st.UpdateSlideMeta(ctx, "s1", "B", "content"); err != nil { t.Fatal(err) }
	got, _ := st.GetSlide(ctx, "s1")
	if !got.OutlineDirty || got.Title != "B" || got.Layout != "content" {
		t.Fatalf("got %+v", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestSetOutlineDirtyAndUpdateMeta -v`
Expected: FAIL（未定义方法）。

- [ ] **Step 3: 实现方法 + 接口**

`slide_store.go` 增加：

```go
func (s *Store) SetOutlineDirty(ctx context.Context, slideID string, dirty bool) error {
	return s.db.WithContext(ctx).Model(&slidePO{}).
		Where("id = ?", slideID).Update("outline_dirty", dirty).Error
}

func (s *Store) UpdateSlideMeta(ctx context.Context, slideID, title, layout string) error {
	return s.db.WithContext(ctx).Model(&slidePO{}).
		Where("id = ?", slideID).
		Updates(map[string]any{"title": title, "layout": layout}).Error
}
```

`store/store.go` 接口加这两行签名。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestSetOutlineDirtyAndUpdateMeta -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/store
git commit -m "feat(outline-modes): store SetOutlineDirty and UpdateSlideMeta"
```

### Task 2.2: service 读 slide-json 内容（ReadContent）

**Files:**
- Create: `backend/internal/service/slide_content.go`
- Test: `backend/internal/service/slide_test.go`

**Interfaces:**
- Consumes: `store.GetSlide`、`store.GetProject`、`tools.NewSandbox`、`slidejson.SlideJSON`
- Produces: `func (svc *SlideService) ReadContent(ctx, slideID string) (slidejson.SlideJSON, error)`（从 `slides/<id>/slide.json` 读；文件缺失返回由元数据回退的最小 SlideJSON）

- [ ] **Step 1: 写测试（先失败）**

```go
func TestReadContentFromDisk(t *testing.T) {
	svc, st, workDir := newSlideServiceWithProject(t) // 见下：若无 helper 就地建
	ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{{ID:"s1",ProjectID:"p1",Order:10,Layout:"bullets",Title:"标题",
		JSONPath: model.SlideJSONPath("s1"), HTMLPath: model.SlideHTMLPath("s1")}})
	writeFile(t, workDir, model.SlideJSONPath("s1"),
		`{"id":"s1","layout":"bullets","title":"标题","bullets":["a","b"]}`)
	got, err := svc.ReadContent(ctx, "s1")
	if err != nil { t.Fatal(err) }
	if got.Title != "标题" || len(got.Bullets) != 2 {
		t.Fatalf("got %+v", got)
	}
}
```

`newSlideServiceWithProject` 若无：就地用 `sqlite.Open`+`Migrate` 建 store、`ProjectService.CreateProject` 建 p1 拿 workDir、`NewSlideService(st)`。`writeFile` 用 `tools.NewSandbox(workDir).Write`。

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/service/ -run TestReadContentFromDisk -v`
Expected: FAIL（未定义 ReadContent）。

- [ ] **Step 3: 实现 ReadContent**

`slide_content.go`：

```go
package service

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ReadContent 读某页 slide.json 全文；文件缺失时用元数据回退最小结构。
func (svc *SlideService) ReadContent(ctx context.Context, slideID string) (slidejson.SlideJSON, error) {
	sl, err := svc.store.GetSlide(ctx, slideID)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	proj, err := svc.store.GetProject(ctx, sl.ProjectID)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	sb, err := tools.NewSandbox(proj.WorkDir)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	raw, err := sb.Read(model.SlideJSONPath(slideID))
	if err != nil {
		return slidejson.SlideJSON{ID: sl.ID, Idx: sl.Idx, Layout: sl.Layout, Title: sl.Title}, nil
	}
	return slidejson.Parse(raw)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/service/ -run TestReadContentFromDisk -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/slide_content.go backend/internal/service/slide_test.go
git commit -m "feat(outline-modes): SlideService.ReadContent from slide.json"
```

### Task 2.3: service PatchContent（字段级即时保存 + 置脏）

**Files:**
- Modify: `backend/internal/service/slide_content.go`
- Test: `backend/internal/service/slide_test.go`

**Interfaces:**
- Produces:
  - `type SlidePatch struct { Title, Subtitle, ContentIntent, Layout *string; Bullets *[]string; ChartIntent *slidejson.ChartIntent; Steps *int }`
  - `func (svc *SlideService) PatchContent(ctx, slideID string, p SlidePatch) (slidejson.SlideJSON, error)`（局部更新 slide.json → 回写 → 若该页有 html 则 SetOutlineDirty(true) → 若改了 title/layout 则 UpdateSlideMeta；不产版本）

- [ ] **Step 1: 写测试（先失败）**

```go
func TestPatchContentUpdatesAndMarksDirty(t *testing.T) {
	svc, st, workDir := newSlideServiceWithProject(t); ctx := context.Background()
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{{ID:"s1",ProjectID:"p1",Order:10,Layout:"bullets",Title:"旧",
		JSONPath: model.SlideJSONPath("s1"), HTMLPath: model.SlideHTMLPath("s1")}})
	writeFile(t, workDir, model.SlideJSONPath("s1"), `{"id":"s1","layout":"bullets","title":"旧","bullets":["a"]}`)
	writeFile(t, workDir, model.SlideHTMLPath("s1"), `<html></html>`) // 有 html → 应置脏
	newTitle := "新标题"
	got, err := svc.PatchContent(ctx, "s1", SlidePatch{Title: &newTitle})
	if err != nil { t.Fatal(err) }
	if got.Title != "新标题" { t.Fatalf("json title not updated: %+v", got) }
	sl, _ := st.GetSlide(ctx, "s1")
	if !sl.OutlineDirty { t.Fatal("expected outline_dirty=true when html exists") }
	if sl.Title != "新标题" { t.Fatal("db meta title not synced") }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/service/ -run TestPatchContentUpdatesAndMarksDirty -v`
Expected: FAIL（未定义）。

- [ ] **Step 3: 实现 PatchContent**

在 `slide_content.go` 追加（用 `encoding/json` 回写；判断 html 是否存在用 `sb.Read(SlideHTMLPath)` err==nil）：

```go
type SlidePatch struct {
	Title         *string
	Subtitle      *string
	ContentIntent *string
	Layout        *string
	Bullets       *[]string
	ChartIntent   *slidejson.ChartIntent
	Steps         *int
}

func (svc *SlideService) PatchContent(ctx context.Context, slideID string, p SlidePatch) (slidejson.SlideJSON, error) {
	sl, err := svc.store.GetSlide(ctx, slideID)
	if err != nil { return slidejson.SlideJSON{}, err }
	proj, err := svc.store.GetProject(ctx, sl.ProjectID)
	if err != nil { return slidejson.SlideJSON{}, err }
	sb, err := tools.NewSandbox(proj.WorkDir)
	if err != nil { return slidejson.SlideJSON{}, err }

	cur, err := svc.ReadContent(ctx, slideID)
	if err != nil { return slidejson.SlideJSON{}, err }

	if p.Title != nil { cur.Title = *p.Title }
	if p.Subtitle != nil { cur.Subtitle = *p.Subtitle }
	if p.ContentIntent != nil { cur.ContentIntent = *p.ContentIntent }
	if p.Layout != nil { cur.Layout = *p.Layout }
	if p.Bullets != nil { cur.Bullets = *p.Bullets }
	if p.ChartIntent != nil { cur.ChartIntent = p.ChartIntent }
	if p.Steps != nil { cur.Steps = *p.Steps }
	cur.ID, cur.Idx = sl.ID, sl.Idx

	if err := slidejson.Validate(cur); err != nil {
		return slidejson.SlideJSON{}, validationError(err.Error())
	}
	raw, err := json.MarshalIndent(cur, "", "  ")
	if err != nil { return slidejson.SlideJSON{}, err }
	if err := sb.Write(model.SlideJSONPath(slideID), raw); err != nil {
		return slidejson.SlideJSON{}, err
	}

	if p.Title != nil || p.Layout != nil {
		if err := svc.store.UpdateSlideMeta(ctx, slideID, cur.Title, cur.Layout); err != nil {
			return slidejson.SlideJSON{}, err
		}
	}
	if _, herr := sb.Read(model.SlideHTMLPath(slideID)); herr == nil {
		if err := svc.store.SetOutlineDirty(ctx, slideID, true); err != nil {
			return slidejson.SlideJSON{}, err
		}
	}
	return cur, nil
}
```

在文件顶部 import 增加 `"encoding/json"`。`validationError` 已存在于 service 包（asset.go 使用过）——若签名不符，就地用 `fmt.Errorf`。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/service/ -run TestPatchContentUpdatesAndMarksDirty -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/slide_content.go backend/internal/service/slide_test.go
git commit -m "feat(outline-modes): SlideService.PatchContent field-level save with dirty flag"
```

### Task 2.4: RUN_ACTIVE 互斥判定

**Files:**
- Modify: `backend/internal/service/slide_content.go`（PatchContent 前置检查）
- Modify: `backend/internal/service/run.go` 或新增 helper：暴露"project 是否有活跃 run"
- Test: `backend/internal/service/slide_test.go`

**Interfaces:**
- Produces: `var ErrRunActive = errors.New("service: project has an active run")`；PatchContent 在写前若 project 有活跃 run 返回 ErrRunActive
- Consumes: 判定活跃 run 的方式——查询 `store` 无直接方法；用 `run.Engine` 已持有的 actives，或加 `store` 查询"该 project 下非终态 run"。**采用 store 查询**（更简单、无需注入 engine）

- [ ] **Step 1: store 加 HasActiveRun（先失败）**

在 `store/store.go` 接口加 `HasActiveRun(ctx context.Context, projectID string) (bool, error)`；`sqlite` 实现：查 `runs` 表 `project_id=? AND status NOT IN ('done','failed','canceled')` 是否存在。测试：

```go
func TestHasActiveRun(t *testing.T) {
	st := newTestStore(t); ctx := context.Background()
	mustCreateProject(t, st, "p1")
	_ = st.CreateRun(ctx, model.Run{ID:"r1",ProjectID:"p1",ThreadID:"t1",Status:model.RunRunning})
	on, _ := st.HasActiveRun(ctx, "p1")
	if !on { t.Fatal("expected active") }
	_ = st.SetRunStatus(ctx, "r1", model.RunDone)
	off, _ := st.HasActiveRun(ctx, "p1")
	if off { t.Fatal("expected inactive after done") }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestHasActiveRun -v`
Expected: FAIL

- [ ] **Step 3: 实现 HasActiveRun**

`store/sqlite` 新增（放 run_store.go）：

```go
func (s *Store) HasActiveRun(ctx context.Context, projectID string) (bool, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&runPO{}).
		Where("project_id = ? AND status NOT IN ?", projectID, []string{"done", "failed", "canceled"}).
		Count(&n).Error
	return n > 0, err
}
```

接口加签名。

- [ ] **Step 4: PatchContent 前置检查**

在 `PatchContent` 开头（取到 sl 后）加：

```go
	if active, err := svc.store.HasActiveRun(ctx, sl.ProjectID); err != nil {
		return slidejson.SlideJSON{}, err
	} else if active {
		return slidejson.SlideJSON{}, ErrRunActive
	}
```

在 service 包定义 `ErrRunActive`（`errors.go` 或 slide_content.go）。

- [ ] **Step 5: 运行确认通过**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestHasActiveRun -v && go test ./internal/service/ -run TestPatch -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/store backend/internal/service
git commit -m "feat(outline-modes): RUN_ACTIVE mutex guard for manual slide patch"
```

### Task 2.5: HTTP 层——读响应加字段 + PATCH 端点 + 路由

**Files:**
- Modify: `backend/internal/httpapi/slide_handler.go`
- Modify: `backend/internal/httpapi/project_handler.go`（ListSlides 响应）
- Modify: `backend/internal/httpapi/router.go`
- Test: `backend/internal/httpapi/slide_content_e2e_test.go`（新建）

**Interfaces:**
- Produces:
  - `GET /slides/:id` 响应加 `order`、`outline_dirty`、`content`（slidejson.SlideJSON）
  - `GET /projects/:id/slides` 同步加这些字段
  - `PATCH /slides/:id` body `{title?,subtitle?,bullets?,content_intent?,chart_intent?,layout?,steps?}` → 200 返回更新后 slideResponse；活跃 run → 409 `RUN_ACTIVE`；校验失败 → 422

- [ ] **Step 1: 写 e2e 测试（先失败）**

新建 `slide_content_e2e_test.go`，复用本包 e2e helper（`newTestServer`/`sqlitestore.Open`）：建 project、ReplaceSlides 一页（有 JSONPath）、写 slide.json 到 workDir，然后：
- `GET /api/v1/slides/s1` → 200，body 含 `content.bullets`、`outline_dirty=false`、`order`。
- `PATCH /api/v1/slides/s1` body `{"title":"X"}` → 200，`title=="X"`。
- 造一个 running run 于 p1 → `PATCH` → 409。

（断言用现有 e2e 的 httptest 模式，参考 `project_thread_e2e_test.go`。）

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/httpapi/ -run TestSlideContent -v`
Expected: FAIL（响应无 content / 无 PATCH 路由 404）。

- [ ] **Step 3: 扩展响应 + handler**

`slide_handler.go`：`slideResponse` 加 `Order int json:"order"`、`OutlineDirty bool json:"outline_dirty"`、`Content any json:"content,omitempty"`。`toSlideResponse` 填 Order/OutlineDirty。`GetSlide` handler 里额外 `svc.ReadContent` 填 Content。新增：

```go
func (h *SlideHandler) PatchSlide(c *gin.Context) {
	var body struct {
		Title, Subtitle, ContentIntent, Layout *string
		Bullets *[]string
		ChartIntent *slidejson.ChartIntent
		Steps *int
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		AbortWithError(c, ErrBadRequest("invalid body")); return
	}
	sl, err := h.svc.PatchContent(c.Request.Context(), c.Param("id"), service.SlidePatch{
		Title: body.Title, Subtitle: body.Subtitle, ContentIntent: body.ContentIntent,
		Layout: body.Layout, Bullets: body.Bullets, ChartIntent: body.ChartIntent, Steps: body.Steps,
	})
	switch {
	case err == nil:
		content, _ := h.svc.ReadContent(c.Request.Context(), c.Param("id"))
		_ = sl
		resp := toSlideResponseWithContent(c.Request.Context(), h.svc, c.Param("id"), content)
		c.JSON(http.StatusOK, resp)
	case errors.Is(err, service.ErrRunActive):
		AbortWithError(c, &APIError{HTTPStatus: http.StatusConflict, Code: "RUN_ACTIVE", Message: "project has an active run"})
	default:
		AbortWithError(c, ErrInternal(err.Error()))
	}
}
```

（`toSlideResponseWithContent` 是小 helper：GetSlide + 填 content。为简洁可直接在 handler 内组装。）
`project_handler.go` 的 `ListSlides`：对每页调用 `svc.ReadContent`（经 ProjectService 暴露一个 `SlideContent(ctx, id)` 或直接注入 SlideService）——**简化**：ListSlides 响应加 order/outline_dirty（已在 model），content 可选；若列表也要 content，则 ProjectHandler 需要 SlideService 依赖，在 wire 注入。
`router.go`：加 `v1.PATCH("/slides/:id", r.slide.PatchSlide)`。

- [ ] **Step 4: 运行确认通过**

Run: `cd backend && go test ./internal/httpapi/ -run TestSlideContent -v`
Expected: PASS

- [ ] **Step 5: generate 清脏**

`generate/write_tool.go` 写页成功后（落版本、SetSlideVersion 之后）调用 `store.SetOutlineDirty(ctx, slideID, false)`。补一条断言：单页重生成后 `outline_dirty=false`（在 `generate/runner_test.go` 或 write_tool 测试内）。

Run: `cd backend && go test ./internal/agent/generate/ -v`
Expected: PASS

- [ ] **Step 6: 全量 + Commit**

Run: `cd backend && go vet ./... && go test ./...`
Expected: PASS

```bash
git add backend/internal/httpapi backend/internal/agent/generate backend/cmd
git commit -m "feat(outline-modes): slide content read/patch endpoints; clear dirty on regenerate"
```

### Phase 2 验收

curl 冒烟：`GET /slides/{id}` 有 content/order/outline_dirty；`PATCH` 改 title 生效且有 html 的页 outline_dirty 变 true；活跃 run 时 PATCH 得 409。`go test ./...` 全绿。

---

## Phase 3 — 前端双视图（大纲/HTML 每页独立切换）

**阶段目标（可测交付）**：预览区每页可在大纲视图/HTML 视图切换，智能默认；OutlineCard 渲染 slide-json；DeckNavigator 显示真实 title + 脏标记；总览网格未生成页显示大纲缩略卡；OutlineCard 支持手动就地编辑（失焦 PATCH，活跃 run 只读）。

### 文件结构（Phase 3 触及）

- 修改：`frontend/src/api/types.ts`（Slide 加 order/outline_dirty/content；SlideContent 类型）
- 修改：`frontend/src/api/slides.ts`（新建：get/patch）或并入 projects.ts
- 修改：`frontend/src/stores/deckStore.ts`（viewByPage + effectiveView）
- 新建：`frontend/src/features/viewer/OutlineCard.tsx`
- 修改：`frontend/src/features/viewer/PreviewWorkspace.tsx`（视图段控 + 分支渲染 + 总览缩略卡）
- 修改：`frontend/src/features/deck/DeckNavigator.tsx`（真实 title + 脏标记）

### Task 3.1: 前端类型与 slides API

**Files:**
- Modify: `frontend/src/api/types.ts`
- Create: `frontend/src/api/slides.ts`
- Test: `frontend/src/api/slides.test.ts`（可选，用 vitest mock fetch）

**Interfaces:**
- Produces:
  - `interface SlideContent { subtitle?: string; bullets?: string[]; content_intent?: string; chart_intent?: {type:string;data_hint?:string}; steps?: number; notes?: string; layout: string; title: string }`
  - `Slide` 加 `order: number; outline_dirty: boolean; content?: SlideContent`
  - `slidesApi.patch(slideId, patch): Promise<Slide>`、`slidesApi.get(slideId): Promise<Slide>`

- [ ] **Step 1: 扩展 types.ts**

`Slide` 接口加 `order: number; outline_dirty: boolean; content?: SlideContent;`，并定义 `SlideContent`。

- [ ] **Step 2: 新建 slides.ts**

```ts
import { fetchClient } from './client';
import { Slide } from './types';

export const slidesApi = {
  get: (id: string) => fetchClient<Slide>(`/slides/${id}`),
  patch: (id: string, patch: Partial<Pick<Slide,'title'|'layout'>> & { subtitle?: string; bullets?: string[]; content_intent?: string; steps?: number }) =>
    fetchClient<Slide>(`/slides/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
};
```

- [ ] **Step 3: tsc 通过**

Run: `cd frontend && pnpm tsc --noEmit`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add frontend/src/api/types.ts frontend/src/api/slides.ts
git commit -m "feat(outline-modes): frontend slide content types and slides api"
```

### Task 3.2: deckStore 每页视图偏好

**Files:**
- Modify: `frontend/src/stores/deckStore.ts`
- Test: `frontend/src/stores/deckStore.test.ts`（新建）

**Interfaces:**
- Produces:
  - `viewByPage: Record<string, 'outline' | 'html'>`（key=slideId）
  - `setPageView(slideId: string, view: 'outline'|'html'): void`
  - `effectiveView(slideId: string, hasHtml: boolean): 'outline'|'html'`（有手动选→尊重；否则 hasHtml?'html':'outline'）

- [ ] **Step 1: 写测试（先失败）**

```ts
import { useDeckStore } from './deckStore';
test('effectiveView smart default and override', () => {
  const s = useDeckStore.getState();
  expect(s.effectiveView('s1', true)).toBe('html');
  expect(s.effectiveView('s2', false)).toBe('outline');
  s.setPageView('s2', 'html'); // 覆盖，即便无 html 也返回 html（UI 层禁用不可选）
  expect(useDeckStore.getState().effectiveView('s2', false)).toBe('html');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && pnpm test deckStore -run`（或 `pnpm vitest run src/stores/deckStore.test.ts`）
Expected: FAIL

- [ ] **Step 3: 实现**

`deckStore` state 加 `viewByPage: {}`，action：

```ts
setPageView: (slideId, view) => set((s) => ({ viewByPage: { ...s.viewByPage, [slideId]: view } })),
effectiveView: (slideId, hasHtml) => {
  const v = get().viewByPage[slideId];
  return v ?? (hasHtml ? 'html' : 'outline');
},
```

（zustand `get` 需在 create 回调签名里取到。）

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && pnpm vitest run src/stores/deckStore.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/deckStore.ts frontend/src/stores/deckStore.test.ts
git commit -m "feat(outline-modes): deckStore per-page view preference"
```

### Task 3.3: OutlineCard 渲染组件

**Files:**
- Create: `frontend/src/features/viewer/OutlineCard.tsx`
- Test: `frontend/src/features/viewer/OutlineCard.test.tsx`

**Interfaces:**
- Consumes: `SlideContent`（types）
- Produces: `<OutlineCard slide={Slide} editable={boolean} onPatch={(patch)=>void} dirty={boolean} compact?={boolean} />`

- [ ] **Step 1: 写渲染测试（先失败）**

```tsx
import { render, screen } from '@testing-library/react';
import { OutlineCard } from './OutlineCard';
test('renders title bullets and dirty badge', () => {
  render(<OutlineCard
    slide={{ id:'s1', project_id:'p1', order:10, outline_dirty:true, layout:'bullets', title:'市场分析',
      html_path:'', json_path:'', current_version:0, idx:0,
      content:{ layout:'bullets', title:'市场分析', bullets:['增长放缓','头部集中'] } } as any}
    editable={false} onPatch={()=>{}} dirty />);
  expect(screen.getByText('市场分析')).toBeInTheDocument();
  expect(screen.getByText('增长放缓')).toBeInTheDocument();
  expect(screen.getByText(/待更新/)).toBeInTheDocument();
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && pnpm vitest run src/features/viewer/OutlineCard.test.tsx`
Expected: FAIL

- [ ] **Step 3: 实现 OutlineCard**

渲染 16:9 卡片：layout 徽标、dirty 时右上角"⚠️待更新"、title（大字）、subtitle、bullets 列表、chart_intent 占位、content_intent、steps。`editable` 时 title/bullets 用 `contentEditable` 或输入框，`onBlur` 调 `onPatch({...})`；`editable=false` 时纯展示。`compact` 时缩小字号用于总览缩略卡。

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && pnpm vitest run src/features/viewer/OutlineCard.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/viewer/OutlineCard.tsx frontend/src/features/viewer/OutlineCard.test.tsx
git commit -m "feat(outline-modes): OutlineCard renders slide-json"
```

### Task 3.4: PreviewWorkspace 集成双视图 + 段控 + 总览缩略卡

**Files:**
- Modify: `frontend/src/features/viewer/PreviewWorkspace.tsx`
- Test: `frontend/src/features/viewer/PreviewWorkspace.test.tsx`（扩展现有）

**Interfaces:**
- Consumes: `deckStore.effectiveView/setPageView`、`OutlineCard`、`slidesApi.patch`、`runStore` 活跃状态（当前聚焦 thread 的 status）

- [ ] **Step 1: 写测试（先失败）**

测试：给定当前页无 html → 主视图渲染 OutlineCard（断言出现 title 文本而非 iframe）；点段控"HTML"在无 html 时禁用（断言 disabled）。给定当前页有 html → 默认渲染 iframe。

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && pnpm vitest run src/features/viewer/PreviewWorkspace.test.tsx`
Expected: FAIL

- [ ] **Step 3: 实现**

工具栏右侧加 `[大纲 | HTML]` 段控，值 = `effectiveView(currentSlide.id, hasHtml)`，点击调 `setPageView`；无 html 时 HTML 段 disabled + title 提示"该页尚未生成"。主视图区按 effectiveView 分支：`html` → 现有 iframe；`outline` → `<OutlineCard editable={!runActive} onPatch={p=>slidesApi.patch(id,p).then(reloadSlides)} dirty={slide.outline_dirty} />`。总览网格：`slide.html_path ? iframe缩略 : <OutlineCard compact />`。`runActive` 取自当前聚焦 thread 的 session.status ∈ {running, needs_input}。patch 成功后刷新该 project 的 slides（`projectStore.loadProjectSlides`）。

- [ ] **Step 4: 运行确认通过 + 全量**

Run: `cd frontend && pnpm vitest run src/features/viewer/ && pnpm tsc --noEmit`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/viewer/PreviewWorkspace.tsx frontend/src/features/viewer/PreviewWorkspace.test.tsx
git commit -m "feat(outline-modes): dual view toggle + outline card in preview"
```

### Task 3.5: DeckNavigator 真实 title + 脏标记

**Files:**
- Modify: `frontend/src/features/deck/DeckNavigator.tsx`
- Test: `frontend/src/features/deck/DeckNavigator.test.tsx`（新建）

**Interfaces:**
- Consumes: `slide.title`、`slide.outline_dirty`

- [ ] **Step 1: 写测试（先失败）**

断言：给两页（一页 outline_dirty=true）→ 列表显示各自 `title`（不再是 "Slide 1"），脏页显示"待更新"标记。

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && pnpm vitest run src/features/deck/DeckNavigator.test.tsx`
Expected: FAIL

- [ ] **Step 3: 实现**

列表项文字从 `Slide {index+1}` 改为 `{slide.title || '未命名'}`；`slide.outline_dirty` 时在标题后加 ⚠️ 小点 + title 属性"大纲已改，待更新"。

- [ ] **Step 4: 运行确认通过**

Run: `cd frontend && pnpm vitest run src/features/deck/DeckNavigator.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/deck/DeckNavigator.tsx frontend/src/features/deck/DeckNavigator.test.tsx
git commit -m "feat(outline-modes): deck shows real titles and dirty badge"
```

### Phase 3 验收

Run: `cd frontend && pnpm test && pnpm tsc --noEmit && pnpm build`
Expected: 全绿。手动：outline 后每页可看大纲卡、可就地改标题（失焦保存）、有 html 的页可切 HTML 视图、脏页有徽标、活跃 run 时卡片只读。

---

## Phase 4 — 结构操作（加/删/重排，AI + 手动）

**阶段目标（可测交付）**：REST 加/删/重排端点（受 RUN_ACTIVE + project 锁）；前端 Deck 加页/删页(二次确认)/拖拽重排；大纲编辑 runner（AI）支持 patch/add/delete/reorder 工具，删页走 needs_input 确认；Outline 模式有大纲时路由到大纲编辑 runner（替换现有"退化 Overview"）。

### 文件结构（Phase 4 触及）

- 修改：`backend/internal/service/slide_content.go`（AddSlide/DeleteSlide/ReorderSlides）
- 修改：`backend/internal/store/store.go` + `slide_store.go`（InsertSlide/DeleteSlideByID/SetSlidesOrder）
- 修改：`backend/internal/httpapi/slide_handler.go` + `project_handler.go` + `router.go`（3 端点）
- 新建：`backend/internal/agent/outline/edit_runner.go`（大纲编辑 runner）
- 新建：`backend/internal/agent/outline/outline_tools.go`（patch/add/delete/reorder 工具）
- 修改：`backend/internal/service/run.go`（Outline 有大纲时路由到 edit_runner）
- 修改：`backend/internal/agent/prompt/outline.go`（大纲编辑 prompt）
- 前端：`frontend/src/api/slides.ts`（add/delete/reorder）、`DeckNavigator.tsx`（加/删/拖拽）、`modeMapping.ts`（Outline 有大纲→新 kind 路由，若后端用 kind=outline 区分则前端不变）

### Task 4.1: store 结构操作（Insert/Delete/SetOrder）

**Files:**
- Modify: `backend/internal/store/store.go`、`backend/internal/store/sqlite/slide_store.go`
- Test: `backend/internal/store/sqlite/slide_store_test.go`

**Interfaces:**
- Produces:
  - `InsertSlide(ctx, s model.Slide) error`（单页插入，不清空其它页）
  - `DeleteSlideByID(ctx, slideID string) error`
  - `SetSlidesOrder(ctx, projectID string, orderByID map[string]int) error`（事务批量更新 order）

- [ ] **Step 1: 写测试（先失败）**

```go
func TestInsertDeleteReorder(t *testing.T) {
	st := newTestStore(t); ctx := context.Background(); mustCreateProject(t, st, "p1")
	_ = st.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID:"a",ProjectID:"p1",Order:10,Layout:"cover",Title:"A"},
		{ID:"b",ProjectID:"p1",Order:20,Layout:"thanks",Title:"B"}})
	_ = st.InsertSlide(ctx, model.Slide{ID:"c",ProjectID:"p1",Order:15,Layout:"content",Title:"C"})
	got, _ := st.ListSlides(ctx, "p1")
	if len(got)!=3 || got[1].ID!="c" { t.Fatalf("insert/order wrong: %+v", got) }
	_ = st.SetSlidesOrder(ctx, "p1", map[string]int{"a":30,"b":20,"c":10})
	got, _ = st.ListSlides(ctx, "p1")
	if got[0].ID!="c" || got[2].ID!="a" { t.Fatalf("reorder wrong: %+v", got) }
	_ = st.DeleteSlideByID(ctx, "b")
	got, _ = st.ListSlides(ctx, "p1")
	if len(got)!=2 { t.Fatalf("delete wrong: %+v", got) }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestInsertDeleteReorder -v`
Expected: FAIL

- [ ] **Step 3: 实现三方法**

```go
func (s *Store) InsertSlide(ctx context.Context, sl model.Slide) error {
	po := slideToPO(sl)
	return s.db.WithContext(ctx).Create(&po).Error
}
func (s *Store) DeleteSlideByID(ctx context.Context, slideID string) error {
	return s.db.WithContext(ctx).Where("id = ?", slideID).Delete(&slidePO{}).Error
}
func (s *Store) SetSlidesOrder(ctx context.Context, projectID string, orderByID map[string]int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for id, ord := range orderByID {
			if err := tx.Model(&slidePO{}).Where("id = ? AND project_id = ?", id, projectID).
				Update("order", ord).Error; err != nil { return err }
		}
		return nil
	})
}
```

接口加三行签名。

- [ ] **Step 4: 运行确认通过 + Commit**

Run: `cd backend && go test ./internal/store/sqlite/ -run TestInsertDeleteReorder -v`
Expected: PASS

```bash
git add backend/internal/store
git commit -m "feat(outline-modes): store insert/delete/reorder slides"
```

### Task 4.2: service 结构操作 + 磁盘

**Files:**
- Modify: `backend/internal/service/slide_content.go`
- Test: `backend/internal/service/slide_test.go`

**Interfaces:**
- Produces:
  - `AddSlide(ctx, projectID string, afterSlideID, layout string) (model.Slide, error)`（新建空白 slide.json，order 落锚点与其后一页之间；受 ErrRunActive）
  - `DeleteSlide(ctx, slideID string) error`（删 DB 行 + `slides/<id>/` 目录 + 该页 versions；受 ErrRunActive）
  - `ReorderSlides(ctx, projectID string, orderedIDs []string) error`（按序赋 order=i*10；受 ErrRunActive）

- [ ] **Step 1: 写测试（先失败）**

测试 AddSlide 后磁盘存在 `slides/<newID>/slide.json`（空白 bullets）且列表新增；DeleteSlide 后目录不存在且列表减一；ReorderSlides 后顺序符合。活跃 run 时三者均返回 ErrRunActive。

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/service/ -run TestStructural -v`
Expected: FAIL

- [ ] **Step 3: 实现**

`AddSlide`：校验 run 非活跃；`orderedIDs`/list 取锚点 order 与下一页 order 求中值（无下一页则 +10）；`newID`；写空白 `slidejson.SlideJSON{ID,Layout,Title:""}` 到 `SlideJSONPath(id)`；`store.InsertSlide`（含 JSONPath/HTMLPath/Order）。
`DeleteSlide`：校验；`store.DeleteSlideByID`；`sandbox.Delete` 删 `SlideDir(id)`（递归）与 `versions/slide-<id>`（若 sandbox 无递归删则遍历；简单起见删 slide.json+index.html+目录）。
`ReorderSlides`：校验；`orderByID[id]=i*10`；`store.SetSlidesOrder`。

- [ ] **Step 4: 运行确认通过 + Commit**

Run: `cd backend && go test ./internal/service/ -run TestStructural -v`
Expected: PASS

```bash
git add backend/internal/service
git commit -m "feat(outline-modes): service add/delete/reorder slides with disk ops"
```

### Task 4.3: HTTP 端点（加/删/重排）

**Files:**
- Modify: `backend/internal/httpapi/slide_handler.go`、`project_handler.go`、`router.go`
- Test: `backend/internal/httpapi/slide_content_e2e_test.go`

**Interfaces:**
- Produces:
  - `POST /projects/:id/slides` body `{after_slide_id?, layout?}` → 201 新 slide
  - `DELETE /slides/:id` → 204
  - `POST /projects/:id/slides/reorder` body `{ordered_ids:[...]}` → 200
  - 活跃 run → 409 RUN_ACTIVE

- [ ] **Step 1..2: 写 e2e（加/删/重排 + 409）并确认失败**

Run: `cd backend && go test ./internal/httpapi/ -run TestSlideStructural -v`
Expected: FAIL

- [ ] **Step 3: handler + 路由**

ProjectHandler 加 `CreateSlide`、`ReorderSlides`；SlideHandler 加 `DeleteSlide`（调 service，ErrRunActive→409）。router 注册三条。DeleteSlide 复用现有 `DELETE /slides/:id`？当前无此路由（只有 rollback），新增。

- [ ] **Step 4: 通过 + 全量 + Commit**

Run: `cd backend && go vet ./... && go test ./...`
Expected: PASS

```bash
git add backend/internal/httpapi backend/cmd
git commit -m "feat(outline-modes): REST endpoints for add/delete/reorder slides"
```

### Task 4.4: 大纲编辑 runner + 工具（AI 路径，删页走 needs_input）

**Files:**
- Create: `backend/internal/agent/outline/edit_runner.go`
- Create: `backend/internal/agent/outline/outline_tools.go`
- Modify: `backend/internal/agent/prompt/outline.go`（编辑 prompt）
- Modify: `backend/internal/service/run.go`（Outline 有大纲→edit_runner 路由）
- Test: `backend/internal/agent/outline/edit_runner_test.go`

**Interfaces:**
- Consumes: `harness.Loop`、`run.Prompter`（needs_input）、Phase 4.2 的 service 结构操作、Phase 2.3 的 PatchContent
- Produces:
  - 工具：`patch_outline_slide(slide_id, {...})`、`add_outline_slide({after_slide_id?,layout?,title?,bullets?})`、`delete_outline_slide(slide_id)`（内部经 prompter.NeedsInput 确认后才删）、`reorder_outline_slides([slide_id,...])`、`finish`
  - `outline.NewEditRunner(client, store, svc, params, ...) run.Runner`

- [ ] **Step 1: 写 runner 测试（先失败）**

用 `llmtest.Fake` 编排一次工具调用（如 patch_outline_slide 改标题）→ 断言 slide.json 更新 + 有 html 页置脏。再测 delete：Fake 调 delete_outline_slide → 断言发了 needs_input（用假 Prompter 返回"确认"）→ 页被删。

- [ ] **Step 2: 运行确认失败**

Run: `cd backend && go test ./internal/agent/outline/ -run TestEditRunner -v`
Expected: FAIL

- [ ] **Step 3: 实现工具 + runner**

`outline_tools.go`：四个工具，`Class()=ClassWrite`，`Scopes()` 返回一个新 scope 或复用（大纲编辑不依赖 scope 门控，可 `Scopes()=nil` 并只在 edit_runner 注册）。各工具 Execute 调用注入的 service 方法。`delete_outline_slide` 持有 `prompter run.Prompter`：Execute 内先 `prompter.NeedsInput(runID, question, []string{"确认删除","取消"})`，回答含"确认"才调 `svc.DeleteSlide`，否则返回未删 observation。
`edit_runner.go`：装配 harness.Loop（工具集含四工具 + finish），system/user prompt 来自 `prompt.OutlineEdit*`，注入当前大纲 slide-json 列表摘要。

- [ ] **Step 4: run.go 路由**

`buildRunner`/outline 分支：`r.Kind==KindOutline` 时，若 `len(slides)>0` → `outline.NewEditRunner(...)`；否则现有首次生成 runner。同时（若 Phase 3 前端仍把"有大纲的 Outline"映射成 overview）——本 Task 让后端支持 `kind=outline` 编辑；Task 4.6 让前端 Outline 有大纲时发 `kind=outline` 而非 overview。

- [ ] **Step 5: 通过 + Commit**

Run: `cd backend && go test ./internal/agent/outline/ -v && go vet ./...`
Expected: PASS

```bash
git add backend/internal/agent/outline backend/internal/agent/prompt/outline.go backend/internal/service/run.go
git commit -m "feat(outline-modes): AI outline-edit runner with structural tools and delete confirm"
```

### Task 4.5: 前端结构操作 API + Deck 交互

**Files:**
- Modify: `frontend/src/api/slides.ts`
- Modify: `frontend/src/features/deck/DeckNavigator.tsx`
- Test: `frontend/src/features/deck/DeckNavigator.test.tsx`

**Interfaces:**
- Produces: `slidesApi.add(projectId, {after_slide_id?, layout?})`、`slidesApi.remove(slideId)`、`slidesApi.reorder(projectId, orderedIds)`；Deck 加页按钮、删除(二次确认)、拖拽重排

- [ ] **Step 1: 写测试（先失败）**

断言：点 [+ 加页] 调 `slidesApi.add`；点删除弹确认，确认后调 `slidesApi.remove`；拖拽后调 `slidesApi.reorder`（可用 mock 验证调用参数）。

- [ ] **Step 2..4: 实现 + 通过 + Commit**

Run: `cd frontend && pnpm vitest run src/features/deck/ && pnpm tsc --noEmit`
Expected: PASS

```bash
git add frontend/src/api/slides.ts frontend/src/features/deck
git commit -m "feat(outline-modes): deck add/delete/reorder interactions"
```

### Task 4.6: Outline 模式路由到大纲编辑（前端）

**Files:**
- Modify: `frontend/src/features/agent/modeMapping.ts`
- Test: `frontend/src/features/agent/modeMapping.test.ts`

**Interfaces:**
- Produces: `interactionMode==='outline' && hasOutline && subMode!=='talk'` → `{kind:'outline', mode: subMode==='ask'?'ask':'normal', instruction}`（不再映射成 `kind:'edit', scope:'overview'`）

- [ ] **Step 1: 改测试预期（先失败）**

把现有"有大纲 Outline → overview"的用例改为断言 `kind==='outline'`。

- [ ] **Step 2: 运行确认失败**

Run: `cd frontend && pnpm vitest run src/features/agent/modeMapping.test.ts`
Expected: FAIL

- [ ] **Step 3: 改 mapModeToPayload**

`case 'outline'` 的"有大纲"分支：normal 返回 `{...base, kind:'outline', mode:'normal'}`；ask 返回 `{...base, kind:'outline', mode:'ask'}`（删掉映射成 overview 的旧逻辑）。

- [ ] **Step 4: 通过 + 全量 + Commit**

Run: `cd frontend && pnpm test && pnpm tsc --noEmit && pnpm build`
Expected: PASS

```bash
git add frontend/src/features/agent/modeMapping.ts frontend/src/features/agent/modeMapping.test.ts
git commit -m "feat(outline-modes): route outline mode with existing outline to outline-edit"
```

### Phase 4 验收

- 后端：`go vet ./... && go test ./...` 全绿。
- 前端：`pnpm test && pnpm tsc && pnpm build` 全绿。
- e2e 手动：Outline 模式下 AI"加一页总结/删第5页(弹确认)/把标题都加序号"生效；Deck 手动加/删(确认)/拖拽重排生效；结构改动后相关有 html 页置脏；活跃 run 时手动结构操作被拒。

---

## 自审记录

- **Spec 覆盖**：§4.1→Phase1；§4.2→Phase2；§4.3→Phase3；§4.4 脏标记→Phase2(置/清)+Phase3(展示)；§4.4 AI/手动编辑→Phase3(手动)+Phase4(AI runner)；§4.5 结构操作→Phase4。SOP(§5)由各阶段合成。
- **删页确认**：手动=前端二次确认(4.5)；AI=needs_input(4.4)。均覆盖。
- **RUN_ACTIVE 互斥**：PatchContent(2.4) + 结构操作(4.2) 统一 ErrRunActive→409。
- **版本快照**：大纲编辑不产版本（PatchContent/结构操作均不调 CreateVersion）；html 产物版本不变。
- **类型一致**：`SetSlideVersion(ctx, slideID, no)`、`SlideVersionTarget(projectID, slideID)`、`model.Slide{Order,OutlineDirty}`、`SlidePatch`、`slidesApi` 全程一致。

