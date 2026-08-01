package httpapi_test

import (
	"bytes"
	"context"
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
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type noOpRunner struct{}

func (noOpRunner) Run(context.Context, workflow.EventEmitter, run.Checkpointer, run.Prompter) workflow.StructuredOutcome {
	return workflow.StructuredOutcome{Status: workflow.StatusCompleted, Strategy: workflow.StrategyRespond}
}

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
	engine := run.NewEngine(st, run.NewLockManager(), nil, zap.NewNop())
	runSvc := service.NewRunServiceWithExecutionFactory(st, engine, func(model.Run, model.CreateRunParams, model.Project) run.Execution {
		return noOpRunner{}
	})
	projectSvc := service.NewProjectService(st, service.WorkRoot(root))
	threadSvc := service.NewThreadService(st)
	router := httpapi.NewRouter(
		cfg,
		zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewProjectHandler(projectSvc, service.NewSlideService(st)),
		httpapi.NewThreadHandler(threadSvc),
		httpapi.NewSlideHandler(service.NewSlideService(st)),
		httpapi.NewAssetHandler(service.NewAssetService(st, root)),
		httpapi.NewBlueprintHandler(service.NewBlueprintService(st)),
	)
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, root
}

func TestArtifactTargetRunAndBlueprintAPI(t *testing.T) {
	srv, _ := setupProjectThreadServer(t)
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Artifact R0","language":"zh-CN"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &project)
	projectID := project["id"].(string)

	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID+"/blueprint", "")
	if resp.Code != http.StatusOK {
		t.Fatalf("get blueprint: %d %s", resp.Code, resp.Body.String())
	}
	var view map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &view)
	if view["deck"].(map[string]any)["schema_version"] != "2.0" {
		t.Fatalf("unexpected blueprint response: %s", resp.Body.String())
	}

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{"title":"R0"}`)
	var thread map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &thread)
	threadID := thread["id"].(string)

	body := `{
		"target":{"artifact":"blueprint","level":"deck"},
		"interaction":{"intent":"consult","clarification":"never"},
		"instruction":"评估当前叙事结构"
	}`
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("new protocol run: %d %s", resp.Code, resp.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &created)
	target := created["target"].(map[string]any)
	interaction := created["interaction"].(map[string]any)
	if target["artifact"] != "blueprint" || target["level"] != "deck" || interaction["intent"] != "consult" {
		t.Fatalf("new protocol was not preserved: %s", resp.Body.String())
	}

	invalid := `{
		"target":{"artifact":"presentation","level":"slide","slide_id":"current"},
		"interaction":{"intent":"apply","clarification":"when_blocked"},
		"instruction":"修改当前页"
	}`
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", invalid)
	if resp.Code != http.StatusUnprocessableEntity || !strings.Contains(resp.Body.String(), "INVALID_TARGET") {
		t.Fatalf("unstable current target must be rejected: %d %s", resp.Code, resp.Body.String())
	}
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
	if projectID == "" || workDir != filepath.Join(root, "projects", projectID) {
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

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", `{"target":{"artifact":"blueprint","level":"deck"},"interaction":{"intent":"apply","clarification":"when_blocked"},"instruction":"生成蓝图"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /threads/{id}/runs should work with API-created thread, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestRenameProjectAndThread(t *testing.T) {
	srv, _ := setupProjectThreadServer(t)

	// Create Project
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Test Project","language":"zh"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /projects want 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	json.Unmarshal(resp.Body.Bytes(), &project)
	projectID, _ := project["id"].(string)

	// Rename Project Success
	resp = apiReq(t, http.MethodPatch, srv.URL+"/api/v1/projects/"+projectID, `{"title":"New Project Title"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH /projects/%s want 200, got %d: %s", projectID, resp.Code, resp.Body.String())
	}
	var updatedProj map[string]any
	json.Unmarshal(resp.Body.Bytes(), &updatedProj)
	if updatedProj["title"] != "New Project Title" {
		t.Fatalf("want title New Project Title, got %v", updatedProj["title"])
	}

	// Rename Project Empty Title
	resp = apiReq(t, http.MethodPatch, srv.URL+"/api/v1/projects/"+projectID, `{"title":"   "}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("PATCH empty title want 400, got %d", resp.Code)
	}

	// Rename Project Too Long Title
	longTitle := strings.Repeat("a", 61)
	resp = apiReq(t, http.MethodPatch, srv.URL+"/api/v1/projects/"+projectID, `{"title":"`+longTitle+`"}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("PATCH long title want 400, got %d", resp.Code)
	}

	// Rename Project Not Found
	resp = apiReq(t, http.MethodPatch, srv.URL+"/api/v1/projects/not-found", `{"title":"Title"}`)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("PATCH not found want 404, got %d", resp.Code)
	}

	// Create Thread
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{"title":"Old Thread"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /threads want 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var thread map[string]any
	json.Unmarshal(resp.Body.Bytes(), &thread)
	threadID, _ := thread["id"].(string)

	// Rename Thread Success
	resp = apiReq(t, http.MethodPatch, srv.URL+"/api/v1/threads/"+threadID, `{"title":"New Thread Title"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH /threads/%s want 200, got %d: %s", threadID, resp.Code, resp.Body.String())
	}
	var updatedThread map[string]any
	json.Unmarshal(resp.Body.Bytes(), &updatedThread)
	if updatedThread["title"] != "New Thread Title" {
		t.Fatalf("want title New Thread Title, got %v", updatedThread["title"])
	}

	// Rename Thread Empty Title
	resp = apiReq(t, http.MethodPatch, srv.URL+"/api/v1/threads/"+threadID, `{"title":""}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("PATCH empty title want 400, got %d", resp.Code)
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
	rec.HeaderMap = resp.Header.Clone()
	_, _ = rec.Body.ReadFrom(resp.Body)
	return rec
}
