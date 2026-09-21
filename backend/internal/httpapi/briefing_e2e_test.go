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
		ProviderName: "fake", ModelName: "briefing-model",
		Script: []llm.GenerateResponse{
			{Content: llm.TextContent("# Kickoff\nBuild the feature.")},
			{Content: llm.TextContent("# Handoff\nContinue the feature.")},
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
		`{"op":"outline.init","structure":[{"client_ref":"opening","title":"开场","purpose":"建立主题","slides":[{"client_ref":"cover","title":"封面","role":"cover"}],"subsections":[]}]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("initialize project: %d %s", response.Code, response.Body.String())
	}
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/threads", `{"title":"Briefing"}`)
	var thread struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &thread)

	for _, kind := range []string{"kickoff", "handoff"} {
		body := `{"thread_id":"` + thread.ID + `"}`
		response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/"+kind, body)
		if response.Code != http.StatusOK ||
			!strings.Contains(response.Body.String(), `"kind":"`+kind+`"`) ||
			!strings.Contains(response.Body.String(), `"version_no":1`) {
			t.Fatalf("%s response: %d %s", kind, response.Code, response.Body.String())
		}
	}
	response = apiReq(t, http.MethodGet, server.URL+"/api/v1/threads/"+thread.ID+"/history", "")
	if response.Code != http.StatusOK || strings.Count(response.Body.String(), `"type":"briefing"`) != 2 {
		t.Fatalf("briefing history: %d %s", response.Code, response.Body.String())
	}
	if len(provider.Requests()) != 2 {
		t.Fatalf("expected two single model calls, got %d", len(provider.Requests()))
	}
}

// A slow provider makes it observable whether the project-history middleware
// forwards progress before completion, and whether disconnect cancels the work.
type blockingBriefingProvider struct {
	*llmtest.FakeProvider
	stopped chan struct{}
}

func (p *blockingBriefingProvider) Generate(ctx context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
	<-ctx.Done()
	close(p.stopped)
	return llm.GenerateResponse{}, ctx.Err()
}

func TestBriefingProgressStreamsThroughHistoryGateAndDisconnectCancels(t *testing.T) {
	provider := &blockingBriefingProvider{FakeProvider: &llmtest.FakeProvider{}, stopped: make(chan struct{})}
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
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/mutations", `{"op":"outline.init","structure":[{"client_ref":"opening","title":"开场","purpose":"建立主题","slides":[{"client_ref":"cover","title":"封面","role":"cover"}],"subsections":[]}]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("initialize: %s", response.Body.String())
	}
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/threads", `{"title":"Stream"}`)
	var thread struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &thread)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/kickoff", strings.NewReader(`{"thread_id":"`+thread.ID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	stream, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	decoder := json.NewDecoder(stream.Body)
	for phase := 0; phase < 3; phase++ {
		var event struct {
			Type  string `json:"type"`
			Phase int    `json:"phase"`
		}
		if err := decoder.Decode(&event); err != nil {
			t.Fatalf("phase %d was buffered until provider completion: %v", phase, err)
		}
		if event.Type != "phase" || event.Phase != phase {
			t.Fatalf("wrong phase: %+v", event)
		}
	}
	_ = stream.Body.Close()
	select {
	case <-provider.stopped:
	case <-time.After(time.Second):
		t.Fatal("disconnect did not cancel provider")
	}
}
