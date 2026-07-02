package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestCreateRunParsesRawCommandInstruction(t *testing.T) {
	srv, got := setupCommandParseServer(t)

	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/th1/runs", `{"kind":"edit","instruction":"/page 3 改标题"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("raw /page run should be created, got %d: %s", resp.Code, resp.Body.String())
	}
	if got.run.Scope != model.ScopePage || got.run.PageIndex == nil || *got.run.PageIndex != 3 {
		t.Fatalf("raw /page must set scope/page_index, got %+v", got.run)
	}
	if got.params.Instruction != "改标题" {
		t.Fatalf("instruction should strip command prefix, got %q", got.params.Instruction)
	}

	combo := apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/th1/runs", `{"kind":"edit","instruction":"/page 3 /overview 改"}`)
	if combo.Code != http.StatusBadRequest {
		t.Fatalf("combo command should be rejected with 400, got %d: %s", combo.Code, combo.Body.String())
	}
	if !strings.Contains(combo.Body.String(), `"code":"BAD_REQUEST"`) {
		t.Fatalf("combo command should use BAD_REQUEST body, got %s", combo.Body.String())
	}

	unknown := apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/th1/runs", `{"kind":"edit","instruction":"/bogus 改"}`)
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown command should be rejected with 400, got %d: %s", unknown.Code, unknown.Body.String())
	}
}

type capturedRunCreate struct {
	run    model.Run
	params model.CreateRunParams
}

func setupCommandParseServer(t *testing.T) (*httptest.Server, *capturedRunCreate) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(root, "cmd.db"), WorkRoot: root}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	now := time.Now().Unix()
	if err := st.CreateProject(context.Background(), model.Project{ID: "p1", Title: "t", WorkDir: filepath.Join(root, "p1"), Theme: "default", Status: "draft", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := st.CreateThread(context.Background(), model.Thread{ID: "th1", ProjectID: "p1", HistoryPath: "threads/th1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("seed thread: %v", err)
	}
	var slides []model.Slide
	for i := 0; i < 4; i++ {
		slides = append(slides, model.Slide{
			ID: "s" + string(rune('0'+i)), ProjectID: "p1", Idx: i, Layout: "bullets", Title: "T",
			JSONPath: "slides/000/slide.json", HTMLPath: "slides/000/index.html",
		})
	}
	if err := st.ReplaceSlides(context.Background(), "p1", slides); err != nil {
		t.Fatalf("seed slides: %v", err)
	}

	captured := &capturedRunCreate{}
	engine := run.NewEngine(st, run.NewLockManager(), zap.NewNop())
	runSvc := service.NewRunServiceWithFactory(st, engine, func(r model.Run, p model.CreateRunParams, _ model.Project) run.Runner {
		captured.run = r
		captured.params = p
		return noOpRunner{}
	})
	router := httpapi.NewRouter(
		cfg, zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewProjectHandler(service.NewProjectService(st, service.WorkRoot(root))),
		httpapi.NewThreadHandler(service.NewThreadService(st)),
		httpapi.NewSlideHandler(service.NewSlideService(st)),
		httpapi.NewAssetHandler(service.NewAssetService(st, root)),
	)
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, captured
}
