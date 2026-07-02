package httpapi_test

import (
	"context"
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

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

// genFakeClient：每页返回一次 write_slide(合规 html)，之后 finish（按 user 指令中的 slide_idx 区分页）。
type genFakeClient struct {
	seen map[int]bool
}

func (c *genFakeClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c *genFakeClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (c *genFakeClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	if c.seen == nil {
		c.seen = map[int]bool{}
	}
	idx := 0
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role != llm.RoleUser {
			continue
		}
		s := req.Messages[i].Content
		if k := strings.Index(s, "slide_idx="); k >= 0 {
			fmt.Sscanf(s[k+len("slide_idx="):], "%d", &idx)
			break
		}
	}
	if !c.seen[idx] {
		c.seen[idx] = true
		html := fmt.Sprintf(`<!doctype html><html><head>`+
			`<link rel="stylesheet" href="../../common/tokens.css">`+
			`<link rel="stylesheet" href="../../common/base.css"></head>`+
			`<body><div class="slide-scaler"><section class="slide-stage">`+
			`<h1 class="slide-title">页 %d</h1></section></div></body></html>`, idx)
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "write_slide", Args: map[string]any{"slide_idx": idx, "html": html}}}, nil
	}
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "done"}}}, nil
}

// AC-GEN-001 / AC-GEN-006（端到端）：POST generate run → SSE 收到逐页 progress → 各页合规 html 落盘。
func TestE2EGenerateDeck(t *testing.T) {
	workRoot := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(workRoot, "e2e.db"), WorkRoot: workRoot}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	// 冷启动 seeding：载入主题等资产（generate 读 seed 主题 tokens）。
	if err := asset.NewSeeder(st, workRoot, nil, nil).Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// project work_dir + slide.json（大纲已产出）。
	workDir := filepath.Join(workRoot, "p1")
	now := time.Now().Unix()
	if err := st.CreateProject(context.Background(), model.Project{ID: "p1", Title: "t", WorkDir: workDir, Theme: "tokyo-night", Status: "draft", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("project: %v", err)
	}
	if err := st.CreateThread(context.Background(), model.Thread{ID: "th1", ProjectID: "p1", HistoryPath: "threads/th1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("thread: %v", err)
	}
	layouts := []string{"cover", "bullets", "bullets", "thanks"}
	metas := make([]model.Slide, len(layouts))
	for i, l := range layouts {
		sj := slidejson.SlideJSON{ID: fmt.Sprint(i), Idx: i, Layout: l, Title: "T" + fmt.Sprint(i), Bullets: []string{"x"}}
		raw, _ := json.MarshalIndent(sj, "", "  ")
		dir := filepath.Join(workDir, fmt.Sprintf("slides/%03d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "slide.json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		metas[i] = model.Slide{ID: fmt.Sprint(i), ProjectID: "p1", Idx: i, Layout: l, Title: sj.Title,
			JSONPath: fmt.Sprintf("slides/%03d/slide.json", i), HTMLPath: fmt.Sprintf("slides/%03d/index.html", i)}
	}
	if err := st.ReplaceSlides(context.Background(), "p1", metas); err != nil {
		t.Fatalf("replace slides: %v", err)
	}

	// 真实工厂（kind=generate → generate.Runner）+ fake LLM。
	engine := run.NewEngine(st, run.NewLockManager(), zap.NewNop())
	runSvc := service.NewRunService(st, engine, &genFakeClient{})
	router := httpapi.NewRouter(cfg, zap.NewNop(), httpapi.NewHealthHandler(service.NewHealthService(st)), httpapi.NewRunHandler(runSvc))
	srv := newTestServer(t, router)

	runID := createGenerateRun(t, srv, "th1")
	events := readSSE(t, srv, runID, "", 0)

	// AC-GEN-006：逐页 progress（stage=page）事件数 == 页数。
	pageCount := 0
	for _, e := range events {
		if e.event == "progress" && strings.Contains(e.data, "\"stage\":\"page\"") {
			pageCount++
		}
	}
	if pageCount != len(layouts) {
		t.Errorf("want %d page progress events, got %d (%v)", len(layouts), pageCount, eventTypes(events))
	}
	if countEvent(events, "done") != 1 {
		t.Errorf("want exactly 1 done, got: %v", eventTypes(events))
	}

	// AC-GEN-001：各页 html 落盘且通过 lint-slide；公共层写入。
	for i := range layouts {
		p := filepath.Join(workDir, fmt.Sprintf("slides/%03d/index.html", i))
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("page %d html missing: %v", i, err)
		}
		if ok, reason := designsystem.LintSlideResult(raw); !ok {
			t.Errorf("page %d html failed lint: %s", i, reason)
		}
	}
	if _, err := os.ReadFile(filepath.Join(workDir, "common/tokens.css")); err != nil {
		t.Errorf("common/tokens.css not written: %v", err)
	}
}

func newTestServer(t *testing.T, router *httpapi.Router) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv
}

func createGenerateRun(t *testing.T, srv *httptest.Server, threadID string) string {
	t.Helper()
	body := `{"kind":"generate","scope":"current","mode":"normal","theme":"tokyo-night","instruction":"生成"}`
	resp, err := http.Post(srv.URL+"/api/v1/threads/"+threadID+"/runs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create run status %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if out.ID == "" {
		t.Fatal("empty run id")
	}
	return out.ID
}
