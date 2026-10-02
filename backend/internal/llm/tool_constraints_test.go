package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

func TestToolConstraintsReachBothWireProtocols(t *testing.T) {
	for _, protocol := range []string{ProtocolResponses, ProtocolAnthropic} {
		t.Run(protocol, func(t *testing.T) {
			var bodies []map[string]any
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				bodies = append(bodies, body)
				if protocol == ProtocolResponses {
					_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`))
				} else {
					_, _ = w.Write([]byte(`{"type":"message","content":[{"type":"text","text":"ok"}]}`))
				}
			}))
			defer s.Close()
			var p Provider = NewResponsesAdapter(AdapterConfig{Provider: "gateway", Model: "m", BaseURL: s.URL})
			if protocol == ProtocolAnthropic {
				p = NewAnthropicAdapter(AdapterConfig{Provider: "gateway", Model: "m", BaseURL: s.URL})
			}
			parallel := false
			req := GenerateRequest{Tools: []ToolSchema{{Name: "rename_thread", OutputSchema: SubmissionNoReplyOutput("A valid submission ends without a reply; errors may be corrected.")}}, RequiredTool: "rename_thread", ParallelToolCalls: &parallel}
			if _, err := p.Generate(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			choice := bodies[0]["tool_choice"].(map[string]any)
			if choice["name"] != "rename_thread" {
				t.Fatal("required tool missing")
			}
			if protocol == ProtocolResponses {
				if choice["type"] != "function" || bodies[0]["parallel_tool_calls"] != false {
					t.Fatal("Responses wire constraints incorrect")
				}
			} else {
				if choice["type"] != "tool" || choice["disable_parallel_tool_use"] != true {
					t.Fatal("Anthropic wire constraints incorrect")
				}
				if _, exists := bodies[0]["parallel_tool_calls"]; exists {
					t.Fatal("parallel limit incorrectly at top level")
				}
			}
			description := bodies[0]["tools"].([]any)[0].(map[string]any)["description"].(string)
			if !strings.Contains(description, `"x-error-schema"`) || !strings.Contains(description, `"next_action"`) {
				t.Fatal("error output contract not projected")
			}
			req.RequiredTool, req.ParallelToolCalls = "", nil
			if _, err := p.Generate(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			if _, exists := bodies[1]["tool_choice"]; exists {
				t.Fatal("unset caller acquired tool choice")
			}
			if _, exists := bodies[1]["parallel_tool_calls"]; exists {
				t.Fatal("unset caller acquired parallel restriction")
			}
			req.RequiredTool = "undisclosed"
			if _, err := p.Generate(context.Background(), req); !errors.Is(err, ErrBadRequest) || len(bodies) != 2 {
				t.Fatal("undisclosed required tool sent upstream")
			}
		})
	}
}

func TestUnsupportedToolConstraintsRequirePreciseMachineEvidence(t *testing.T) {
	parallel := false
	req := GenerateRequest{Tools: []ToolSchema{{Name: "rename_thread"}}, RequiredTool: "rename_thread", ParallelToolCalls: &parallel, ToolConstraintPolicy: &ToolConstraintPolicy{}}
	strategy, _ := toolStrategy(req, "gateway", ProtocolAnthropic, "m", "https://endpoint/a")
	for _, failure := range []*ProviderError{
		{Kind: ErrBadRequest, StatusCode: 400, Code: "invalid_request_error", Param: "tool_choice", Message: "tool_choice unsupported"},
		{Kind: ErrBadRequest, StatusCode: 400, Code: "unsupported_parameter", Param: "max_tokens"},
		{Kind: ErrBadRequest, StatusCode: 401, Code: "unsupported_parameter", Param: "tool_choice"},
		{Kind: ErrUnavailable, StatusCode: 429, Code: "unsupported_parameter", Param: "tool_choice"},
	} {
		if req.ToolConstraintPolicy.Learn(classifyToolConstraintError(failure, strategy)) {
			t.Fatal("unrelated provider error caused withdrawal")
		}
	}
	err := classifyToolConstraintError(&ProviderError{Kind: ErrBadRequest, StatusCode: 400, Code: "unsupported_value", Param: "tool_choice.type"}, strategy)
	if !req.ToolConstraintPolicy.Learn(err) {
		t.Fatal("explicit unsupported field not learned")
	}
	auto, _ := toolStrategy(req, "gateway", ProtocolAnthropic, "m", "https://endpoint/a")
	if auto.RequiredTool != "" || auto.ParallelToolCalls == nil {
		t.Fatal("supported single-call limit was discarded")
	}
	for _, identity := range [][3]string{{ProtocolAnthropic, "backup", "https://endpoint/a"}, {ProtocolResponses, "m", "https://endpoint/a"}, {ProtocolAnthropic, "m", "https://endpoint/b"}} {
		fresh, _ := toolStrategy(req, "gateway", identity[0], identity[1], identity[2])
		if fresh.RequiredTool != "rename_thread" {
			t.Fatal("capability evidence leaked to another route")
		}
	}
}

func TestFallbackReevaluatesToolConstraintsAndDropsContinuation(t *testing.T) {
	mainCalls, backupCalls := 0, 0
	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainCalls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if mainCalls == 1 {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":{"code":"unsupported_parameter","param":"tool_choice","message":"unsupported"}}`))
		} else {
			if _, exists := body["tool_choice"]; exists {
				t.Error("known unsupported choice still sent")
			}
			if body["parallel_tool_calls"] != false {
				t.Error("single-call restriction lost")
			}
			w.WriteHeader(503)
		}
	}))
	defer mainServer.Close()
	backupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupCalls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		choice, ok := body["tool_choice"].(map[string]any)
		if !ok || choice["type"] != "tool" || choice["name"] != "rename_thread" || choice["disable_parallel_tool_use"] != true {
			t.Error("fallback did not reevaluate actual protocol strategy")
		}
		_, _ = w.Write([]byte(`{"type":"message","content":[{"type":"tool_use","id":"kept","name":"rename_thread","input":{"action":"keep"}}]}`))
	}))
	defer backupServer.Close()
	main := NewResponsesAdapter(AdapterConfig{Provider: "gateway", Model: "m", BaseURL: mainServer.URL})
	main.http.maxRetries = 0
	backup := NewAnthropicAdapter(AdapterConfig{Provider: "gateway", Model: "m", BaseURL: backupServer.URL})
	r, err := NewRegistryWithProfiles("Main", []Profile{NewTestProfile("Main", mainServer.URL, main), NewTestProfile("Backup", backupServer.URL, backup)}, RoadConfig{Main: config.MainRoadLLMConfig{Default: "Main", Fallback: "Backup"}})
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := r.RoutedProfile("main", "")
	parallel := false
	policy := &ToolConstraintPolicy{}
	req := GenerateRequest{Messages: []Message{{Role: RoleUser, Content: TextContent("title")}}, Tools: []ToolSchema{{Name: "rename_thread"}}, RequiredTool: "rename_thread", ParallelToolCalls: &parallel, ToolConstraintPolicy: policy}
	_, err = profile.Adapter().Generate(context.Background(), req)
	if !policy.Learn(err) {
		t.Fatalf("explicit unsupported error missing: %v", err)
	}
	// Existing state belongs to the primary; Anthropic must never receive it.
	opaque, _ := json.Marshal(responsesState{Protocol: ProtocolResponses, EndpointHash: continuationFingerprint(main.http.baseURL), ToolsHash: continuationFingerprint(req.Tools)})
	req.Continuation = &ProviderContinuation{Provider: "gateway", Model: "m", Opaque: opaque}
	response, err := profile.Adapter().Generate(context.Background(), req)
	if err != nil || len(response.ToolCalls) != 1 || backupCalls != 1 || mainCalls != 2 {
		t.Fatalf("response=%v err=%v main=%d backup=%d", response, err, mainCalls, backupCalls)
	}
}
