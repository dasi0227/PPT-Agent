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

// m5FakeClient 按 scope/mode 自适应：
//   - overview 主循环：patch_design 改主色 → finish；
//   - talk：CallTool 返回纯文本（分析）；
//   - 其它：直接 finish。
type m5FakeClient struct{ designPatched bool }

func (c *m5FakeClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{Content: "改写后的清晰指令"}, nil
}
func (c *m5FakeClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (c *m5FakeClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	// talk：工具集只剩 finish（Gate 剥离写工具）→ 返回纯文本分析，触发 info + finished。
	names := map[string]bool{}
	for _, t := range req.Tools {
		names[t.Name] = true
	}
	if names["patch_design"] && !c.designPatched {
		c.designPatched = true
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{
			Name: "patch_design",
			Args: map[string]any{"edits": []any{map[string]any{"old_text": "#ff0000", "new_text": "#0047ff"}}},
		}}, nil
	}
	// talk 模式：无写工具，返回分析文本。
	if !names["patch_design"] && !names["patch_slide"] && !names["patch_asset"] {
		return llm.ToolCallResponse{Text: "深色方案的取舍分析……"}, nil
	}
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}}, nil
}

func setupM5Server(t *testing.T, pages int) (*httptest.Server, string, string) {
	t.Helper()
	work := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(work, "m5.db"), WorkRoot: work}
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
	if err := st.CreateThread(ctx, model.Thread{ID: "th1", ProjectID: "p1", HistoryPath: "x", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	metas := make([]model.Slide, pages)
	for i := 0; i < pages; i++ {
		id := fmt.Sprintf("%03d", i)
		html := fmt.Sprintf(`<!doctype html><html><head>`+
			`<link rel="stylesheet" href="../../common/tokens.css">`+
			`<link rel="stylesheet" href="../../common/base.css"></head>`+
			`<body><div class="slide-scaler"><section class="slide-stage">`+
			`<h1 class="slide-title">原标题%d</h1></section></div></body></html>`, i)
		writeAtE(t, workDir, model.SlideHTMLPath(id), html)
		metas[i] = model.Slide{ID: id, ProjectID: "p1", Idx: i, Order: i * 10, Layout: "content", Title: fmt.Sprintf("第%d页", i), JSONPath: model.SlideJSONPath(id), HTMLPath: model.SlideHTMLPath(id)}
	}
	if err := st.ReplaceSlides(ctx, "p1", metas); err != nil {
		t.Fatal(err)
	}
	writeV2ContextSources(t, workDir, "p1", metas)
	writeAtE(t, workDir, "common/tokens.css", ":root{--color-primary:#ff0000;}")
	writeAtE(t, workDir, "common/base.css", ".slide-stage{}")

	engine := run.NewEngine(st, run.NewLockManager(), nil, zap.NewNop())
	runSvc := service.NewRunService(st, engine, &m5FakeClient{}, service.WorkRoot(work))
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
	return srv, "th1", workDir
}

func hashM5Tree(t *testing.T, workDir string, pages int) map[string]string {
	t.Helper()
	out := map[string]string{}
	paths := []string{"common/tokens.css"}
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

func postM5Run(t *testing.T, srv *httptest.Server, threadID string, body map[string]any) (int, string) {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(srv.URL+"/api/v1/threads/"+threadID+"/runs", "application/json", strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	b := make([]byte, 4096)
	n, _ := resp.Body.Read(b)
	return resp.StatusCode, string(b[:n])
}

func runIDFrom(t *testing.T, body string) string {
	t.Helper()
	var out struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(body), &out)
	if out.ID == "" {
		t.Fatalf("no run id in %s", body)
	}
	return out.ID
}

// presentation/deck 全局修改：公共设计层变化，且每页写入新的物化来源元数据。
func TestE2EOverviewPatchDesign(t *testing.T) {
	srv, threadID, workDir := setupM5Server(t, 8)
	before := hashM5Tree(t, workDir, 8)

	code, body := postM5Run(t, srv, threadID, map[string]any{
		"target":      map[string]any{"artifact": "presentation", "level": "deck"},
		"interaction": map[string]any{"intent": "apply", "clarification": "when_blocked"},
		"instruction": "主色改成品牌蓝",
	})
	if code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", code, body)
	}
	waitDone(t, srv, runIDFrom(t, body))

	after := hashM5Tree(t, workDir, 8)
	if before["common/tokens.css"] == after["common/tokens.css"] {
		t.Error("tokens.css should change")
	}
	for i := 0; i < 8; i++ {
		key := fmt.Sprintf("slides/%03d/index.html", i)
		if before[key] == after[key] {
			t.Errorf("page %d should record the new deck materialization revision", i)
		}
	}
}

// consult → 无文件变更，且响应保留标准化 interaction。
func TestE2ETalkNoFileChange(t *testing.T) {
	srv, threadID, workDir := setupM5Server(t, 3)
	before := hashM5Tree(t, workDir, 3)

	code, body := postM5Run(t, srv, threadID, map[string]any{
		"target":      map[string]any{"artifact": "presentation", "level": "deck"},
		"interaction": map[string]any{"intent": "consult", "clarification": "never"},
		"instruction": "聊聊深色方案",
	})
	if code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", code, body)
	}
	if !strings.Contains(body, `"intent":"consult"`) {
		t.Errorf("run interaction should be consult: %s", body)
	}
	waitDone(t, srv, runIDFrom(t, body))

	after := hashM5Tree(t, workDir, 3)
	for k, v := range before {
		if after[k] != v {
			t.Errorf("talk MUST NOT change files (%s changed)", k)
		}
	}
}

// 每个 Run 显式携带 interaction，不继承上一条 consult。
func TestE2EModeNotPersisted(t *testing.T) {
	srv, threadID, _ := setupM5Server(t, 3)

	_, talkBody := postM5Run(t, srv, threadID, map[string]any{
		"target":      map[string]any{"artifact": "presentation", "level": "deck"},
		"interaction": map[string]any{"intent": "consult", "clarification": "never"},
		"instruction": "聊聊",
	})
	if !strings.Contains(talkBody, `"intent":"consult"`) {
		t.Fatalf("first run should be consult: %s", talkBody)
	}

	_, editBody := postM5Run(t, srv, threadID, map[string]any{
		"target":      map[string]any{"artifact": "presentation", "level": "slide", "slide_id": "000"},
		"interaction": map[string]any{"intent": "apply", "clarification": "when_blocked"},
		"instruction": "改标题",
	})
	if !strings.Contains(editBody, `"intent":"apply"`) {
		t.Errorf("second run must apply, not inherit consult: %s", editBody)
	}
}

// Blueprint consult 只给建议，不修改文件。
func TestE2EPromptNoFileChange(t *testing.T) {
	srv, threadID, workDir := setupM5Server(t, 2)
	before := hashM5Tree(t, workDir, 2)

	code, body := postM5Run(t, srv, threadID, map[string]any{
		"target":      map[string]any{"artifact": "blueprint", "level": "deck"},
		"interaction": map[string]any{"intent": "consult", "clarification": "never"},
		"instruction": "让这页好看点",
	})
	if code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", code, body)
	}
	waitDone(t, srv, runIDFrom(t, body))

	after := hashM5Tree(t, workDir, 2)
	for k, v := range before {
		if after[k] != v {
			t.Errorf("prompt MUST NOT change files (%s changed)", k)
		}
	}
}
