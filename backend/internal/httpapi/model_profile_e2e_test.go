package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

type profileFakeProvider struct {
	name  string
	model string
	caps  llm.Capabilities
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response: %v body=%s", err, response.Body.String())
	}
}

func (p profileFakeProvider) Name() string                   { return p.name }
func (p profileFakeProvider) Model() string                  { return p.model }
func (p profileFakeProvider) Capabilities() llm.Capabilities { return p.caps }
func (p profileFakeProvider) Generate(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
	return llm.GenerateResponse{}, nil
}

func testModelRegistry(t *testing.T) *llm.Registry {
	t.Helper()
	registry, err := llm.NewRegistryWithProfiles("Vision Profile", []llm.Profile{
		llm.NewTestProfile("Vision Profile", "https://private-vision.invalid/v1", profileFakeProvider{
			name: "kimi", model: "kimi-k3",
			caps: llm.Capabilities{Vision: true, ToolCalls: true, MultipleToolCalls: true},
		}),
		llm.NewTestProfile("Text Profile", "https://private-text.invalid", profileFakeProvider{
			name: "deepseek", model: "deepseek-v4-pro",
			caps: llm.Capabilities{ToolCalls: true, MultipleToolCalls: true},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func setupModelProjectThread(t *testing.T) (*llm.Registry, string, string) {
	t.Helper()
	registry := testModelRegistry(t)
	server, _ := setupProjectThreadServerWithFactoryAndRegistry(
		t,
		func(model.Run, model.CreateRunParams, model.Project) run.Execution { return noOpRunner{} },
		registry,
	)
	response := apiReq(t, http.MethodPost, server.URL+"/api/v1/projects", `{"topic":"Models","language":"zh-CN"}`)
	var project struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &project)
	response = apiReq(t, http.MethodPost, server.URL+"/api/v1/projects/"+project.ID+"/threads", `{"title":"Models"}`)
	var thread struct {
		ID string `json:"id"`
	}
	decodeResponse(t, response, &thread)
	return registry, server.URL, thread.ID
}

func TestLLMProfilesEndpointIsSafe(t *testing.T) {
	_, baseURL, _ := setupModelProjectThread(t)
	response := apiReq(t, http.MethodGet, baseURL+"/api/v1/llm/profiles", "")
	if response.Code != http.StatusOK {
		t.Fatalf("profiles: %d %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, forbidden := range []string{
		`"provider"`, `"url"`, `"key"`, "private-vision.invalid", "private-text.invalid", `"reasoning"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("profile API leaked %q: %s", forbidden, body)
		}
	}
	for _, required := range []string{
		`"default":"Vision Profile"`, `"name":"Vision Profile"`,
		`"model":"kimi-k3"`, `"vision":true`, `"tool_calls":true`,
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("profile API omitted %q: %s", required, body)
		}
	}
}

func TestCreateRunSelectsExplicitAndDefaultProfiles(t *testing.T) {
	_, baseURL, threadID := setupModelProjectThread(t)
	explicit := `{
		"client_request_id":"req-model-explicit",
		"model":"Text Profile",
		"target":{"artifact":"spec","level":"deck"},
		"interaction":{"intent":"execute"},
		"instruction":"write spec"
	}`
	response := apiReq(t, http.MethodPost, baseURL+"/api/v1/threads/"+threadID+"/runs", explicit)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"model":"Text Profile"`) {
		t.Fatalf("explicit model selection failed: %d %s", response.Code, response.Body.String())
	}
	defaulted := strings.Replace(explicit, `"client_request_id":"req-model-explicit",`, `"client_request_id":"req-model-default",`, 1)
	defaulted = strings.Replace(defaulted, `"model":"Text Profile",`, "", 1)
	response = apiReq(t, http.MethodPost, baseURL+"/api/v1/threads/"+threadID+"/runs", defaulted)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"model":"Vision Profile"`) {
		t.Fatalf("default model selection failed: %d %s", response.Code, response.Body.String())
	}
}

func TestCreateRunRejectsMissingProfileAndCapabilityMismatch(t *testing.T) {
	_, baseURL, threadID := setupModelProjectThread(t)
	missing := `{
		"client_request_id":"req-model-missing",
		"model":"Removed Profile",
		"target":{"artifact":"spec","level":"deck"},
		"interaction":{"intent":"talk"},
		"instruction":"inspect"
	}`
	response := apiReq(t, http.MethodPost, baseURL+"/api/v1/threads/"+threadID+"/runs", missing)
	if response.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(response.Body.String(), "MODEL_PROFILE_NOT_FOUND") {
		t.Fatalf("missing profile error mismatch: %d %s", response.Code, response.Body.String())
	}
	mismatch := `{
		"client_request_id":"req-model-mismatch",
		"model":"Text Profile",
		"target":{"artifact":"presentation","level":"deck"},
		"interaction":{"intent":"execute"},
		"instruction":"make presentation"
	}`
	response = apiReq(t, http.MethodPost, baseURL+"/api/v1/threads/"+threadID+"/runs", mismatch)
	if response.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(response.Body.String(), "MODEL_CAPABILITY_MISMATCH") ||
		!strings.Contains(response.Body.String(), `"required_capability":"vision"`) {
		t.Fatalf("capability mismatch error is not actionable: %d %s", response.Code, response.Body.String())
	}
}

func TestCreateRunIdempotencyHashIncludesResolvedProfile(t *testing.T) {
	_, baseURL, threadID := setupModelProjectThread(t)
	request := `{
		"client_request_id":"req-model-idempotency",
		"model":"Text Profile",
		"target":{"artifact":"spec","level":"deck"},
		"interaction":{"intent":"execute"},
		"instruction":"write spec"
	}`
	first := apiReq(t, http.MethodPost, baseURL+"/api/v1/threads/"+threadID+"/runs", request)
	if first.Code != http.StatusCreated {
		t.Fatalf("first run: %d %s", first.Code, first.Body.String())
	}
	changed := strings.Replace(request, `"model":"Text Profile"`, `"model":"Vision Profile"`, 1)
	second := apiReq(t, http.MethodPost, baseURL+"/api/v1/threads/"+threadID+"/runs", changed)
	if second.Code != http.StatusConflict || !strings.Contains(second.Body.String(), "IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("model change reused idempotency key: %d %s", second.Code, second.Body.String())
	}
}
