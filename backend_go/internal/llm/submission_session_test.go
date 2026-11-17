package llm

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"net/http"
	"testing"
)

type submissionTransport func(*http.Request) (*http.Response, error)

func (f submissionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func submissionRequest() GenerateRequest {
	return GenerateRequest{Messages: []Message{{Role: RoleSystem, Content: TextContent("submit policy")}, {Role: RoleUser, Content: TextContent("original input")}}, Tools: []ToolSchema{{Name: "submit_result", Parameters: map[string]any{"type": "object"}, Description: "Submit the result; success ends without a tool reply."}}}
}

func validateTestSubmission(response GenerateResponse) error {
	if err := ValidateSubmissionEnvelope(response, "submit_result", true); err != nil {
		return err
	}
	if value, ok := response.ToolCalls[0].Args["value"].(string); !ok || value == "" {
		return SubmissionFailure(model.CodeToolArgumentInvalid, "/value", "value must be a non-empty string.")
	}
	return nil
}

type submissionScript struct {
	requests int
	caps     Capabilities
	generate func(context.Context, GenerateRequest) (GenerateResponse, error)
}

func (p *submissionScript) Name() string { return "fake" }

func (p *submissionScript) Model() string { return "model" }

func (p *submissionScript) Capabilities() Capabilities { return p.caps }

func (p *submissionScript) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	p.requests++
	return p.generate(ctx, req)
}

func testValidSubmission() GenerateResponse {
	return GenerateResponse{ToolCalls: []ToolCall{{ID: "result", Name: "submit_result", Args: map[string]any{"value": "accepted"}}}, Usage: Usage{OutputTokens: 10}}
}

func TestSubmissionSessionStopsAtSafetyAndRetryBoundaries(t *testing.T) {
	for _, kind := range []string{"normal", "exhausted", "unsafe", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			p := &submissionScript{}
			p.generate = func(context.Context, GenerateRequest) (GenerateResponse, error) {
				if kind == "cancel" {
					cancel()
				}
				if kind == "unsafe" {
					return GenerateResponse{ToolCalls: []ToolCall{{Name: "unknown"}, {ID: "duplicate", Name: "unknown"}, {ID: "duplicate", Name: "unknown"}}}, nil
				}
				if kind == "exhausted" {
					return GenerateResponse{Content: TextContent("invalid"), Usage: Usage{OutputTokens: 10}}, nil
				}
				return testValidSubmission(), nil
			}
			s := NewSubmissionSession("test")
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
			if kind == "exhausted" {
				want = 3
			}
			if p.requests != want {
				t.Fatalf("boundary retried: %d want %d", p.requests, want)
			}
		})
	}
}
