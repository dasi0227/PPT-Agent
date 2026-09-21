package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"go.uber.org/zap"
)

// Deliberately returns a successful response after cancellation, like a provider
// that cannot interrupt its in-flight network request.
type lateCommandProvider struct {
	*llmtest.FakeProvider
	cancel context.CancelFunc
}

func (p *lateCommandProvider) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	result, err := p.FakeProvider.Generate(ctx, req)
	p.cancel()
	return result, err
}

func TestBriefingCancellationDiscardsLateProviderResult(t *testing.T) {
	fixture := newBriefingFixture(t, "late content")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := &lateCommandProvider{FakeProvider: fixture.provider, cancel: cancel}
	registry, err := llm.NewRegistryWithProfiles("Briefing", []llm.Profile{
		llm.NewTestProfile("Briefing", "https://example.invalid", provider),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewKickoffService(fixture.store, registry, fixture.locks).Generate(ctx, "p1", BriefingParams{ThreadID: "t1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	briefings, err := fixture.store.ListThreadBriefings(context.Background(), "t1")
	if err != nil || len(briefings) != 0 {
		t.Fatalf("late response persisted: %+v, %v", briefings, err)
	}
}

func TestBriefingReportsPhasesAndAllowsRetryWithoutFeedback(t *testing.T) {
	fixture := newBriefingFixture(t, "first", "latest")
	svc := NewKickoffService(fixture.store, fixture.registry, fixture.locks)
	var phases []int
	ctx := WithCommandProgress(context.Background(), func(phase int) error { phases = append(phases, phase); return nil })
	first, err := svc.Generate(ctx, "p1", BriefingParams{ThreadID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(phases, []int{0, 1, 2}) {
		t.Fatalf("incomplete phases: %v", phases)
	}
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
			_, err := svc.GenerateNow(WithCommandProgress(ctx, func(phase int) error { phases = append(phases, phase); return nil }), "t1")
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

func TestGitCancelStopsBeforeWritingVersionAndReleasesProject(t *testing.T) {
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
	operation, err := svc.Start(ctx, "p1", GitCommitParams{ThreadID: "t1", ClientRequestID: "cancel-command-test"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-provider.started:
	case <-ctx.Done():
		t.Fatal("commit did not reach generation")
	}
	stopped, err := svc.Cancel(ctx, operation.ID)
	if err != nil || stopped.Status != model.GitCommitFailed || !strings.Contains(stopped.ErrorJSON, "COMMIT_CANCELED") {
		t.Fatalf("unexpected cancel result: %+v %v", stopped, err)
	}
	events, err := fixture.store.GitCommitEventsSince(ctx, operation.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if strings.Contains(event.Payload, `"phase":"committing"`) || event.Type == model.EventGitCommitCompleted {
			t.Fatalf("canceled operation wrote a version: %+v", event)
		}
	}
	release, acquired := fixture.locks.TryAcquire("p1")
	if !acquired {
		t.Fatal("cancellation acknowledged before releasing project")
	}
	release()
}
