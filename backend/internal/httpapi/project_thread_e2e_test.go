package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func setupProjectThreadServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(root, "api.db"), WorkRoot: root}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	engine := run.NewEngine(st, run.NewLockManager(), zap.NewNop())
	runSvc := service.NewRunServiceWithFactory(st, engine, func(model.Run, model.CreateRunParams, model.Project) run.Runner {
		return noOpRunner{}
	})
	projectSvc := service.NewProjectService(st, service.WorkRoot(root))
	threadSvc := service.NewThreadService(st)
	router := httpapi.NewRouter(
		cfg,
		zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewProjectHandler(projectSvc),
		httpapi.NewThreadHandler(threadSvc),
		httpapi.NewSlideHandler(service.NewSlideService(st)),
		httpapi.NewAssetHandler(service.NewAssetService(st, root)),
	)
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, root
}

func TestProjectThreadAPIClosesRunCreationLoop(t *testing.T) {
	srv, root := setupProjectThreadServer(t)

	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"云原生可观测性实践","language":"zh"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /projects want 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	projectID, _ := project["id"].(string)
	workDir, _ := project["work_dir"].(string)
	if projectID == "" || !strings.HasPrefix(workDir, root) {
		t.Fatalf("bad project response: %+v", project)
	}
	if _, err := os.Stat(filepath.Join(workDir, "state.json")); err != nil {
		t.Fatalf("project creation must initialize state.json: %v", err)
	}

	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID+"/slides", "")
	if resp.Code != http.StatusOK || strings.TrimSpace(resp.Body.String()) != "[]" {
		t.Fatalf("new project slides should be empty array, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{"title":"内容打磨"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /projects/{id}/threads want 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var thread map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &thread); err != nil {
		t.Fatalf("decode thread: %v", err)
	}
	threadID, _ := thread["id"].(string)
	if threadID == "" || thread["project_id"] != projectID {
		t.Fatalf("bad thread response: %+v", thread)
	}

	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID+"/threads", "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), threadID) {
		t.Fatalf("GET project threads failed: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/threads/"+threadID+"/history", "")
	if resp.Code != http.StatusOK || strings.TrimSpace(resp.Body.String()) != "[]" {
		t.Fatalf("new thread history should be empty array, got %d: %s", resp.Code, resp.Body.String())
	}

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", `{"kind":"outline","instruction":"生成大纲"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /threads/{id}/runs should work with API-created thread, got %d: %s", resp.Code, resp.Body.String())
	}
}

func apiReq(t *testing.T, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	rec.Code = resp.StatusCode
	_, _ = rec.Body.ReadFrom(resp.Body)
	return rec
}
