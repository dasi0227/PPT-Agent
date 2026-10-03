package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type submissionTransport func(*http.Request) (*http.Response, error)

func (f submissionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func submissionRequest() GenerateRequest {
	return GenerateRequest{Messages: []Message{{Role: RoleSystem, Content: TextContent("submit policy")}, {Role: RoleUser, Content: TextContent("original input")}}, Tools: []ToolSchema{{Name: "submit_result", Parameters: map[string]any{"type": "object"}, OutputSchema: SubmissionNoReplyOutput("Accept once; errors can be corrected.")}}, MaxOutputTokens: 1000}
}
func validateTestSubmission(response GenerateResponse) error {
	if err := ValidateSubmissionEnvelope(response, "submit_result", true); err != nil {
		return err
	}
	if value, ok := response.ToolCalls[0].Args["value"].(string); !ok || value == "" {
		return SubmissionFailure("INVALID_ARGUMENTS", "/value", "value must be a non-empty string.")
	}
	return nil
}

func TestSubmissionSessionPairsFailuresOnBothProtocols(t *testing.T) {
	for _, protocol := range []string{ProtocolResponses, ProtocolAnthropic} {
		t.Run(protocol, func(t *testing.T) {
			calls := 0
			var bodies []map[string]any
			transport := submissionTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				bodies = append(bodies, body)
				choice := body["tool_choice"].(map[string]any)
				if choice["name"] != "submit_result" {
					t.Fatal("designated tool missing")
				}
				wire := `{"output":[{"type":"function_call","call_id":"accepted","name":"submit_result","arguments":"{\"value\":\"result\"}"}],"usage":{"output_tokens":10}}`
				if protocol == ProtocolResponses {
					if body["parallel_tool_calls"] != false || choice["type"] != "function" {
						t.Fatal("Responses constraints missing")
					}
					if calls == 1 {
						wire = `{"output":[{"type":"reasoning","encrypted_content":"PRIVATE_REASONING"},{"type":"message","phase":"commentary","content":[{"type":"output_text","text":"PRIVATE_TEXT"}]},{"type":"function_call","call_id":"wrong","name":"unknown_tool","arguments":"{}"},{"type":"function_call","call_id":"bad","name":"submit_result","arguments":"{}"}],"usage":{"output_tokens":10}}`
					}
				} else {
					if choice["type"] != "tool" || choice["disable_parallel_tool_use"] != true {
						t.Fatal("Anthropic constraints misplaced")
					}
					wire = `{"type":"message","content":[{"type":"tool_use","id":"accepted","name":"submit_result","input":{"value":"result"}}],"usage":{"output_tokens":10}}`
					if calls == 1 {
						wire = `{"type":"message","content":[{"type":"text","text":"PRIVATE_TEXT"},{"type":"tool_use","id":"wrong","name":"unknown_tool","input":{}},{"type":"tool_use","id":"bad","name":"submit_result","input":{}}],"usage":{"output_tokens":10}}`
					}
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(wire))}, nil
			})
			var provider Provider
			if protocol == ProtocolResponses {
				adapter := NewResponsesAdapter(AdapterConfig{Provider: "gateway", Model: "model", BaseURL: "https://example.invalid"})
				adapter.http.client.Transport = transport
				provider = adapter
			} else {
				adapter := NewAnthropicAdapter(AdapterConfig{Provider: "gateway", Model: "model", BaseURL: "https://example.invalid"})
				adapter.http.client.Transport = transport
				provider = adapter
			}
			session := NewSubmissionSession("test", 1000)
			var diagnostics []map[string]any
			session.Diagnose = func(d map[string]any) { diagnostics = append(diagnostics, d) }
			response, err := session.Generate(context.Background(), provider, submissionRequest(), validateTestSubmission, "Call submit_result alone.")
			if err != nil || response.ToolCalls[0].Args["value"] != "result" || calls != 2 || session.additional != 1 || session.modelRequests != 2 {
				t.Fatalf("calls=%d response=%v err=%v", calls, response, err)
			}
			if bodies[1]["max_output_tokens"] != float64(990) && protocol == ProtocolResponses {
				t.Fatal("output budget reset during correction")
			}
			paired := map[string]bool{}
			if protocol == ProtocolResponses {
				encoded, _ := json.Marshal(bodies[1]["input"])
				if !strings.Contains(string(encoded), "PRIVATE_REASONING") || !strings.Contains(string(encoded), "commentary") {
					t.Fatal("Responses continuation rewritten")
				}
				for _, raw := range bodies[1]["input"].([]any) {
					item := raw.(map[string]any)
					if item["type"] == "function_call_output" {
						paired[item["call_id"].(string)] = true
					}
				}
			} else {
				for _, raw := range bodies[1]["messages"].([]any) {
					message := raw.(map[string]any)
					for _, rawPart := range message["content"].([]any) {
						part := rawPart.(map[string]any)
						if part["type"] == "tool_result" {
							if part["is_error"] != true {
								t.Fatal("failed call not marked as error")
							}
							paired[part["tool_use_id"].(string)] = true
						}
					}
				}
			}
			if !paired["wrong"] || !paired["bad"] {
				t.Fatal("a rejected call has no failure result")
			}
			encoded, _ := json.Marshal(diagnostics)
			if strings.Contains(string(encoded), "PRIVATE_") {
				t.Fatal("private model data entered diagnostics")
			}
		})
	}
}

type submissionScript struct {
	requests int
	caps     Capabilities
	generate func(context.Context, GenerateRequest) (GenerateResponse, error)
}

func (p *submissionScript) Name() string               { return "fake" }
func (p *submissionScript) Model() string              { return "model" }
func (p *submissionScript) Capabilities() Capabilities { return p.caps }
func (p *submissionScript) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	p.requests++
	return p.generate(ctx, req)
}
func testValidSubmission() GenerateResponse {
	return GenerateResponse{ToolCalls: []ToolCall{{ID: "result", Name: "submit_result", Args: map[string]any{"value": "accepted"}}}, Usage: Usage{OutputTokens: 10}}
}

func TestSubmissionSessionSharesQuotaAcrossResendsAndBatches(t *testing.T) {
	p := &submissionScript{}
	p.generate = func(context.Context, GenerateRequest) (GenerateResponse, error) {
		switch p.requests {
		case 1:
			return GenerateResponse{}, &UnsupportedToolConstraintError{Strategy: ToolStrategy{Provider: "fake", Protocol: ProtocolResponses, Model: "model", RequiredTool: "submit_result"}, Field: "tool_choice", Cause: ErrBadRequest}
		case 3:
			return testValidSubmission(), nil
		default:
			return GenerateResponse{Content: TextContent("not submitted"), Usage: Usage{OutputTokens: 10}}, nil
		}
	}
	s := NewSubmissionSession("compact", 1000)
	s.Diagnose = func(map[string]any) {}
	if _, err := s.Generate(context.Background(), p, submissionRequest(), validateTestSubmission, "submit"); err != nil || p.requests != 3 || s.additional != 2 || s.corrections != 1 || s.unsupported != 1 {
		t.Fatalf("requests=%d err=%v", p.requests, err)
	}
	// The next required input batch gets its base request, but no new corrections.
	if _, err := s.Generate(context.Background(), p, submissionRequest(), validateTestSubmission, "submit"); err == nil || p.requests != 4 || s.additional != 2 {
		t.Fatalf("batch reset correction quota: %d %v", p.requests, err)
	}
}

func TestSubmissionSessionStopsAtSafetyAndBudgetBoundaries(t *testing.T) {
	for _, kind := range []string{"normal", "exhausted", "unsafe", "reused_id", "cancel", "deadline", "context", "output", "business"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "deadline" {
				var end context.CancelFunc
				ctx, end = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer end()
			}
			p := &submissionScript{}
			p.generate = func(context.Context, GenerateRequest) (GenerateResponse, error) {
				if kind == "cancel" {
					cancel()
				}
				if kind == "unsafe" {
					return GenerateResponse{ToolCalls: []ToolCall{{Name: "unknown"}, {ID: "duplicate", Name: "unknown"}, {ID: "duplicate", Name: "unknown"}}}, nil
				}
				if kind == "reused_id" {
					return GenerateResponse{ToolCalls: []ToolCall{{ID: "reused", Name: "unknown"}}}, nil
				}
				if kind == "exhausted" {
					return GenerateResponse{Content: TextContent("invalid"), Usage: Usage{OutputTokens: 10}}, nil
				}
				if kind == "business" {
					return GenerateResponse{}, errors.New("execution failed")
				}
				return testValidSubmission(), nil
			}
			if kind == "context" {
				p.caps.ContextWindowTokens = 1
			}
			budget := 1000
			if kind == "output" {
				budget = 1
			}
			s := NewSubmissionSession("test", budget)
			s.Diagnose = func(map[string]any) {}
			_, err := s.Generate(ctx, p, submissionRequest(), validateTestSubmission, "submit")
			if kind == "normal" {
				if err != nil || p.requests != 1 {
					t.Fatalf("normal path: %v %d", err, p.requests)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid or expired response accepted")
			}
			want := 1
			if kind == "context" || kind == "deadline" {
				want = 0
			}
			if kind == "exhausted" {
				want = 3
			}
			if kind == "reused_id" {
				want = 2
			}
			if p.requests != want {
				t.Fatalf("boundary retried: %d want %d", p.requests, want)
			}
		})
	}
}

func TestSubmissionSessionSeparatesNetworkRetryFromCorrection(t *testing.T) {
	calls := 0
	adapter := NewResponsesAdapter(AdapterConfig{Provider: "gateway", Model: "model", BaseURL: "https://example.invalid"})
	adapter.http.client.Transport = submissionTransport(func(*http.Request) (*http.Response, error) {
		calls++
		status, wire := 200, `{"output":[{"type":"function_call","call_id":"accepted","name":"submit_result","arguments":"{\"value\":\"result\"}"}],"usage":{"output_tokens":10}}`
		if calls == 1 {
			status, wire = 503, `{"error":{"code":"unavailable","message":"PRIVATE_PROVIDER_MESSAGE"}}`
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(wire))}, nil
	})
	session := NewSubmissionSession("test", 1000)
	var diagnostics []map[string]any
	session.Diagnose = func(d map[string]any) { diagnostics = append(diagnostics, d) }
	if _, err := session.Generate(context.Background(), adapter, submissionRequest(), validateTestSubmission, "submit"); err != nil {
		t.Fatal(err)
	}
	if session.requests != 1 || session.modelRequests != 2 || session.httpRequests != 2 || session.networkRetries != 1 || session.additional != 0 || calls != 2 {
		t.Fatalf("retry counters conflated: %+v", session)
	}
	encoded, _ := json.Marshal(diagnostics)
	if strings.Contains(string(encoded), "PRIVATE_") {
		t.Fatal("provider message leaked into diagnostics")
	}
}
