package httpapi_test

import (
	"context"
	"encoding/json"
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

type m6MountClient struct {
	step          int
	sawSearchTool bool
	sawMountTool  bool
}

func (c *m6MountClient) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (c *m6MountClient) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (c *m6MountClient) CallTool(_ context.Context, req llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	names := map[string]bool{}
	for _, tl := range req.Tools {
		names[tl.Name] = true
	}
	c.sawSearchTool = c.sawSearchTool || names["search_assets"]
	c.sawMountTool = c.sawMountTool || names["mount_asset"]
	switch c.step {
	case 0:
		c.step++
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "search_assets", Args: map[string]any{
			"kind": "fx", "query": "particle",
		}}}, nil
	case 1:
		c.step++
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "mount_asset", Args: map[string]any{
			"slide_idx": 0, "asset_id": "fx-particle",
		}}}, nil
	default:
		return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "mounted"}}}, nil
	}
}

func TestE2ERepoSearchAndMountAsset(t *testing.T) {
	client := &m6MountClient{}
	srv, threadID, workDir, store := setupM6MountServer(t, client)

	runID := createM6Run(t, srv, threadID, map[string]any{
		"kind": "edit", "scope": "current", "mode": "normal", "page_index": 0,
		"instruction": "给封面加上粒子特效",
	})
	waitDone(t, srv, runID)

	if !client.sawSearchTool || !client.sawMountTool {
		t.Fatalf("LLM should see search_assets and mount_asset, saw search=%v mount=%v", client.sawSearchTool, client.sawMountTool)
	}
	raw, _ := os.ReadFile(filepath.Join(workDir, "slides/000/index.html"))
	html := string(raw)
	if !strings.Contains(html, `data-fx="particle-burst"`) || !strings.Contains(html, `import { init }`) {
		t.Fatalf("particle fx not mounted into slide: %s", html)
	}
	versions, err := store.ListVersions(context.Background(), "slide", model.SlideVersionTarget("p1", 0))
	if err != nil {
		t.Fatalf("list slide versions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("mount_asset should create one slide version, got %+v", versions)
	}
}

func setupM6MountServer(t *testing.T, client llm.Client) (*httptest.Server, string, string, *sqlitestore.Store) {
	t.Helper()
	work := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(work, "m6.db"), WorkRoot: work}
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
	slideHTML := `<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/tokens.css">` +
		`<link rel="stylesheet" href="../../common/base.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage">` +
		`<div class="slide-content"><h1 class="slide-title">封面</h1></div>` +
		`</section></div></body></html>`
	writeAtE(t, workDir, "slides/000/index.html", slideHTML)
	writeAtE(t, workDir, "common/tokens.css", ":root{}")
	writeAtE(t, workDir, "common/base.css", ".slide-stage{}")
	if err := st.ReplaceSlides(ctx, "p1", []model.Slide{{
		ID: "s0", ProjectID: "p1", Idx: 0, Layout: "cover", Title: "封面",
		JSONPath: "slides/000/slide.json", HTMLPath: "slides/000/index.html",
	}}); err != nil {
		t.Fatal(err)
	}

	assetDir := "_assets/fx/particle-burst"
	writeAtE(t, work, assetDir+"/manifest.json", `{
  "name":"particle-burst","version":"1.0.0","kind":"fx","source":"preset",
  "description":"粒子特效","tags":["particle"],"mount":{"target":"[data-fx=\"particle-burst\"]","position":"append"},
  "assets":{"js":"effect.js"}
}`)
	writeAtE(t, work, assetDir+"/effect.js", `export function init(root){ return root; }`)
	if err := st.CreateAsset(ctx, model.Asset{
		ID: "fx-particle", Name: "particle-burst", Kind: "fx", Source: "preset", Description: "粒子特效",
		Tags: []string{"particle"}, ManifestPath: assetDir + "/manifest.json", Dir: assetDir,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	engine := run.NewEngine(st, run.NewLockManager(), zap.NewNop())
	runSvc := service.NewRunService(st, engine, client, service.WorkRoot(work))
	assetSvc := service.NewAssetService(st, work)
	router := httpapi.NewRouter(
		cfg, zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewProjectHandler(service.NewProjectService(st, service.WorkRoot(work))),
		httpapi.NewThreadHandler(service.NewThreadService(st)),
		httpapi.NewSlideHandler(service.NewSlideService(st)),
		httpapi.NewAssetHandler(assetSvc),
	)
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, "th1", workDir, st
}

func createM6Run(t *testing.T, srv *httptest.Server, threadID string, body map[string]any) string {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(srv.URL+"/api/v1/threads/"+threadID+"/runs", "application/json", strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("post run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		buf := make([]byte, 4096)
		n, _ := resp.Body.Read(buf)
		t.Fatalf("create run want 201, got %d (%s)", resp.StatusCode, string(buf[:n]))
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
