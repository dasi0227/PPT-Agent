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

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
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
	runSvc := service.NewRunServiceWithExecutionFactoryAndRegistry(st, engine, factory, registry)
	projectSvc := service.NewProjectService(st, service.WorkRoot(root))
	themes := service.NewThemeService(service.WorkRoot(root), st)
	components := service.NewComponentService(service.WorkRoot(root), st)
	skills := service.NewSkillService(service.WorkRoot(root), st)
	threadSvc := service.NewThreadService(st)
	naming := service.NewNamingService(st, nil, service.NewThreadEventHub(st), zap.NewNop())
	t.Cleanup(naming.Close)
	var polishHandler *httpapi.PolishHandler
	if registry != nil {
		polishHandler = httpapi.NewPolishHandler(service.NewPolishService(st, registry))
	}
	var briefingHandler *httpapi.BriefingHandler
	if registry != nil {
		briefingHandler = httpapi.NewBriefingHandler(
			service.NewHandoffService(st, registry, locks),
		)
	}
	router := httpapi.NewRouter(
		cfg,
		zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewProjectHandler(projectSvc, service.NewPPTMutationService(st)),
		httpapi.NewThreadHandler(threadSvc, naming),
		httpapi.NewSlideHandler(service.NewSlideService(st)),
		httpapi.NewRepositoryHandler(themes, components, skills),
		func() *httpapi.LLMHandler {
			if registry == nil {
				return nil
			}
			return httpapi.NewLLMHandler(registry)
		}(),
		polishHandler,
		briefingHandler,
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

func TestCanonicalMutationHTTPReturnsAuthoritativeSnapshot(t *testing.T) {
	srv, _ := setupProjectThreadServer(t)
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Rename directory"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &project)
	projectID := project["id"].(string)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/mutations", `{"op":"outline.init","structure":[{"id":"sec_agent","client_ref":"opening","title":"开场","slides":[],"subsections":[]}]}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("agent supplied id accepted: %d %s", resp.Code, resp.Body.String())
	}

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/mutations", `{"op":"outline.init","structure":[{"client_ref":"opening","title":"开场","slides":[{"client_ref":"cover","title":"封面"}],"subsections":[]}]}`)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"spec_state":"pending"`) {
		t.Fatalf("init outline: %d %s", resp.Code, resp.Body.String())
	}
	var mutation map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &mutation)
	created := mutation["mutation"].(map[string]any)["created"].(map[string]any)
	sectionID, slideID := created["opening"].(string), created["cover"].(string)
	if !strings.HasPrefix(sectionID, "sec_") || !strings.HasPrefix(slideID, "sli_") {
		t.Fatalf("runtime IDs=%v", created)
	}

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/mutations", `{"op":"outline.update","expected_hash":"`+mutation["content"].(map[string]any)["hashes"].(map[string]any)["outline"].(string)+`","node_id":"`+sectionID+`","changes":{"title":"新章节"}}`)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"title":"新章节"`) {
		t.Fatalf("update outline: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID+"/content", "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), slideID) {
		t.Fatalf("content snapshot: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/slides", `{}`)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("removed slide endpoint returned %d", resp.Code)
	}
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

func TestSteerAndCancelHTTPAuthority(t *testing.T) {
	started := make(chan struct{}, 1)
	srv, _ := setupProjectThreadServerWithFactory(t, func(model.Run, model.CreateRunParams, model.Project) run.Execution {
		return blockingRunner{started: started}
	})
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Authority"}`)
	var project map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &project)
	projectID := project["id"].(string)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{"title":"Authority"}`)
	var thread map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &thread)
	threadID := thread["id"].(string)
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", `{
		"scene_revision":"1",
		"client_request_id":"req-authority-1",
		"scope":{"selection":{"kind":"all_pages"}},
		"mode":"create",
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

	steerBody := `{"scene_revision":"1","expected_run_id":"` + runID + `","client_message_id":"msg-authority-1","content":"后续页面统一改成深色"}`
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
	if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "IDEMPOTENCY_CONFLICT") {
		t.Fatalf("steering hash conflict: %d %s", resp.Code, resp.Body.String())
	}

	view := readHTTPSessionView(t, srv.URL, threadID)
	if view.ActiveRun == nil || view.ActiveRun.ID != runID {
		t.Fatal("active run missing from session view")
	}
	cancelBody := fmt.Sprintf(`{"scene_revision":"%d","entity_revision":"%d","request_key":"stop-authority"}`, view.SceneRevision, view.ActiveRun.EntityRevision)
	resp = apiReq(t, http.MethodDelete, srv.URL+"/api/v1/runs/"+runID, cancelBody)
	var canceled sessionstate.RunMutation
	decodeResponse(t, resp, &canceled)
	if (resp.Code != http.StatusAccepted && resp.Code != http.StatusOK) || canceled.Run.ID != runID || canceled.Run.CancelRequestedAt < 1 || canceled.Receipt.CommitID == "" {
		t.Fatalf("first cancel: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodDelete, srv.URL+"/api/v1/runs/"+runID, cancelBody)
	var replayedCancel sessionstate.RunMutation
	decodeResponse(t, resp, &replayedCancel)
	if (resp.Code != http.StatusAccepted && resp.Code != http.StatusOK) || replayedCancel.Receipt.CommitID != canceled.Receipt.CommitID {
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
		`{"scene_revision":"1","expected_run_id":"`+runID+`","client_message_id":"msg-late","content":"too late"}`)
	if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "RUN_NOT_STEERABLE") {
		t.Fatalf("terminal steering was accepted: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/threads/"+threadID+"/timeline?scene_revision=1", "")
	if resp.Code != http.StatusOK {
		t.Fatalf("timeline: %d %s", resp.Code, resp.Body.String())
	}
	var timeline sessionstate.TimelinePage
	decodeResponse(t, resp, &timeline)
	steeringCount := 0
	for _, item := range timeline.Items {
		var summary struct {
			Text     string `json:"text"`
			Steering bool   `json:"steering"`
		}
		if err := json.Unmarshal(item.Summary, &summary); err != nil {
			t.Fatal(err)
		}
		if item.Kind == "user_turn" && summary.Steering && summary.Text == "后续页面统一改成深色" {
			steeringCount++
		}
	}
	if steeringCount != 1 {
		t.Fatalf("steering was not recoverable once from timeline: %s", resp.Body.String())
	}
}

func TestArtifactTargetRunAndContentAPI(t *testing.T) {
	srv, _ := setupProjectThreadServer(t)
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Artifact R0"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &project)
	projectID := project["id"].(string)

	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID+"/content", "")
	if resp.Code != http.StatusOK {
		t.Fatalf("get spec: %d %s", resp.Code, resp.Body.String())
	}
	var view map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &view)
	if view["project_id"] != projectID {
		t.Fatalf("unexpected snapshot project identity: %s", resp.Body.String())
	}
	for _, kind := range []string{"meta", "outline", "constraints"} {
		content := view[kind].(map[string]any)
		for _, field := range []string{"project_id", "version", "created_at", "updated_at"} {
			if _, exists := content[field]; exists {
				t.Fatalf("%s still contains removed metadata %s", kind, field)
			}
		}
	}

	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects/"+projectID+"/threads", `{"title":"R0"}`)
	var thread map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &thread)
	threadID := thread["id"].(string)

	body := `{
		"scene_revision":"1",
		"client_request_id":"req-artifact-1",
		"scope":{"selection":{"kind":"all_pages"}},
		"mode":"chat",
		"instruction":"评估当前叙事结构"
	}`
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("new protocol run: %d %s", resp.Code, resp.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &created)
	scope := created["scope"].(map[string]any)
	if scope["object"] != nil || scope["revision"] != "1" || created["mode"] != "chat" {
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
	if queried["id"] != runID || queried["thread_id"] != threadID || queried["project_id"] != projectID {
		t.Fatalf("unexpected run response: %s", resp.Body.String())
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/runs/missing", "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("GET missing run want 404, got %d: %s", resp.Code, resp.Body.String())
	}

	legacy := `{
		"scene_revision":"1",
		"client_request_id":"req-artifact-legacy",
		"target":{"artifact":"presentation","level":"deck"},
		"interaction":{"mode":"chat"},
		"instruction":"legacy"
	}`
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", legacy)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("legacy command shape must be rejected: %d %s", resp.Code, resp.Body.String())
	}

	invalid := `{
		"scene_revision":"1",
		"client_request_id":"req-artifact-invalid",
		"scope":{"selection":{"kind":"current_page","current_slide_id":"current"}},
		"mode":"create",
		"instruction":"修改当前页"
	}`
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/runs", invalid)
	if resp.Code != http.StatusUnprocessableEntity || !strings.Contains(resp.Body.String(), "INVALID_SCOPE") {
		t.Fatalf("unstable current target must be rejected: %d %s", resp.Code, resp.Body.String())
	}
}

func TestRunScreenshotEndpointUsesOpaqueRunScopedReference(t *testing.T) {
	srv, root := setupProjectThreadServer(t)
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Screenshots"}`)
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
		"scene_revision":"1",
		"client_request_id":"req-screenshot-1",
		"scope":{"selection":{"kind":"all_pages"}},
		"mode":"chat",
		"instruction":"查看当前演示"
	}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create run: %d %s", resp.Code, resp.Body.String())
	}
	var runModel map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &runModel)
	runID := runModel["id"].(string)
	screenshotID := "shot_123e4567-e89b-12d3-a456-426614174000"
	path := filepath.Join(root, "projects", projectID, "artifacts", ".runtime", "renders", runID, screenshotID+".png")
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

func TestDeleteProjectAcceptsBeforeBackgroundContainerCleanup(t *testing.T) {
	srv, root := setupProjectThreadServer(t)
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Delete me"}`)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", resp.Code, resp.Body.String())
	}
	var project map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	projectID := project["id"].(string)
	projectRoot := filepath.Join(root, "projects", projectID)
	resp = apiReq(t, http.MethodDelete, srv.URL+"/api/v1/projects/"+projectID, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("delete project: %d %s", resp.Code, resp.Body.String())
	}
	if resp := apiReq(t, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID, ""); resp.Code != http.StatusNotFound {
		t.Fatalf("deleted project remains accessible: %d", resp.Code)
	}
	deadline := time.Now().Add(7 * time.Second)
	for {
		if _, err := os.Stat(projectRoot); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background deletion did not clean project container")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRenameProjectAndThread(t *testing.T) {
	srv, _ := setupProjectThreadServer(t)

	// Create Project
	resp := apiReq(t, http.MethodPost, srv.URL+"/api/v1/projects", `{"topic":"Test Project"}`)
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
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/commands", `{"scene_revision":"1","entity_revision":"0","request_key":"rename-thread","kind":"rename","input":{"mode":"manual","title":"New Thread Title"}}`)
	resp = awaitHTTPCommand(t, srv.URL, resp)
	var renamed model.CommandExecution
	decodeResponse(t, resp, &renamed)
	if resp.Code != http.StatusOK || renamed.Kind != "rename" || renamed.Status != "completed" {
		t.Fatalf("rename command: %d %s", resp.Code, resp.Body.String())
	}
	resp = apiReq(t, http.MethodGet, srv.URL+"/api/v1/threads/"+threadID, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /threads/%s want 200, got %d: %s", threadID, resp.Code, resp.Body.String())
	}
	var updatedThread map[string]any
	json.Unmarshal(resp.Body.Bytes(), &updatedThread)
	if updatedThread["title"] != "New Thread Title" {
		t.Fatalf("want title New Thread Title, got %v", updatedThread["title"])
	}

	// Rename Thread Empty Title
	resp = apiReq(t, http.MethodPost, srv.URL+"/api/v1/threads/"+threadID+"/commands", `{"scene_revision":"1","entity_revision":"0","request_key":"rename-thread-empty","kind":"rename","input":{"mode":"manual","title":""}}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("rename empty title want 400, got %d", resp.Code)
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
