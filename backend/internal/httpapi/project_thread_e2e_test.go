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
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type noOpRunner struct{}

func (noOpRunner) Run(context.Context, workflow.EventEmitter, run.Checkpointer, run.Prompter) workflow.StructuredOutcome {
	return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
}

func setupProjectThreadServer(t *testing.T) (*httptest.Server, string) {
	return setupProjectThreadServerWithFactory(t, func(model.Run, model.CreateRunParams, model.Project) run.Execution {
		return noOpRunner{}
	})
}

func setupProjectThreadServerWithFactory(t *testing.T, factory service.ExecutionFactory) (*httptest.Server, string) {
	return setupProjectThreadServerWithFactoryAndRegistry(t, factory, nil)
}

func setupProjectThreadServerWithFactoryAndRegistry(
	t *testing.T,
	factory service.ExecutionFactory,
	registry *llm.Registry,
) (*httptest.Server, string) {
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
	runSvc := service.NewRunServiceWithExecutionFactoryAndRegistry(st, engine, factory, registry)
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
		func() *httpapi.LLMHandler {
			if registry == nil {
				return nil
			}
			return httpapi.NewLLMHandler(registry)
		}(),
		httpapi.NewSpecHandler(service.NewSpecService(st)),
	)
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, root
}

type blockingRunner struct {
	started chan<- struct{}
}

func (r blockingRunner) Run(ctx context.Context, _ workflow.EventEmitter, _ run.Checkpointer, _ run.Prompter) workflow.StructuredOutcome {
	select {
	case r.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return workflow.StructuredOutcome{Status: workflow.StatusCanceled, Code: workflow.CodeCanceled}
}

func TestSteerAndCancelHTTPAuthority(t *testing.T) {
	started := make(chan struct{}, 1)
	srv, _ := setupProjectThreadServerWithFactory(t, func(model.Run, model.CreateRunParams, model.Project) run.Execution {
		return blockingRunner{started: started}
	})
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Authority","language":"zh-CN"}`)
	var project map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &project)
	projectID := project["id"].(string)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{"title":"Authority"}`)
	var thread map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &thread)
	threadID := thread["id"].(string)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", `{
		"client_request_id":"req-authority-1",
		"target":{"artifact":"spec","level":"deck"},
		"interaction":{"intent":"execute"},
		"instruction":"生成内容"
	}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create run: %d %s", resp.Code, resp.Body.String())
	}
	var runModel map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &runModel)
	runID := runModel["id"].(string)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not start")
	}

	steerBody := `{"expected_run_id":"` + runID + `","client_message_id":"msg-authority-1","content":"后续页面统一改成深色"}`
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/runs/"+runID+"/steer", steerBody)
	if resp.Code != http.StatusAccepted || !strings.Contains(resp.Body.String(), `"status":"accepted"`) {
		t.Fatalf("steer: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/runs/"+runID+"/steer", steerBody)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("same steering should replay: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/runs/"+runID+"/steer",
		strings.Replace(steerBody, "统一改成深色", "统一改成浅色", 1))
	if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("steering hash conflict: %d %s", resp.Code, resp.Body.String())
	}

	resp = apiReq(t, http.MethodDelete, srv.URL+"/api/v1/runs/"+runID, "")
	if resp.Code != http.StatusAccepted || !strings.Contains(resp.Body.String(), "cancel_requested") {
		t.Fatalf("first cancel: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodDelete, srv.URL+"/api/v1/runs/"+runID, "")
	if resp.Code != http.StatusAccepted && resp.Code != http.StatusOK {
		t.Fatalf("repeated cancel: %d %s", resp.Code, resp.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/runs/"+runID, "")
		if strings.Contains(resp.Body.String(), `"status":"canceled"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(resp.Body.String(), `"status":"canceled"`) {
		t.Fatalf("cancel did not reach authoritative terminal: %s", resp.Body.String())
	}
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/runs/"+runID+"/steer",
		`{"expected_run_id":"`+runID+`","client_message_id":"msg-late","content":"too late"}`)
	if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "RUN_NOT_STEERABLE") {
		t.Fatalf("terminal steering was accepted: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/threads/"+threadID+"/history", "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "后续页面统一改成深色") ||
		!strings.Contains(resp.Body.String(), `"type":"steering"`) {
		t.Fatalf("steering was not recoverable from history: %d %s", resp.Code, resp.Body.String())
	}
}

func TestArtifactTargetRunAndSpecAPI(t *testing.T) {
	srv, _ := setupProjectThreadServer(t)
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Artifact R0","language":"zh-CN"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &project)
	projectID := project["id"].(string)

	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID+"/spec", "")
	if resp.Code != http.StatusOK {
		t.Fatalf("get spec: %d %s", resp.Code, resp.Body.String())
	}
	var view map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &view)
	if view["outline"].(map[string]any)["version"] != "3.0" {
		t.Fatalf("unexpected spec response: %s", resp.Body.String())
	}

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{"title":"R0"}`)
	var thread map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &thread)
	threadID := thread["id"].(string)

	body := `{
		"client_request_id":"req-artifact-1",
		"target":{"artifact":"spec","level":"deck"},
		"interaction":{"intent":"talk"},
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
	if target["artifact"] != "spec" || target["level"] != "deck" || interaction["intent"] != "talk" {
		t.Fatalf("new protocol was not preserved: %s", resp.Body.String())
	}
	runID := created["id"].(string)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("same create request should replay: %d %s", resp.Code, resp.Body.String())
	}
	var replayed map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &replayed)
	if replayed["id"] != runID {
		t.Fatalf("create idempotency produced another run: first=%s replay=%s", runID, resp.Body.String())
	}
	conflictingBody := strings.Replace(body, "评估当前叙事结构", "生成全新内容", 1)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", conflictingBody)
	if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("create request hash conflict was not rejected: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/runs/"+runID, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("GET run: %d %s", resp.Code, resp.Body.String())
	}
	var queried map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &queried)
	if queried["id"] != runID || queried["thread_id"] != threadID || queried["project_id"] != projectID ||
		queried["events_url"] != "/api/v1/runs/"+runID+"/events" {
		t.Fatalf("unexpected run response: %s", resp.Body.String())
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/runs/missing", "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("GET missing run want 404, got %d: %s", resp.Code, resp.Body.String())
	}

	invalid := `{
		"client_request_id":"req-artifact-invalid",
		"target":{"artifact":"presentation","level":"slide","slide_id":"current"},
		"interaction":{"intent":"execute"},
		"instruction":"修改当前页"
	}`
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", invalid)
	if resp.Code != http.StatusUnprocessableEntity || !strings.Contains(resp.Body.String(), "INVALID_TARGET") {
		t.Fatalf("unstable current target must be rejected: %d %s", resp.Code, resp.Body.String())
	}
}

func TestRunScreenshotEndpointUsesOpaqueRunScopedReference(t *testing.T) {
	srv, root := setupProjectThreadServer(t)
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Screenshots","language":"zh-CN"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &project)
	projectID := project["id"].(string)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{"title":"Render"}`)
	var thread map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &thread)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+thread["id"].(string)+"/runs", `{
		"client_request_id":"req-screenshot-1",
		"target":{"artifact":"presentation","level":"deck"},
		"interaction":{"intent":"talk"},
		"instruction":"查看当前演示"
	}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create run: %d %s", resp.Code, resp.Body.String())
	}
	var runModel map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &runModel)
	runID := runModel["id"].(string)
	screenshotID := "shot_123e4567-e89b-12d3-a456-426614174000"
	path := filepath.Join(root, "projects", projectID, ".runtime", "renders", runID, screenshotID+".png")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	png := []byte("\x89PNG\r\n\x1a\n")
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatal(err)
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/runs/"+runID+"/screenshots/"+screenshotID, "")
	if resp.Code != http.StatusOK || resp.Header().Get("Content-Type") != "image/png" ||
		!bytes.Equal(resp.Body.Bytes(), png) {
		t.Fatalf("screenshot: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/runs/"+runID+"/screenshots/outline.json", "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("path-like screenshot id was not rejected: %d", resp.Code)
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

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", `{"client_request_id":"req-created-thread-1","target":{"artifact":"spec","level":"deck"},"interaction":{"intent":"execute"},"instruction":"生成设计稿"}`)
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
