package httpapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

// editFakeClient：一次 patch_slide（把"原标题N"→"新标题N"）+ finish，slide_idx 从 user 指令解析。
type editFakeClient struct{ done bool }

func (c *editFakeClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c *editFakeClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (c *editFakeClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	if c.done {
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}}, nil
	}
	c.done = true
	idx := 0
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role != llm.RoleUser {
			continue
		}
		if k := strings.Index(req.Messages[i].Content, "slide_idx="); k >= 0 {
			fmt.Sscanf(req.Messages[i].Content[k+len("slide_idx="):], "%d", &idx)
			break
		}
	}
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{
		Name: "patch_slide",
		Args: map[string]any{
			"slide_idx": idx,
			"edits": []any{map[string]any{
				"old_text": fmt.Sprintf("原标题%d", idx), "new_text": fmt.Sprintf("新标题%d", idx),
			}},
		},
	}}, nil
}

// setupEditServer 建库 + 项目 + N 页真实 html（含可锚定标题），返回 server + threadID + workDir + slide IDs。
func setupEditServer(t *testing.T, pages int, client llm.Client) (*httptest.Server, string, string, []string) {
	t.Helper()
	work := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(work, "e.db"), WorkRoot: work}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	ctx := context.Background()
	now := time.Now().Unix()
	workDir := filepath.Join(work, "p1")
	if err := st.CreateProject(ctx, model.Project{ID: "p1", Title: "t", WorkDir: workDir, Theme: "tokyo-night", Status: "ready", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{ID: "th1", ProjectID: "p1", HistoryPath: "threads/th1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	metas := make([]model.Slide, pages)
	ids := make([]string, pages)
	for i := 0; i < pages; i++ {
		id := fmt.Sprintf("%03d", i)
		html := fmt.Sprintf(`<!doctype html><html><head>`+
			`<link rel="stylesheet" href="../../common/tokens.css">`+
			`<link rel="stylesheet" href="../../common/base.css"></head>`+
			`<body><div class="slide-scaler"><section class="slide-stage">`+
			`<h1 class="slide-title">原标题%d</h1></section></div></body></html>`, i)
		writeAtE(t, workDir, model.SlideHTMLPath(id), html)
		ids[i] = id
		metas[i] = model.Slide{ID: id, ProjectID: "p1", Idx: i, Order: i * 10, Layout: "bullets", Title: "T",
			JSONPath: model.SlideJSONPath(id), HTMLPath: model.SlideHTMLPath(id)}
	}
	if err := st.ReplaceSlides(ctx, "p1", metas); err != nil {
		t.Fatal(err)
	}
	// 写公共层，用于 hash 隔离断言。
	writeAtE(t, workDir, "common/tokens.css", ":root{--color-bg:#000}")
	writeAtE(t, workDir, "common/base.css", ".slide-stage{}")

	engine := run.NewEngine(st, run.NewLockManager(), zap.NewNop())
	runSvc := service.NewRunService(st, engine, client, service.WorkRoot(workDir))
	router := httpapi.NewRouter(
		cfg, zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewProjectHandler(service.NewProjectService(st, service.WorkRoot(work)), service.NewSlideService(st)),
		httpapi.NewThreadHandler(service.NewThreadService(st)),
		httpapi.NewSlideHandler(service.NewSlideService(st)),
		httpapi.NewAssetHandler(service.NewAssetService(st, work)),
	)
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, "th1", workDir, ids
}

// AC-EDIT-003 / AC-CMD-PAGE-001：/page 3 编辑 → 仅第 3 页 html 变，公共层+别页 hash 不变。
func TestE2EEditPageIsolation(t *testing.T) {
	srv, threadID, workDir, _ := setupEditServer(t, 8, &editFakeClient{})
	before := hashEditTree(t, workDir, 8)

	runID := createEditRun(t, srv, threadID, model.ScopePage, intPtr(3), "把标题改大")
	waitDone(t, srv, runID)

	after := hashEditTree(t, workDir, 8)
	for path, h := range before {
		changed := after[path] != h
		if path == "slides/003/index.html" {
			if !changed {
				t.Errorf("target page %s should change", path)
			}
			continue
		}
		if changed {
			t.Errorf("non-target %s changed on page-scope edit (AC-EDIT-003 violated)", path)
		}
	}
}

// AC-CMD-CURRENT-001：无 scope 指令（current）+ 上报 page_index → 等价 /page，仅该页变。
func TestE2EEditCurrentScope(t *testing.T) {
	srv, threadID, workDir, _ := setupEditServer(t, 5, &editFakeClient{})
	before := hashEditTree(t, workDir, 5)

	runID := createEditRun(t, srv, threadID, model.ScopeCurrent, intPtr(2), "把标题改大")
	waitDone(t, srv, runID)

	after := hashEditTree(t, workDir, 5)
	for path, h := range before {
		if path == "slides/002/index.html" {
			if after[path] == h {
				t.Errorf("current-scope target page %s should change", path)
			}
			continue
		}
		if after[path] != h {
			t.Errorf("non-target %s changed on current-scope edit", path)
		}
	}
}

// AC-CMD-PAGE-003：/page 99 越界 → 400 BAD_REQUEST，无文件变更、无 Run。
func TestE2EEditPageOutOfBounds(t *testing.T) {
	srv, threadID, workDir, _ := setupEditServer(t, 8, &editFakeClient{})
	before := hashEditTree(t, workDir, 8)

	code, body := postEditRun(t, srv, threadID, model.ScopePage, intPtr(99), "改")
	if code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", code, body)
	}
	after := hashEditTree(t, workDir, 8)
	for path, h := range before {
		if after[path] != h {
			t.Errorf("no file must change on out-of-bounds page (%s changed)", path)
		}
	}
}

// current scope 缺 page_index → 400。
func TestE2EEditCurrentMissingPageIndex(t *testing.T) {
	srv, threadID, _, _ := setupEditServer(t, 3, &editFakeClient{})
	code, _ := postEditRun(t, srv, threadID, model.ScopeCurrent, nil, "改")
	if code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing page_index, got %d", code)
	}
}

// AC-VERSION-004（端到端）：编辑产版本后，POST /slides/{id}/rollback 到 v0 → 200，内容回滚。
func TestE2ERollbackEndpoint(t *testing.T) {
	srv, threadID, workDir, ids := setupEditServer(t, 3, &editFakeClient{})

	// 先编辑第 1 页产生 v0（编辑本身产版本）。
	runID := createEditRun(t, srv, threadID, model.ScopePage, intPtr(1), "改标题")
	waitDone(t, srv, runID)
	edited, _ := os.ReadFile(filepath.Join(workDir, "slides/001/index.html"))
	if !strings.Contains(string(edited), "新标题1") {
		t.Fatalf("edit did not apply: %s", edited)
	}

	// 回滚到 v0（编辑产生的快照，内容=编辑后的"新标题1"）——验证端点连通 + 产新版本。
	code, body := postRollback(t, srv, ids[1], 0)
	if code != http.StatusOK {
		t.Fatalf("rollback want 200, got %d (%s)", code, body)
	}
	// 版本列表应含 v0 与 v1（回滚新增）。
	versions := getVersions(t, srv, ids[1])
	if len(versions) < 2 {
		t.Errorf("expected >=2 versions after edit+rollback, got %d", len(versions))
	}
}

// ── helpers ─────────────────────────────────────────

func createEditRun(t *testing.T, srv *httptest.Server, threadID string, scope model.Scope, pageIdx *int, instr string) string {
	t.Helper()
	code, body := postEditRun(t, srv, threadID, scope, pageIdx, instr)
	if code != http.StatusCreated {
		t.Fatalf("create edit run want 201, got %d (%s)", code, body)
	}
	var out struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(body), &out)
	if out.ID == "" {
		t.Fatal("empty run id")
	}
	return out.ID
}

func postEditRun(t *testing.T, srv *httptest.Server, threadID string, scope model.Scope, pageIdx *int, instr string) (int, string) {
	t.Helper()
	payload := map[string]any{"kind": "edit", "scope": string(scope), "mode": "normal", "instruction": instr}
	if pageIdx != nil {
		payload["page_index"] = *pageIdx
	}
	raw, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/v1/threads/"+threadID+"/runs", "application/json", strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("post run: %v", err)
	}
	defer resp.Body.Close()
	b := make([]byte, 4096)
	n, _ := resp.Body.Read(b)
	return resp.StatusCode, string(b[:n])
}

func postRollback(t *testing.T, srv *httptest.Server, slideID string, versionNo int) (int, string) {
	t.Helper()
	body := fmt.Sprintf(`{"version_no":%d}`, versionNo)
	resp, err := http.Post(srv.URL+"/api/v1/slides/"+slideID+"/rollback", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post rollback: %v", err)
	}
	defer resp.Body.Close()
	b := make([]byte, 4096)
	n, _ := resp.Body.Read(b)
	return resp.StatusCode, string(b[:n])
}

func getVersions(t *testing.T, srv *httptest.Server, slideID string) []map[string]any {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/slides/" + slideID + "/versions")
	if err != nil {
		t.Fatalf("get versions: %v", err)
	}
	defer resp.Body.Close()
	var out []map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return out
}

// waitDone 订阅 SSE 直到终态（复用 readSSE）。
func waitDone(t *testing.T, srv *httptest.Server, runID string) {
	t.Helper()
	events := readSSE(t, srv, runID, "", 0)
	if countEvent(events, "done") != 1 {
		t.Fatalf("expected done event, got: %v", eventTypes(events))
	}
}

func hashEditTree(t *testing.T, workDir string, pages int) map[string]string {
	t.Helper()
	out := map[string]string{}
	paths := []string{"common/tokens.css", "common/base.css"}
	for i := 0; i < pages; i++ {
		paths = append(paths, fmt.Sprintf("slides/%03d/index.html", i))
	}
	for _, p := range paths {
		raw, err := os.ReadFile(filepath.Join(workDir, p))
		if err != nil {
			out[p] = "MISSING"
			continue
		}
		sum := sha256.Sum256(raw)
		out[p] = fmt.Sprintf("%x", sum)
	}
	return out
}

func writeAtE(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func intPtr(n int) *int { return &n }
