package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"go.uber.org/zap"
)

// Deliberately returns a successful response after cancellation, like a provider
// that cannot interrupt its in-flight network request.
type lateCommandProvider struct {
	*llmtest.FakeProvider
	cancel     context.CancelFunc
	afterCalls int
}

func (p *lateCommandProvider) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	result, err := p.FakeProvider.Generate(ctx, req)
	if p.afterCalls == 0 || len(p.Requests()) >= p.afterCalls {
		p.cancel()
	}
	return result, err
}

func TestBriefingCancellationDiscardsLateProviderResult(t *testing.T) {
	for _, afterCalls := range []int{1, 2} {
		t.Run(map[int]string{1: "initial", 2: "correction"}[afterCalls], func(t *testing.T) {
			fixture := newBriefingFixture(t, "late content")
			if afterCalls == 2 {
				fixture.provider.Script = append([]llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{
					ID: "invalid-brief", Name: "handoff_thread", Args: map[string]any{"title": "缺少内容"},
				}}}}, fixture.provider.Script...)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			provider := &lateCommandProvider{FakeProvider: fixture.provider, cancel: cancel, afterCalls: afterCalls}
			registry, err := llm.NewRegistryWithProfiles("Briefing", []llm.Profile{
				llm.NewTestProfile("Briefing", "https://example.invalid", provider),
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = NewHandoffService(fixture.store, registry, fixture.locks).Generate(ctx, "p1", BriefingParams{ThreadID: "t1"})
			if !errors.Is(err, context.Canceled) || len(fixture.provider.Requests()) != afterCalls {
				t.Fatalf("expected cancellation after %d calls, got %v", afterCalls, err)
			}
			var count int64
			if err := fixture.db.Table("briefing_versions").Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("late response persisted: %d %v", count, err)
			}
		})
	}
}

func TestBriefingReportsPhasesAndAllowsRetryWithoutFeedback(t *testing.T) {
	fixture := newBriefingFixture(t, "first", "latest")
	svc := NewHandoffService(fixture.store, fixture.registry, fixture.locks)
	var phases []int
	ctx := WithCommandProgress(context.Background(), func(phase int) error { phases = append(phases, phase); return nil })
	first, err := svc.Generate(ctx, "p1", BriefingParams{ThreadID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(phases, []int{0, 1, 2}) {
		t.Fatalf("incomplete phases: %v", phases)
	}
	commitBriefingResult(t, fixture, first)
	next, err := svc.Generate(context.Background(), "p1", BriefingParams{ThreadID: "t1", BriefingID: first.Briefing.BriefingID})
	if err != nil {
		t.Fatal(err)
	}
	if next.Briefing.BriefingID != first.Briefing.BriefingID || len(next.Briefing.Versions) != 2 || next.Briefing.Versions[1].Content != "latest" {
		t.Fatalf("unexpected retry: %+v", next)
	}
}

func TestExplicitRenameRunsBeforeFirstInputAndDiscardsCanceledResult(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "immediate", true: "canceled"}[canceled], func(t *testing.T) {
			fixture := newBriefingFixture(t)
			fixture.provider.Script = []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{ID: "rename-1", Name: renameToolName, Args: map[string]any{"action": "rename", "title": "季度汇报"}}}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var provider llm.Provider = fixture.provider
			if canceled {
				provider = &lateCommandProvider{FakeProvider: fixture.provider, cancel: cancel}
			}
			svc := NewNamingService(fixture.store, provider, NewThreadEventHub(fixture.store), zap.NewNop())
			defer svc.Close()
			var phases []int
			candidate, err := svc.GenerateNow(WithCommandProgress(ctx, func(phase int) error { phases = append(phases, phase); return nil }), "t1")
			if err == nil {
				commitRenameResult(t, fixture, candidate)
			}
			thread, readErr := fixture.store.GetThread(context.Background(), "t1")
			if readErr != nil {
				t.Fatal(readErr)
			}
			if thread.RenameFirstInputSeen {
				t.Fatal("fixture unexpectedly has an accepted input")
			}
			if canceled {
				if !errors.Is(err, context.Canceled) || thread.Title == "季度汇报" {
					t.Fatalf("late title applied: %+v, %v", thread, err)
				}
			} else if err != nil || thread.Title != "季度汇报" || !reflect.DeepEqual(phases, []int{0, 1, 2}) {
				t.Fatalf("rename waited for input or skipped phases: %+v, %v, %v", thread, phases, err)
			}
		})
	}
}

type waitingCommandProvider struct {
	*llmtest.FakeProvider
	started chan struct{}
}

func (p *waitingCommandProvider) Generate(ctx context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
	close(p.started)
	<-ctx.Done()
	return llm.GenerateResponse{}, ctx.Err()
}

func TestGitStopBeforeWritingVersionAndReleasesProject(t *testing.T) {
	for _, mode := range []string{"cancel", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newBriefingFixture(t)
			fixture.provider.Caps = llm.Capabilities{ToolCalls: true}
			provider := &waitingCommandProvider{FakeProvider: fixture.provider, started: make(chan struct{})}
			registry, err := llm.NewRegistryWithProfiles("Commit", []llm.Profile{llm.NewTestProfile("Commit", "https://example.invalid", provider)})
			if err != nil {
				t.Fatal(err)
			}
			svc := NewGitCommitService(fixture.store, registry, fixture.locks)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			commands := NewCommandService(fixture.store, svc.ExecuteCommand)
			operation, err := commands.Accept(ctx, "t1", model.CommandRequest{RequestKey: "cancel-command-test", Kind: "commit", Input: json.RawMessage(`{}`)}, 1)
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-provider.started:
			case <-ctx.Done():
				t.Fatal("commit did not reach generation")
			}
			if mode == "shutdown" {
				if err := commands.Shutdown(ctx); err != nil {
					t.Fatal(err)
				}
				_, err := commands.Accept(ctx, "t1", model.CommandRequest{RequestKey: "after-shutdown", Kind: "commit", Input: json.RawMessage(`{}`)}, 1)
				if !errors.Is(err, run.ErrEngineStopping) {
					t.Fatalf("accepted new work after shutdown: %v", err)
				}
			} else {
				current, err := commands.Get(ctx, operation.Command.CommandID)
				if err != nil {
					t.Fatal(err)
				}
				cancelCtx := sessionstate.WithEntityRevision(sessionstate.WithRequestScene(ctx, 1), current.EntityRevision)
				if _, err := commands.Cancel(cancelCtx, operation.Command.CommandID, operation.Command.AttemptID, "cancel"); err != nil {
					t.Fatal(err)
				}
			}
			stopped := waitCommand(t, fixture.store, operation.Command.CommandID)
			if stopped.Status != "canceled" {
				t.Fatalf("unexpected result: %+v", stopped)
			}
			if len(stopped.Operation) != 0 {
				t.Fatal("canceled commit entered side effect")
			}
			release, acquired := fixture.locks.TryAcquire("p1")
			if !acquired {
				t.Fatal("terminal status preceded lock release")
			}
			release()
		})
	}
}
