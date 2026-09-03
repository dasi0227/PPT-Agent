package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

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
		body := `{"thread_id":"` + thread.ID + `","model_profile_name":"Briefing"}`
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
