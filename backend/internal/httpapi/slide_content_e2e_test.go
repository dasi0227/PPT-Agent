package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func setupSlideContentServer(t *testing.T) (*httptest.Server, *sqlitestore.Store, string) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(root, "sc.db"), WorkRoot: root}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	slideSvc := service.NewSlideService(st)
	engine := run.NewEngine(st, run.NewLockManager(), nil, zap.NewNop())
	runSvc := service.NewRunServiceWithFactory(st, engine, func(model.Run, model.CreateRunParams, model.Project) run.Runner {
		return noOpRunner{}
	})
	router := httpapi.NewRouter(
		cfg, zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewProjectHandler(service.NewProjectService(st, service.WorkRoot(root)), slideSvc),
		httpapi.NewThreadHandler(service.NewThreadService(st)),
		httpapi.NewSlideHandler(slideSvc),
		httpapi.NewAssetHandler(service.NewAssetService(st, root)),
	)
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, st, root
}

func TestSlideContentReadPatchAndRunActive(t *testing.T) {
	srv, st, root := setupSlideContentServer(t)
	ctx := t.Context()
	now := time.Now().Unix()
	workDir := filepath.Join(root, "p1")
	if err := st.CreateProject(ctx, model.Project{ID: "p1", Title: "t", WorkDir: workDir, Theme: "x", Status: "draft", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceSlides(ctx, "p1", []model.Slide{{ID: "s1", ProjectID: "p1", Order: 10, Layout: "bullets", Title: "标题",
		JSONPath: model.SlideJSONPath("s1"), HTMLPath: model.SlideHTMLPath("s1")}}); err != nil {
		t.Fatal(err)
	}
	writeAtE(t, workDir, model.SlideJSONPath("s1"), `{"id":"s1","layout":"bullets","title":"标题","bullets":["a","b"]}`)

	// GET /slides/s1 → content.bullets、outline_dirty=false、order。
	resp := apiReq(t, http.MethodGet, srv.URL+"/api/v1/slides/s1", "")
	if resp.Code != http.StatusOK {
		t.Fatalf("GET slide want 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["outline_dirty"] != false {
		t.Fatalf("want outline_dirty=false, got %v", got["outline_dirty"])
	}
	if got["order"].(float64) != 10 {
		t.Fatalf("want order=10, got %v", got["order"])
	}
	content, ok := got["content"].(map[string]any)
	if !ok {
		t.Fatalf("want content object, got %v", got["content"])
	}
	if bullets, ok := content["bullets"].([]any); !ok || len(bullets) != 2 {
		t.Fatalf("want content.bullets len 2, got %v", content["bullets"])
	}

	// PATCH /slides/s1 {title:X} → 200, title==X。
	resp = apiReq(t, http.MethodPatch, srv.URL+"/api/v1/slides/s1", `{"title":"X"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH slide want 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var patched map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched["title"] != "X" {
		t.Fatalf("want title=X after patch, got %v", patched["title"])
	}

	// 造一个 running run → PATCH → 409 RUN_ACTIVE。
	if err := st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRun(ctx, model.Run{ID: "r1", ProjectID: "p1", ThreadID: "t1", Kind: model.KindOutline, Scope: model.ScopeCurrent, Mode: model.ModeNormal, Status: model.RunRunning}); err != nil {
		t.Fatal(err)
	}
	resp = apiReq(t, http.MethodPatch, srv.URL+"/api/v1/slides/s1", `{"title":"Y"}`)
	if resp.Code != http.StatusConflict {
		t.Fatalf("PATCH during active run want 409, got %d: %s", resp.Code, resp.Body.String())
	}
	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(resp.Body.Bytes(), &errBody)
	if errBody.Error.Code != "RUN_ACTIVE" {
		t.Fatalf("want error code RUN_ACTIVE, got %q (%s)", errBody.Error.Code, resp.Body.String())
	}
}

func TestSlideStructuralAddDeleteReorder(t *testing.T) {
	srv, st, root := setupSlideContentServer(t)
	ctx := t.Context()
	now := time.Now().Unix()
	workDir := filepath.Join(root, "p1")
	if err := st.CreateProject(ctx, model.Project{ID: "p1", Title: "t", WorkDir: workDir, Theme: "x", Status: "draft", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceSlides(ctx, "p1", []model.Slide{
		{ID: "a", ProjectID: "p1", Idx: 0, Order: 10, Layout: "cover", Title: "A", JSONPath: model.SlideJSONPath("a"), HTMLPath: model.SlideHTMLPath("a")},
		{ID: "b", ProjectID: "p1", Idx: 1, Order: 20, Layout: "thanks", Title: "B", JSONPath: model.SlideJSONPath("b"), HTMLPath: model.SlideHTMLPath("b")},
	}); err != nil {
		t.Fatal(err)
	}

	// POST /projects/p1/slides {after_slide_id:a, layout:bullets} → 201 新页。
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/p1/slides", `{"after_slide_id":"a","layout":"bullets"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST slide want 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &created)
	newID, _ := created["id"].(string)
	if newID == "" {
		t.Fatalf("new slide id empty: %s", resp.Body.String())
	}
	list, _ := st.ListSlides(ctx, "p1")
	if len(list) != 3 || list[1].ID != newID {
		t.Fatalf("insert order wrong: %+v", list)
	}

	// POST /projects/p1/slides/reorder {ordered_ids:[b,newID,a]} → 200。
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/p1/slides/reorder", `{"ordered_ids":["b","`+newID+`","a"]}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("reorder want 200, got %d: %s", resp.Code, resp.Body.String())
	}
	list, _ = st.ListSlides(ctx, "p1")
	if list[0].ID != "b" || list[1].ID != newID || list[2].ID != "a" {
		t.Fatalf("reorder result wrong: %+v", list)
	}

	// DELETE /slides/{newID} → 204。
	resp = apiReq(t, http.MethodDelete, srv.URL+"/api/v1/slides/"+newID, "")
	if resp.Code != http.StatusNoContent {
		t.Fatalf("delete want 204, got %d: %s", resp.Code, resp.Body.String())
	}
	list, _ = st.ListSlides(ctx, "p1")
	if len(list) != 2 {
		t.Fatalf("delete result wrong: %+v", list)
	}

	// 活跃 run → 结构操作 409 RUN_ACTIVE。
	if err := st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRun(ctx, model.Run{ID: "r1", ProjectID: "p1", ThreadID: "t1", Kind: model.KindOutline, Scope: model.ScopeCurrent, Mode: model.ModeNormal, Status: model.RunRunning}); err != nil {
		t.Fatal(err)
	}
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/p1/slides", `{"after_slide_id":"a","layout":"bullets"}`)
	if resp.Code != http.StatusConflict {
		t.Fatalf("add during active run want 409, got %d: %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodDelete, srv.URL+"/api/v1/slides/a", "")
	if resp.Code != http.StatusConflict {
		t.Fatalf("delete during active run want 409, got %d: %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/p1/slides/reorder", `{"ordered_ids":["a","b"]}`)
	if resp.Code != http.StatusConflict {
		t.Fatalf("reorder during active run want 409, got %d: %s", resp.Code, resp.Body.String())
	}
}
