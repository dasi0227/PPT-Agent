package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNamingCorrectsRenameOrKeepWithinOneCommand(t *testing.T) {
	for _, action := range []string{"rename", "keep"} {
		t.Run(action, func(t *testing.T) {
			f := newBriefingFixture(t)
			args := map[string]any{"action": action}
			if action == "rename" {
				args["title"] = "季度图表打磨"
			}
			f.provider.Script = []llm.GenerateResponse{
				{Content: llm.TextContent("private model response"), ToolCalls: []llm.ToolCall{{ID: "bad", Name: "other_tool", Args: map[string]any{"private": "private argument"}}}},
				{ToolCalls: []llm.ToolCall{{ID: "valid", Name: renameToolName, Args: args}}},
			}
			core, logs := observer.New(zap.InfoLevel)
			svc := NewNamingService(f.store, f.provider, NewThreadEventHub(f.store), zap.New(core))
			defer svc.Close()
			phases := []int{}
			ctx := WithCommandProgress(context.Background(), func(phase int) error { phases = append(phases, phase); return nil })
			thread, err := svc.GenerateNow(ctx, f.thread.ID)
			if err != nil {
				t.Fatal(err)
			}
			if (action == "rename" && thread.Title != args["title"]) || (action == "keep" && thread.Title != f.thread.Title) {
				t.Fatal("wrong writeback")
			}
			if len(phases) != 3 || phases[0] != 0 || phases[1] != 1 || phases[2] != 2 {
				t.Fatalf("command phases reset: %v", phases)
			}
			requests := f.provider.Requests()
			if len(requests) != 2 || requests[0].RequiredTool != renameToolName || requests[0].ParallelToolCalls == nil || *requests[0].ParallelToolCalls {
				t.Fatal("request budget or constraints incorrect")
			}
			if requests[1].Messages[1].Text() != requests[0].Messages[1].Text() || requests[1].Messages[3].ToolCallID != "bad" || requests[1].Messages[4].Metadata.Origin != "runtime" {
				t.Fatal("snapshot or correction pairing lost")
			}
			var feedback map[string]any
			_ = json.Unmarshal([]byte(requests[1].Messages[3].Text()), &feedback)
			if feedback["code"] != "TOOL_NAME" {
				t.Fatal("naming feedback not specific")
			}
			encoded, _ := json.Marshal(logs.AllUntimed())
			if strings.Contains(string(encoded), "private model response") || strings.Contains(string(encoded), "private argument") {
				t.Fatal("private response data leaked into diagnostics")
			}
			violations := logs.FilterMessage("naming protocol violation").All()
			if len(violations) != 1 || violations[0].ContextMap()["request_id"] == "" {
				t.Fatal("diagnostic request identity missing")
			}
		})
	}
}

func TestNamingUnsupportedResendSharesCorrectionBudget(t *testing.T) {
	for _, succeed := range []bool{true, false} {
		t.Run(map[bool]string{true: "corrected", false: "exhausted"}[succeed], func(t *testing.T) {
			f := newBriefingFixture(t)
			calls := 0
			var initialContext any
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["max_output_tokens"] != float64(128) || body["parallel_tool_calls"] != false {
					t.Error("output or single-call budget changed")
				}
				if calls == 1 {
					initialContext = body["input"].([]any)[0]
					w.WriteHeader(400)
					_, _ = w.Write([]byte(`{"error":{"code":"unsupported_parameter","param":"tool_choice","message":"unsupported field"}}`))
					return
				}
				if _, exists := body["tool_choice"]; exists {
					t.Error("unsupported tool choice retained")
				}
				contextJSON, _ := json.Marshal(initialContext)
				currentJSON, _ := json.Marshal(body["input"].([]any)[0])
				if string(contextJSON) != string(currentJSON) {
					t.Error("naming context changed during resend")
				}
				if calls == 2 || !succeed {
					_, _ = w.Write([]byte(`{"output":[{"type":"reasoning","encrypted_content":"private-opaque"},{"type":"message","phase":"commentary","content":[{"type":"output_text","text":"not submitted"}]},{"type":"function_call","call_id":"bad-1","name":"rename_thread","arguments":"{\"action\":\"keep\"}"},{"type":"function_call","call_id":"bad-2","name":"unknown_tool","arguments":"{}"}]}`))
					return
				}
				// Responses must replay the original reasoning/phase, not a rewritten summary.
				encoded, _ := json.Marshal(body["input"])
				if !strings.Contains(string(encoded), "private-opaque") || !strings.Contains(string(encoded), "commentary") {
					t.Error("Responses continuation lost")
				}
				paired := map[string]bool{}
				for _, raw := range body["input"].([]any) {
					item := raw.(map[string]any)
					if item["type"] == "function_call_output" {
						paired[item["call_id"].(string)] = true
					}
				}
				if !paired["bad-1"] || !paired["bad-2"] {
					t.Error("invalid naming calls missing failure results on wire")
				}
				_, _ = w.Write([]byte(`{"output":[{"type":"function_call","call_id":"keep","name":"rename_thread","arguments":"{\"action\":\"keep\"}"}]}`))
			}))
			defer s.Close()
			provider := llm.NewResponsesAdapter(llm.AdapterConfig{Provider: "gateway", Model: "m", BaseURL: s.URL})
			svc := NewNamingService(f.store, provider, NewThreadEventHub(f.store), zap.NewNop())
			defer svc.Close()
			thread, err := svc.GenerateNow(context.Background(), f.thread.ID)
			if calls != 3 || (succeed && (err != nil || thread.Title != f.thread.Title)) || (!succeed && err == nil) {
				t.Fatalf("calls=%d err=%v thread=%v", calls, err, thread)
			}
			current, _ := f.store.GetThread(context.Background(), f.thread.ID)
			if current.Title != f.thread.Title {
				t.Fatal("invalid result wrote a title")
			}
		})
	}
}

type namingHookProvider struct {
	llm.Provider
	hook func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error)
}

func (p namingHookProvider) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	return p.hook(ctx, req)
}

func TestNamingCorrectionDiscardsCanceledOrSupersededResults(t *testing.T) {
	for _, invalidate := range []string{"cancel", "manual", "project", "new_request"} {
		t.Run(invalidate, func(t *testing.T) {
			f := newBriefingFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var svc *NamingService
			calls := 0
			provider := namingHookProvider{Provider: f.provider, hook: func(_ context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
				calls++
				if calls == 1 {
					return llm.GenerateResponse{Content: llm.TextContent("not submitted")}, nil
				}
				switch invalidate {
				case "cancel":
					cancel()
				case "manual":
					if _, err := svc.ManualRename(context.Background(), f.thread.ID, "用户标题"); err != nil {
						t.Fatal(err)
					}
				case "project":
					svc.mu.Lock()
					svc.projectGenerations[f.project.ID] = "replacement"
					svc.mu.Unlock()
				case "new_request":
					// Explicit generation starts by advancing this same operation version.
					if _, err := f.store.UpdateThreadNamingState(context.Background(), f.thread.ID, nil, nil, true, 10); err != nil {
						t.Fatal(err)
					}
				}
				return llm.GenerateResponse{ToolCalls: []llm.ToolCall{{ID: "stale", Name: renameToolName, Args: map[string]any{"action": "rename", "title": "过期标题"}}}}, nil
			}}
			svc = NewNamingService(f.store, provider, NewThreadEventHub(f.store), zap.NewNop())
			defer svc.Close()
			_, err := svc.GenerateNow(ctx, f.thread.ID)
			current, _ := f.store.GetThread(context.Background(), f.thread.ID)
			if !errors.Is(err, context.Canceled) || calls != 2 || current.Title == "过期标题" {
				t.Fatalf("calls=%d err=%v current=%v", calls, err, current)
			}
			if invalidate == "manual" && current.Title != "用户标题" {
				t.Fatal("manual title overwritten")
			}
		})
	}
}

func TestNamingPermanentProviderErrorDoesNotWithdrawConstraints(t *testing.T) {
	f := newBriefingFixture(t)
	f.provider.GenerateErr = &llm.ProviderError{Kind: llm.ErrBadRequest, StatusCode: 400, Code: "invalid_parameter", Param: "max_output_tokens", Message: "invalid output budget"}
	svc := NewNamingService(f.store, f.provider, NewThreadEventHub(f.store), zap.NewNop())
	defer svc.Close()
	if _, err := svc.GenerateNow(context.Background(), f.thread.ID); err == nil || len(f.provider.Requests()) != 1 {
		t.Fatal("unrelated provider error was retried")
	}
}
