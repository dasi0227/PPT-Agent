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
		html := fmt.Sprintf(`<!doctype html><html><head>`+
			`<link rel="stylesheet" href="../../common/tokens.css">`+
			`<link rel="stylesheet" href="../../common/base.css"></head>`+
			`<body><div class="slide-scaler"><section class="slide-stage">`+
			`<h1 class="slide-title">原标题%d</h1></section></div></body></html>`, i)
		writeAtE(t, workDir, fmt.Sprintf("slides/%03d/index.html", i), html)
		metas[i] = model.Slide{ID: fmt.Sprintf("s%d", i), ProjectID: "p1", Idx: i, Layout: "content", Title: fmt.Sprintf("第%d页", i), JSONPath: fmt.Sprintf("slides/%03d/slide.json", i), HTMLPath: fmt.Sprintf("slides/%03d/index.html", i)}
	}
	if err := st.ReplaceSlides(ctx, "p1", metas); err != nil {
		t.Fatal(err)
	}
	writeAtE(t, workDir, "common/tokens.css", ":root{--color-primary:#ff0000;}")
	writeAtE(t, workDir, "common/base.css", ".slide-stage{}")

	engine := run.NewEngine(st, run.NewLockManager(), zap.NewNop())
	runSvc := service.NewRunService(st, engine, &m5FakeClient{}, service.WorkRoot(work))
	router := httpapi.NewRouter(cfg, zap.NewNop(), httpapi.NewHealthHandler(service.NewHealthService(st)), httpapi.NewRunHandler(runSvc), httpapi.NewSlideHandler(service.NewSlideService(st)), httpapi.NewAssetHandler(service.NewAssetService(st, work)))
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
		ID   string `json:"id"`
		Mode string `json:"mode"`
	}
	json.Unmarshal([]byte(body), &out)
	if out.ID == "" {
		t.Fatalf("no run id in %s", body)
	}
	return out.ID
}

// AC-EDIT-004 / AC-CMD-OVERVIEW-001（端到端）：/overview 改主色 → 仅 tokens.css 变，8 页 html 不变。
func TestE2EOverviewPatchDesign(t *testing.T) {
	srv, threadID, workDir := setupM5Server(t, 8)
	before := hashM5Tree(t, workDir, 8)

	code, body := postM5Run(t, srv, threadID, map[string]any{
		"kind": "edit", "scope": "overview", "mode": "normal", "instruction": "主色改成品牌蓝",
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
		if before[key] != after[key] {
			t.Errorf("page %d html must not change (AC-EDIT-004)", i)
		}
	}
}

// AC-CMD-TALK-001（端到端）：/talk → 无文件变更；run.mode=talk。
func TestE2ETalkNoFileChange(t *testing.T) {
	srv, threadID, workDir := setupM5Server(t, 3)
	before := hashM5Tree(t, workDir, 3)

	code, body := postM5Run(t, srv, threadID, map[string]any{
		"kind": "command", "scope": "current", "mode": "talk", "command": "talk", "instruction": "聊聊深色方案",
	})
	if code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", code, body)
	}
	// run.mode 暴露为 talk（AGENT-MODE-004）。
	if !strings.Contains(body, `"mode":"talk"`) {
		t.Errorf("run.mode should be talk: %s", body)
	}
	waitDone(t, srv, runIDFrom(t, body))

	after := hashM5Tree(t, workDir, 3)
	for k, v := range before {
		if after[k] != v {
			t.Errorf("talk MUST NOT change files (%s changed)", k)
		}
	}
}

// AC-MODE-005（端到端）：一次 /talk 后，下一条普通指令为 normal，不继承 talk。
func TestE2EModeNotPersisted(t *testing.T) {
	srv, threadID, _ := setupM5Server(t, 3)

	_, talkBody := postM5Run(t, srv, threadID, map[string]any{
		"kind": "command", "scope": "current", "mode": "talk", "command": "talk", "instruction": "聊聊",
	})
	if !strings.Contains(talkBody, `"mode":"talk"`) {
		t.Fatalf("first run should be talk: %s", talkBody)
	}

	// 下一条普通编辑指令（不带 mode）→ normal。
	_, editBody := postM5Run(t, srv, threadID, map[string]any{
		"kind": "edit", "scope": "current", "mode": "normal", "page_index": 0, "instruction": "改标题",
	})
	if !strings.Contains(editBody, `"mode":"normal"`) {
		t.Errorf("second run must be normal, not inherit talk (AC-MODE-005): %s", editBody)
	}
}

// AC-CMD-PROMPT-001（端到端）：/prompt → 无文件变更（改写只发 info）。
func TestE2EPromptNoFileChange(t *testing.T) {
	srv, threadID, workDir := setupM5Server(t, 2)
	before := hashM5Tree(t, workDir, 2)

	code, body := postM5Run(t, srv, threadID, map[string]any{
		"kind": "command", "scope": "current", "mode": "normal", "command": "prompt", "instruction": "让这页好看点",
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
