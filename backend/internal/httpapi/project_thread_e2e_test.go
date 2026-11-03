package httpapi_test

import (
	"bytes"
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

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
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
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(root, "api.db"), WorkRoot: root}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	locks := run.NewLockManager()
	engine := run.NewEngine(st, locks, zap.NewNop())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.PauseAll(ctx, "test_finished"); err != nil {
			t.Error(err)
		}
	})
	runSvc := service.NewRunServiceWithExecutionFactoryAndRegistry(st, engine, factory, nil)
	projectSvc := service.NewProjectService(st, service.WorkRoot(root))
	themes := service.NewThemeService(service.WorkRoot(root), st)
	components := service.NewComponentService(service.WorkRoot(root), st)
	skills := service.NewSkillService(service.WorkRoot(root), st)
	threadSvc := service.NewThreadService(st)
	naming := service.NewNamingService(st, nil, service.NewThreadEventHub(st), zap.NewNop())
	t.Cleanup(naming.Close)
	router := httpapi.NewRouter(
		cfg,
		zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewProjectHandler(projectSvc, service.NewPPTMutationService(st)),
		httpapi.NewThreadHandler(threadSvc, naming),
		httpapi.NewSlideHandler(service.NewSlideService(st)),
		httpapi.NewRepositoryHandler(themes, components, skills),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if _, err := router.WithProjectHistory(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := router.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, root
}

type commandPermissionRunner struct {
	run model.Run
	t   *testing.T
}

func (r commandPermissionRunner) Run(
	ctx context.Context,
	emitter workflow.EventEmitter,
	checkpointer run.Checkpointer,
	prompter run.Prompter,
) workflow.StructuredOutcome {
	commandPrompter, ok := prompter.(interface {
		AskCommandPermission(context.Context, model.CommandPermissionRequestedPayload) (model.CommandPermissionAnswer, error)
	})
	if !ok {
		return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "PROMPTER_MISSING"}
	}
	request := model.CommandPermissionRequestedPayload{
		PublicEventBase: model.NewPublicEventBase(r.run.ID),
		InteractionID:   "cmdperm_http-call",
		CallID:          "http-call",
		Command:         "printf saved > notes.txt",
		CommandHash:     "http-hash",
		Changes:         []model.PublicTarget{{Type: "file", Part: "content", DisplayName: "notes.txt", Diff: &model.ArtifactDiff{Kind: "binary", Status: "added", Filename: "notes.txt"}}},
	}
	checkpoint := workflow.RuntimeCheckpoint{
		RunID: r.run.ID, LoopID: "http-command-permission", SceneRevision: r.run.SceneRevision,
		ContextRevision: r.run.ContextRevision, Scope: r.run.Command.Scope, Mode: r.run.Command.Mode,
		Phase: workflow.PhaseWaitingInput, ResumePhase: workflow.PhaseExecuting, Boundary: "before_command_permission",
		PendingCommand: &workflow.PendingCommandApproval{
			InteractionID: request.InteractionID, CallID: request.CallID, Request: request, ResumePhase: workflow.PhaseExecuting,
		},
	}
	checkpointer.PhaseChanged(workflow.PhaseWaitingInput)
	if err := checkpointer.SaveCheckpoint(ctx, checkpoint); err != nil {
		r.t.Errorf("save pending command permission: %v", err)
		return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "COMMAND_PERMISSION_CHECKPOINT_FAILED"}
	}
	answer, err := commandPrompter.AskCommandPermission(ctx, request)
	if err != nil || answer.Decision != "allow_once" {
		r.t.Errorf("await command permission: answer=%+v err=%v", answer, err)
		return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "COMMAND_PERMISSION_FAILED"}
	}
	encoded, err := json.Marshal(answer)
	if err != nil {
		return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "COMMAND_PERMISSION_FAILED"}
	}
	checkpoint.Phase, checkpoint.Boundary = workflow.PhaseExecuting, "after_command_permission"
	checkpoint.PendingCommand = nil
	checkpoint.InteractionApplications = []model.InteractionApplication{{ID: request.InteractionID, Answer: encoded}}
	checkpointer.PhaseChanged(workflow.PhaseExecuting)
	if err := checkpointer.SaveCheckpoint(ctx, checkpoint); err != nil {
		r.t.Errorf("consume command permission: %v", err)
		return workflow.StructuredOutcome{Status: workflow.StatusFailed, Code: "COMMAND_PERMISSION_CHECKPOINT_FAILED"}
	}
	emitter.Emit(model.EventMessageFinal, model.MessageFinalPayload{
		PublicEventBase: model.NewPublicEventBase(r.run.ID),
		MessageID:       "permission-final", Text: "权限已应用。",
	})
	return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
}

func TestCommandPermissionHTTPAuthority(t *testing.T) {
	srv, _ := setupProjectThreadServerWithFactory(t, func(runModel model.Run, _ model.CreateRunParams, _ model.Project) run.Execution {
		return commandPermissionRunner{run: runModel, t: t}
	})
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Command permission"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &project)
	projectID := project["id"].(string)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{"title":"Command permission"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create thread: %d %s", resp.Code, resp.Body.String())
	}
	var thread map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &thread)
	threadID := thread["id"].(string)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", `{
		"scene_revision":"1",
		"client_request_id":"req-command-permission",
		"scope":{"selection":{"kind":"all_pages"}},
		"mode":"create",
		"instruction":"修改项目文件"
	}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create run: %d %s", resp.Code, resp.Body.String())
	}
	var runModel map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &runModel)
	runID := runModel["id"].(string)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/runs/"+runID, "")
		if strings.Contains(resp.Body.String(), `"status":"waiting"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(resp.Body.String(), `"status":"waiting"`) {
		t.Fatalf("run did not wait for permission: %s", resp.Body.String())
	}

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/runs/"+runID+"/command-permission", `{`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("malformed permission: %d %s", resp.Code, resp.Body.String())
	}
	view := readHTTPSessionView(t, srv.URL, threadID)
	var interaction sessionstate.InteractionView
	for _, pending := range view.PendingInteractions {
		var current sessionstate.InteractionView
		if err := json.Unmarshal(pending, &current); err != nil {
			t.Fatal(err)
		}
		if current.ID == "cmdperm_http-call" {
			interaction = current
		}
	}
	if interaction.RequestRevision < 1 {
		t.Fatal("pending command permission has no request revision")
	}
	permissionBody := fmt.Sprintf(`{"scene_revision":"%d","request_revision":"%d","interaction_id":"%s","call_id":"http-call","command_hash":"http-hash","decision":"allow_once"}`, view.SceneRevision, interaction.RequestRevision, interaction.ID)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/runs/"+runID+"/command-permission",
		strings.Replace(permissionBody, "http-hash", "stale", 1))
	if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "COMMAND_PERMISSION_REJECTED") {
		t.Fatalf("stale permission: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/runs/"+runID+"/command-permission",
		permissionBody)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("valid permission: %d %s", resp.Code, resp.Body.String())
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/runs/"+runID, "")
		if strings.Contains(resp.Body.String(), `"status":"done"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(resp.Body.String(), `"status":"done"`) {
		t.Fatalf("authorized run did not complete: %d %s", resp.Code, resp.Body.String())
	}
}

func TestProjectThreadAPIClosesRunCreationLoop(t *testing.T) {
	srv, root := setupProjectThreadServer(t)

	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"云原生可观测性实践"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /projects want 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	projectID, _ := project["id"].(string)
	workDir, _ := project["work_dir"].(string)
	if projectID == "" || workDir != filepath.Join(root, "projects", projectID, "artifacts") {
		t.Fatalf("bad project response: %+v", project)
	}
	if _, err := os.Stat(filepath.Join(workDir, "state.json")); !os.IsNotExist(err) {
		t.Fatalf("project creation must not create obsolete state.json: %v", err)
	}

	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID+"/content", "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"sections":[]`) || !strings.Contains(resp.Body.String(), `"slides_by_id":{}`) {
		t.Fatalf("new project content should be empty, got %d: %s", resp.Code, resp.Body.String())
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
	view := readHTTPSessionView(t, srv.URL, threadID)
	if view.Timeline.Groups == nil || len(view.Timeline.Groups) != 0 || view.Timeline.Items == nil || len(view.Timeline.Items) != 0 || !view.Timeline.IsEmpty || view.Cursor == "" || view.ThreadID != threadID || view.Thread.ID != threadID || view.ActiveRun != nil || len(view.ActiveCommands) != 0 || len(view.PendingInteractions) != 0 {
		t.Fatalf("new thread session should contain an empty timeline and cursor: %+v", view)
	}

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", `{"scene_revision":"1","client_request_id":"req-created-thread-1","scope":{"selection":{"kind":"all_pages"}},"mode":"create","instruction":"生成设计稿"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /threads/{id}/runs should work with API-created thread, got %d: %s", resp.Code, resp.Body.String())
	}
}

func readHTTPSessionView(t *testing.T, baseURL, threadID string) sessionstate.View {
	t.Helper()
	response := apiReq(t, http.MethodGet, baseURL+"/api/v1/threads/"+threadID+"/view", "")
	if response.Code != http.StatusOK {
		t.Fatalf("session view: %d %s", response.Code, response.Body.String())
	}
	var view sessionstate.View
	decodeResponse(t, response, &view)
	return view
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

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response: %v body=%s", err, response.Body.String())
	}
}
