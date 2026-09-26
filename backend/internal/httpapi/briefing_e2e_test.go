package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

func TestKickoffAndHandoffEndpointsReturnPersistentBriefings(t *testing.T) {
	provider := &llmtest.FakeProvider{
		ProviderName: "fake", ModelName: "briefing-model", Caps: llm.Capabilities{ToolCalls: true},
		Script: []llm.GenerateResponse{
			{ToolCalls: []llm.ToolCall{{ID: "kickoff-result", Name: "kickoff_thread", Args: map[string]any{"title": "启动功能开发", "content": "# Kickoff\nBuild the feature."}}}},
			{ToolCalls: []llm.ToolCall{{ID: "handoff-result", Name: "handoff_thread", Args: map[string]any{"title": "交接功能开发", "content": "# Handoff\nContinue the feature."}}}},
		},
	}
	registry, err := llm.NewRegistryWithProfiles("Briefing", []llm.Profile{
		llm.NewTestProfile("Briefing", "https://example.invalid", provider),
	})
	if err != nil {
		t.Fatal(err)
	}
	server, _ := setupProjectThreadServerWithFactoryAndRegistry(t, func(model.Run, model.CreateRunParams, model.Project) run.Execution {
		return noOpRunner{}
	}, registry)
	response := apiReq(t, http.MethodPost, server.URL+"/api/v1/projects", `{"topic":"Briefing context","language":"zh-CN"}`)
	var project struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &project)
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/mutations",
		`{"op":"outline.init","structure":[{"client_ref":"opening","title":"开场","purpose":"建立主题","slides":[{"client_ref":"cover","title":"封面"}],"subsections":[]}]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("initialize project: %d %s", response.Code, response.Body.String())
	}
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/threads", `{"title":"Briefing"}`)
	var thread struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &thread)

	for _, kind := range []string{"kickoff", "handoff"} {
		body := `{"request_key":"` + kind + `","kind":"` + kind + `","input":{}}`
		response = apiReq(t, http.MethodPost, server.URL+"/api/v1/threads/"+thread.ID+"/commands", body)
		response = awaitHTTPCommand(t, server.URL, response)
		if response.Code != http.StatusOK ||
			!strings.Contains(response.Body.String(), `"kind":"`+kind+`"`) ||
			!strings.Contains(response.Body.String(), `"version_no":1`) ||
			!strings.Contains(response.Body.String(), `"title":`) {
			t.Fatalf("%s response: %d %s", kind, response.Code, response.Body.String())
		}
	}
	response = apiReq(t, http.MethodGet, server.URL+"/api/v1/threads/"+thread.ID+"/history", "")
	if response.Code != http.StatusOK || strings.Count(response.Body.String(), `"type":"command.completed"`) != 2 ||
		!strings.Contains(response.Body.String(), `"title":"启动功能开发"`) ||
		!strings.Contains(response.Body.String(), `"title":"交接功能开发"`) {
		t.Fatalf("briefing history: %d %s", response.Code, response.Body.String())
	}
	if len(provider.Requests()) != 2 {
		t.Fatalf("expected two single model calls, got %d", len(provider.Requests()))
	}
}

// A slow provider makes it observable whether the project-history middleware
// accepts execution independently of the request connection.
type blockingBriefingProvider struct {
	*llmtest.FakeProvider
	started chan struct{}
	stopped chan struct{}
}

func (p *blockingBriefingProvider) Generate(ctx context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
	close(p.started)
	<-ctx.Done()
	close(p.stopped)
	return llm.GenerateResponse{}, ctx.Err()
}

func TestBriefingSurvivesDisconnectAndStopsOnlyOnExplicitCancel(t *testing.T) {
	provider := &blockingBriefingProvider{FakeProvider: &llmtest.FakeProvider{Caps: llm.Capabilities{ToolCalls: true}}, started: make(chan struct{}), stopped: make(chan struct{})}
	registry, err := llm.NewRegistryWithProfiles("Briefing", []llm.Profile{llm.NewTestProfile("Briefing", "https://example.invalid", provider)})
	if err != nil {
		t.Fatal(err)
	}
	server, _ := setupProjectThreadServerWithFactoryAndRegistry(t, func(model.Run, model.CreateRunParams, model.Project) run.Execution { return noOpRunner{} }, registry)
	response := apiReq(t, http.MethodPost, server.URL+"/api/v1/projects", `{"topic":"Stream progress","language":"zh-CN"}`)
	var project struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &project)
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/mutations", `{"op":"outline.init","structure":[{"client_ref":"opening","title":"开场","purpose":"建立主题","slides":[{"client_ref":"cover","title":"封面"}],"subsections":[]}]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("initialize: %s", response.Body.String())
	}
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/threads", `{"title":"Stream"}`)
	var thread struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &thread)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/v1/threads/"+thread.ID+"/commands", strings.NewReader(`{"request_key":"disconnect","kind":"kickoff","input":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	accepted, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var command model.CommandExecution
	if err := json.NewDecoder(accepted.Body).Decode(&command); err != nil {
		t.Fatal(err)
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	accepted.Body.Close()
	cancel()
	select {
	case <-provider.stopped:
		t.Fatal("request disconnect canceled execution")
	case <-time.After(30 * time.Millisecond):
	}
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/commands/"+command.CommandID+"/cancel", `{"request_key":"stop","attempt_id":"`+command.AttemptID+`"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("cancel: %s", response.Body.String())
	}
	select {
	case <-provider.stopped:
	case <-time.After(time.Second):
		t.Fatal("explicit stop did not cancel provider")
	}
}
