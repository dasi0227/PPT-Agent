package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

func TestPolishEndpointReturnsTitleAndContentWithoutStartingRun(t *testing.T) {
	provider := &llmtest.FakeProvider{ProviderName: "fake", ModelName: "polish-model", Caps: llm.Capabilities{ToolCalls: true}, Script: []llm.GenerateResponse{{
		ToolCalls: []llm.ToolCall{{ID: "polish-result", Name: "polish_instruction", Args: map[string]any{"title": "明确核心信息与视觉层级", "content": "请在当前项目中强化核心信息层级，并保持整体视觉克制。"}}},
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
	body := `{"request_key":"polish","kind":"polish","input":{"instruction":"更有冲击力","thread_id":"` + thread.ID + `","scope":{"selection":{"kind":"all_pages"}},"mode":"execute"}}`
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/threads/"+thread.ID+"/commands", body)
	response = awaitHTTPCommand(t, server.URL, response)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"title":"明确核心信息与视觉层级"`) || !strings.Contains(response.Body.String(), `"content":`) || !strings.Contains(response.Body.String(), `"changed":true`) || !strings.Contains(response.Body.String(), `"prompt_version":"`+prompt.Version+`"`) {
		t.Fatalf("polish response: %d %s", response.Code, response.Body.String())
	}
	if len(provider.Requests()) != 1 || len(provider.Requests()[0].Tools) != 1 || provider.Requests()[0].Tools[0].Name != "polish_instruction" {
		t.Fatalf("polish did not stay a single result-tool provider call: %+v", provider.Requests())
	}
	response = apiReq(t, http.MethodGet, server.URL+"/api/v1/threads/"+thread.ID+"/history", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"type":"command.completed"`) ||
		!strings.Contains(response.Body.String(), `"title":"明确核心信息与视觉层级"`) || !strings.Contains(response.Body.String(), `"instruction":"更有冲击力"`) {
		t.Fatalf("polish result and retry input were not persisted: %s", response.Body.String())
	}
}

func awaitHTTPCommand(t *testing.T, base string, response *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	t.Helper()
	if response.Code != http.StatusAccepted {
		t.Fatalf("command rejected: %d %s", response.Code, response.Body.String())
	}
	var accepted struct {
		ID string `json:"command_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		current := apiReq(t, http.MethodGet, base+"/api/v1/commands/"+accepted.ID, "")
		var value struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(current.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		switch value.Status {
		case "accepted", "running", "cancel_requested":
			time.Sleep(time.Millisecond)
		default:
			return current
		}
	}
	t.Fatal("command timed out")
	return nil
}
