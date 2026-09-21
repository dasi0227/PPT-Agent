package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

type routingFake struct {
	name  string
	calls []GenerateRequest
	err   error
}

func (p *routingFake) Name() string  { return p.name }
func (p *routingFake) Model() string { return p.name + "-model" }
func (p *routingFake) Capabilities() Capabilities {
	return Capabilities{ToolCalls: true, Vision: true, ContextWindowTokens: 200000}
}
func (p *routingFake) Generate(_ context.Context, req GenerateRequest) (GenerateResponse, error) {
	p.calls = append(p.calls, req)
	return GenerateResponse{Content: TextContent("ok")}, p.err
}
func routingFixture(t *testing.T, err error) (*Registry, *routingFake, *routingFake) {
	t.Helper()
	main := &routingFake{name: "main", err: err}
	backup := &routingFake{name: "backup"}
	registry, buildErr := NewRegistryWithProfiles("Main", []Profile{NewTestProfile("Main", "main", main), NewTestProfile("Backup", "backup", backup)}, RoadConfig{
		Main: config.MainRoadLLMConfig{Default: "Main", Fallback: "Backup"},
		Side: config.SideRoadLLMConfig{Default: "Backup", Fallback: "Main", Polish: "Main"},
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return registry, main, backup
}
func TestFallbackSticksToOneOperationAndClearsContinuation(t *testing.T) {
	registry, main, backup := routingFixture(t, ErrUnavailable)
	profile, err := registry.RoutedProfile("main", "")
	if err != nil {
		t.Fatal(err)
	}
	provider := profile.Adapter().(*RoutedProvider)
	req := GenerateRequest{Messages: []Message{{Role: RoleUser, Content: TextContent("hello")}}, Continuation: &ProviderContinuation{Provider: "main", Model: "main-model"}}
	for i := 0; i < 2; i++ {
		if _, err := provider.Generate(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	if len(main.calls) != 1 || len(backup.calls) != 2 || backup.calls[0].Continuation != nil || !provider.State().FallbackUsed {
		t.Fatal("fallback reset, reused provider state, or did not become sticky")
	}
	fresh, err := registry.RoutedProfile("main", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fresh.Adapter().Generate(context.Background(), GenerateRequest{}); err != nil {
		t.Fatal(err)
	}
	if len(main.calls) != 2 {
		t.Fatal("new operation did not retry the original selection")
	}
	for _, purpose := range []string{"rename", "compact", "commit", "handoff", "kickoff"} {
		profile, err := registry.RoutedProfile(purpose, "Main")
		if err != nil || profile.Name() != "Backup" {
			t.Fatalf("%s did not use the side default", purpose)
		}
	}
	polish, err := registry.RoutedProfile("polish", "Backup")
	if err != nil || polish.Name() != "Main" {
		t.Fatal("purpose override was ignored")
	}
}
func TestFallbackStopsForCancellationAndPermanentErrors(t *testing.T) {
	for _, failure := range []error{ErrBadRequest, context.Canceled, context.DeadlineExceeded} {
		registry, _, backup := routingFixture(t, failure)
		profile, _ := registry.RoutedProfile("main", "")
		if _, err := profile.Adapter().Generate(context.Background(), GenerateRequest{}); !errors.Is(err, failure) {
			t.Fatal("failure identity was lost")
		}
		if len(backup.calls) != 0 {
			t.Fatal("non-retryable error triggered fallback")
		}
	}
	registry, _, backup := routingFixture(t, ErrUnavailable)
	profile, _ := registry.RoutedProfile("main", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := profile.Adapter().Generate(ctx, GenerateRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled task continued")
	}
	if len(backup.calls) != 0 {
		t.Fatal("canceled task called backup")
	}
}
func TestRuntimeCanPersistSwitchBeforeCallingBackup(t *testing.T) {
	registry, _, backup := routingFixture(t, ErrUnavailable)
	profile, _ := registry.RoutedProfile("main", "")
	provider := profile.Adapter().(*RoutedProvider)
	persisted := false
	provider.OnFallback = func(_ context.Context, state RouteState) error { persisted = state.Active == "Backup"; return nil }
	_, err := provider.Generate(context.Background(), GenerateRequest{PauseOnFallback: true})
	if !errors.Is(err, ErrFallbackActivated) || !persisted || len(backup.calls) != 0 {
		t.Fatal("backup ran before runtime could rebuild its context")
	}
	if _, err = provider.Generate(context.Background(), GenerateRequest{}); err != nil || len(backup.calls) != 1 {
		t.Fatal("activated backup did not handle the next call")
	}
}
