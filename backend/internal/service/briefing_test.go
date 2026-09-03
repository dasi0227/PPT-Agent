package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "briefing.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project := model.Project{
		ID: "p1", Title: "Board narrative", WorkDir: filepath.Join(root, "p1"),
		Theme: "default", Status: "ready", CreatedAt: 1, UpdatedAt: 1,
	}
	thread := model.Thread{
		ID: "t1", ProjectID: project.ID, HistoryPath: "threads/t1.jsonl",
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}
	if err := st.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(context.Background(), thread); err != nil {
		t.Fatal(err)
	}
	writePolishFixture(t, project.WorkDir)
	if err := st.ReplaceSlides(context.Background(), project.ID, []model.Slide{{
		ID: "sli_aaaaaa", ProjectID: project.ID, CurrentVersion: 1,
	}}); err != nil {
		t.Fatal(err)
	}
	script := make([]llm.GenerateResponse, 0, len(responses))
	for _, response := range responses {
		script = append(script, llm.GenerateResponse{Content: llm.TextContent(response)})
	}
	provider := &llmtest.FakeProvider{ProviderName: "fake", ModelName: "briefing-model", Script: script}
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

func TestKickoffPersistsOnlySuccessfulGeneration(t *testing.T) {
	fixture := newBriefingFixture(t, "# Startup\nDo the work.")
	result, err := NewKickoffService(fixture.store, fixture.registry, fixture.locks).Generate(
		context.Background(), fixture.project.ID,
		BriefingParams{ThreadID: fixture.thread.ID, Model: "Briefing"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Briefing.Kind != model.BriefingKickoff ||
		len(result.Briefing.Versions) != 1 ||
		result.Briefing.Versions[0].Content != "# Startup\nDo the work." {
		t.Fatalf("unexpected result: %+v", result)
	}

	fixture.provider.GenerateErr = errors.New("provider failed")
	_, err = NewHandoffService(fixture.store, fixture.registry, fixture.locks).Generate(
		context.Background(), fixture.project.ID,
		BriefingParams{ThreadID: fixture.thread.ID, Model: "Briefing"},
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
}

func TestBriefingRetryUsesTwoLatestVersionsAndAllFeedback(t *testing.T) {
	fixture := newBriefingFixture(t, "version-one", "version-two", "version-three", "version-four")
	service := NewKickoffService(fixture.store, fixture.registry, fixture.locks)
	result, err := service.Generate(context.Background(), "p1", BriefingParams{ThreadID: "t1", Model: "Briefing"})
	if err != nil {
		t.Fatal(err)
	}
	for _, feedback := range []string{"feedback-two", "feedback-three", "feedback-four"} {
		result, err = service.Generate(context.Background(), "p1", BriefingParams{
			ThreadID: "t1", Model: "Briefing",
			BriefingID: result.Briefing.BriefingID, Feedback: feedback,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	requests := fixture.provider.Requests()
	if len(requests) != 4 {
		t.Fatalf("expected four model calls, got %d", len(requests))
	}
	revisionPrompt := requests[3].Messages[1].Text()
	for _, expected := range []string{"version-two", "version-three", "feedback-two", "feedback-three", "feedback-four"} {
		if !strings.Contains(revisionPrompt, expected) {
			t.Fatalf("revision prompt missing %q: %s", expected, revisionPrompt)
		}
	}
	if strings.Contains(revisionPrompt, "version-one") {
		t.Fatalf("revision prompt leaked a version outside the window: %s", revisionPrompt)
	}
	if len(result.Briefing.Versions) != 4 || result.Briefing.Versions[3].VersionNo != 4 {
		t.Fatalf("full version chain was not retained: %+v", result.Briefing.Versions)
	}
}

func TestBriefingRejectsBusyAndEmptyProjects(t *testing.T) {
	fixture := newBriefingFixture(t, "unused")
	release, acquired := fixture.locks.TryAcquire(fixture.project.ID)
	if !acquired {
		t.Fatal("failed to reserve project lock")
	}
	_, err := NewKickoffService(fixture.store, fixture.registry, fixture.locks).Generate(
		context.Background(), fixture.project.ID,
		BriefingParams{ThreadID: fixture.thread.ID, Model: "Briefing"},
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
		BriefingParams{ThreadID: fixture.thread.ID, Model: "Briefing"},
	)
	if !errors.As(err, &agentErr) || agentErr.Code != "PROJECT_EMPTY" {
		t.Fatalf("expected empty project rejection, got %v", err)
	}
	if len(fixture.provider.Requests()) != 0 {
		t.Fatal("precondition failure called the provider")
	}
}
