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

func TestPolishEndpointReturnsTextWithoutStartingRun(t *testing.T) {
	provider := &llmtest.FakeProvider{ProviderName: "fake", ModelName: "polish-model", Script: []llm.GenerateResponse{{
		Content: llm.TextContent("请在当前项目中强化核心信息层级，并保持整体视觉克制。"),
	}}}
	registry, err := llm.NewRegistryWithProfiles("Polish", []llm.Profile{llm.NewTestProfile("Polish", "https://example.invalid", provider)})
	if err != nil {
		t.Fatal(err)
	}
	server, _ := setupProjectThreadServerWithFactoryAndRegistry(t, func(model.Run, model.CreateRunParams, model.Project) run.Execution {
		return noOpRunner{}
	}, registry)
	response := apiReq(t, http.MethodPost, server.URL+"/api/v1/projects", `{"topic":"Polish context","language":"zh-CN"}`)
	var project struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &project)
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/threads", `{"title":"Polish"}`)
	var thread struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &thread)
	body := `{"instruction":"更有冲击力","thread_id":"` + thread.ID + `","scope":{"object":"presentation","selection":{"kind":"all_pages"}},"mode":"execute","model":"Polish"}`
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/polish", body)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"changed":true`) || !strings.Contains(response.Body.String(), `"prompt_version":"2026-08-23.v1"`) {
		t.Fatalf("polish response: %d %s", response.Code, response.Body.String())
	}
	if len(provider.Requests()) != 1 || len(provider.Requests()[0].Tools) != 0 {
		t.Fatalf("polish did not stay a single tool-free provider call: %+v", provider.Requests())
	}
}
