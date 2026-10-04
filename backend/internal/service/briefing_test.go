package service

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

type briefingFixture struct {
	store    *sqlitestore.Store
	project  model.Project
	thread   model.Thread
	provider *llmtest.FakeProvider
	registry *llm.Registry
	locks    *run.LockManager
}

func newBriefingFixture(t *testing.T, responses ...string) briefingFixture {
	t.Helper()
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{WorkRoot: root, DBPath: filepath.Join(root, "briefing.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project := model.Project{
		ID: "p1", Title: "Board narrative", WorkDir: filepath.Join(root, "projects", "p1", "artifacts"),
		Theme: "default", CreatedAt: 1, UpdatedAt: 1,
	}
	thread := model.Thread{
		ID: "t1", ProjectID: project.ID, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := st.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(context.Background(), thread); err != nil {
		t.Fatal(err)
	}
	writePolishFixture(t, project.WorkDir)
	if err := st.ReplaceSlides(context.Background(), project.ID, []model.Slide{{
		ID: "sli_aaaaaa", ProjectID: project.ID,
	}}); err != nil {
		t.Fatal(err)
	}
	script := make([]llm.GenerateResponse, 0, len(responses))
	for _, response := range responses {
		script = append(script, llm.GenerateResponse{ToolCalls: []llm.ToolCall{{ID: "briefing-result", Name: "handoff_thread", Args: map[string]any{"title": "交接当前任务", "content": response}}}})
	}
	provider := &llmtest.FakeProvider{ProviderName: "fake", ModelName: "briefing-model", Caps: llm.Capabilities{ToolCalls: true}, Script: script}
	registry, err := llm.NewRegistryWithProfiles("Briefing", []llm.Profile{
		llm.NewTestProfile("Briefing", "https://example.invalid", provider),
	})
	if err != nil {
		t.Fatal(err)
	}
	return briefingFixture{
		store: st, project: project, thread: thread, provider: provider,
		registry: registry, locks: run.NewLockManager(),
	}
}

func TestHandoffPersistsOnlySuccessfulGeneration(t *testing.T) {
	fixture := newBriefingFixture(t, "# Handoff\nContinue the work.")
	result, err := NewHandoffService(fixture.store, fixture.registry, fixture.locks).Generate(
		context.Background(), fixture.project.ID,
		BriefingParams{ThreadID: fixture.thread.ID},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Briefing.Kind != model.BriefingHandoff ||
		len(result.Briefing.Versions) != 1 ||
		result.Briefing.Versions[0].Title != "交接当前任务" ||
		result.Briefing.Versions[0].Content != "# Handoff\nContinue the work." {
		t.Fatalf("unexpected result: %+v", result)
	}

	fixture.provider.GenerateErr = errors.New("provider failed")
	_, err = NewHandoffService(fixture.store, fixture.registry, fixture.locks).Generate(
		context.Background(), fixture.project.ID,
		BriefingParams{ThreadID: fixture.thread.ID},
	)
	if err == nil {
		t.Fatal("expected provider failure")
	}
	briefings, listErr := fixture.store.ListThreadBriefings(context.Background(), fixture.thread.ID)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(briefings) != 1 {
		t.Fatalf("failed generation was persisted: %+v", briefings)
	}
	fixture.provider.GenerateErr = nil
	invalid := llm.GenerateResponse{Content: llm.TextContent("# Old plain-text result")}
	fixture.provider.Script = []llm.GenerateResponse{invalid, invalid, invalid, {ToolCalls: []llm.ToolCall{{
		ID: "beyond-budget", Name: "handoff_thread", Args: map[string]any{"title": "不应保存", "content": "beyond correction budget"},
	}}}}
	requestsBefore := len(fixture.provider.Requests())
	_, err = NewHandoffService(fixture.store, fixture.registry, fixture.locks).Generate(
		context.Background(), fixture.project.ID,
		BriefingParams{ThreadID: fixture.thread.ID, BriefingID: result.Briefing.BriefingID},
	)
	if err == nil || model.AsAgentError(err, "INTERNAL", "test").Code != "BRIEFING_OUTPUT_INVALID" {
		t.Fatalf("expected invalid result rejection, got %v", err)
	}
	if len(fixture.provider.Requests())-requestsBefore != 3 || len(fixture.provider.Script) != 1 {
		t.Fatal("handoff exceeded the two additional request limit")
	}
	versions, err := fixture.store.GetBriefingVersions(context.Background(), result.Briefing.BriefingID, 0)
	if err != nil || len(versions) != 1 || versions[0].Title != "交接当前任务" {
		t.Fatalf("invalid revision changed saved title/content: %+v, %v", versions, err)
	}
}

func TestBriefingRetryUsesTwoLatestVersionsAndAllFeedback(t *testing.T) {
	fixture := newBriefingFixture(t, "version-one", "version-two", "version-three", "version-four")
	rejected := llm.GenerateResponse{
		Content: llm.TextContent("unsubmitted revision"),
		ToolCalls: []llm.ToolCall{
			{ID: "rejected-one", Name: "handoff_thread", Args: map[string]any{"title": "错误简报", "content": "not persisted"}},
			{ID: "rejected-two", Name: "other_tool", Args: map[string]any{}},
		},
		Continuation: &llm.ProviderContinuation{Provider: fixture.provider.Name(), Model: fixture.provider.Model()},
		Usage:        llm.Usage{OutputTokens: 80},
	}
	fixture.provider.Script = append(fixture.provider.Script[:3], rejected, fixture.provider.Script[3])
	service := NewHandoffService(fixture.store, fixture.registry, fixture.locks)
	result, err := service.Generate(context.Background(), "p1", BriefingParams{ThreadID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, feedback := range []string{"feedback-two", "feedback-three", "feedback-four"} {
		result, err = service.Generate(context.Background(), "p1", BriefingParams{
			ThreadID: "t1", BriefingID: result.Briefing.BriefingID, Feedback: feedback,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	requests := fixture.provider.Requests()
	if len(requests) != 5 {
		t.Fatalf("expected four generations plus one correction, got %d calls", len(requests))
	}
	revisionPrompt := requests[3].Messages[1].Text()
	for _, expected := range []string{"交接当前任务", "version-two", "version-three", "feedback-two", "feedback-three", "feedback-four"} {
		if !strings.Contains(revisionPrompt, expected) {
			t.Fatalf("revision prompt missing %q: %s", expected, revisionPrompt)
		}
	}
	if strings.Contains(revisionPrompt, "version-one") {
		t.Fatalf("revision prompt leaked a version outside the window: %s", revisionPrompt)
	}
	correction := requests[4]
	base := len(requests[3].Messages)
	if len(correction.Messages) != base+len(rejected.ToolCalls)+2 ||
		!reflect.DeepEqual(correction.Messages[:base], requests[3].Messages) ||
		!reflect.DeepEqual(correction.Messages[base].Content, rejected.Content) ||
		!reflect.DeepEqual(correction.Messages[base].ToolCalls, rejected.ToolCalls) ||
		correction.Continuation != rejected.Continuation ||
		correction.MaxOutputTokens != maxBriefingOutputTokens-80 {
		t.Fatal("correction lost original revision, raw response, continuation or shared output budget")
	}
	for i, call := range rejected.ToolCalls {
		message := correction.Messages[base+1+i]
		if message.Role != llm.RoleTool || message.ToolCallID != call.ID || message.Metadata == nil ||
			message.Metadata.Origin != "runtime" || message.Metadata.Kind != "submission_error" {
			t.Fatalf("missing paired runtime failure for %q: %+v", call.ID, message)
		}
	}
	guidance := correction.Messages[len(correction.Messages)-1]
	if guidance.Role != llm.RoleUser || guidance.Metadata == nil || guidance.Metadata.Kind != "guidance" ||
		!strings.Contains(guidance.Text(), "handoff_thread") {
		t.Fatal("correction guidance did not follow all paired tool failures")
	}
	if len(result.Briefing.Versions) != 4 || result.Briefing.Versions[3].VersionNo != 4 {
		t.Fatalf("full version chain was not retained: %+v", result.Briefing.Versions)
	}
	saved, err := fixture.store.GetBriefingVersions(context.Background(), result.Briefing.BriefingID, 0)
	if err != nil || len(saved) != 4 || saved[3].Content != "version-four" || saved[3].Feedback != "feedback-four" {
		t.Fatalf("correction changed or duplicated saved revision: %+v, %v", saved, err)
	}
}

func TestBriefingRejectsBusyAndEmptyProjects(t *testing.T) {
	fixture := newBriefingFixture(t, "unused")
	release, acquired := fixture.locks.TryAcquire(fixture.project.ID)
	if !acquired {
		t.Fatal("failed to reserve project lock")
	}
	_, err := NewHandoffService(fixture.store, fixture.registry, fixture.locks).Generate(
		context.Background(), fixture.project.ID,
		BriefingParams{ThreadID: fixture.thread.ID},
	)
	release()
	var agentErr *model.AgentError
	if !errors.As(err, &agentErr) || agentErr.Code != "BRIEFING_ACTIVE" {
		t.Fatalf("expected busy conflict, got %v", err)
	}

	if err := fixture.store.ReplaceSlides(context.Background(), fixture.project.ID, nil); err != nil {
		t.Fatal(err)
	}
	_, err = NewHandoffService(fixture.store, fixture.registry, fixture.locks).Generate(
		context.Background(), fixture.project.ID,
		BriefingParams{ThreadID: fixture.thread.ID},
	)
	if !errors.As(err, &agentErr) || agentErr.Code != "PROJECT_EMPTY" {
		t.Fatalf("expected empty project rejection, got %v", err)
	}
	if len(fixture.provider.Requests()) != 0 {
		t.Fatal("precondition failure called the provider")
	}
}

func TestBriefingPoliciesKeepProjectContextDynamic(t *testing.T) {
	f := newBriefingFixture(t, "brief")
	params := BriefingParams{ThreadID: f.thread.ID}
	result, err := NewHandoffService(f.store, f.registry, f.locks).Generate(context.Background(), f.project.ID, params)
	if err != nil {
		t.Fatal(err)
	}
	requests := f.provider.Requests()
	if len(requests) != 1 {
		t.Fatalf("valid handoff required %d calls", len(requests))
	}
	req := requests[0]
	if req.Messages[0].Text() != prompts.PublicPolicy("command.handoff") || result.PromptVersion != prompts.Version {
		t.Fatal("wrong catalog policy/version")
	}
	if strings.Contains(req.Messages[0].Text(), f.project.Title) || !strings.Contains(req.Messages[1].Text(), f.project.Title) || len(req.Tools) != 1 || req.Tools[0].Name != "handoff_thread" {
		t.Fatal("briefing context or tools crossed policy boundary")
	}
	if req.RequiredTool != "handoff_thread" || req.ParallelToolCalls == nil || *req.ParallelToolCalls {
		t.Fatal("handoff did not require its single submission tool")
	}
}

func TestBriefingReservesWindowForPolicyFeedbackAndOutput(t *testing.T) {
	f := newBriefingFixture(t, "按已确认方向调整结论页", "继续保留原始数据")
	f.provider.Caps.ContextWindowTokens = 14000
	if err := contextengine.NewJournalTranscriptStore(f.store).Replace(f.project.WorkDir, f.thread.ID, []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("开场目标：改进结论页。" + strings.Repeat("需要保留原始数据。", 5000))},
		{Role: llm.RoleAssistant, Content: llm.TextContent("最新方案：只调整结论层级。")},
		{Role: llm.RoleUser, Content: llm.TextContent("就按这个方案，先给原型。")},
	}); err != nil {
		t.Fatal(err)
	}
	svc := NewHandoffService(f.store, f.registry, f.locks)
	result, err := svc.Generate(context.Background(), f.project.ID, BriefingParams{ThreadID: f.thread.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Generate(context.Background(), f.project.ID, BriefingParams{
		ThreadID: f.thread.ID, BriefingID: result.Briefing.BriefingID, Feedback: strings.Repeat("保留关键约束。", 400),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range f.provider.Requests() {
		if llm.EstimateRequestTokens(req)+req.MaxOutputTokens >= f.provider.Caps.ContextWindowTokens {
			t.Fatal("briefing did not reserve room for policy, revision feedback and output")
		}
		for _, want := range []string{"最新方案", "先给原型"} {
			if !strings.Contains(req.Messages[1].Text(), want) {
				t.Errorf("request lost %q", want)
			}
		}
	}
	f.provider.Caps.ContextWindowTokens = maxBriefingOutputTokens
	_, err = svc.Generate(context.Background(), f.project.ID, BriefingParams{ThreadID: f.thread.ID})
	if err == nil || model.AsAgentError(err, "INTERNAL", "test").Code != "AGENT_FAILED" || len(f.provider.Requests()) != 2 {
		t.Fatalf("insufficient context was sent to the model: %v", err)
	}
}
