package sessionstate

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func TestModelRequestPreservesNativeMessagesAndKeepsLiveCallbacks(t *testing.T) {
	parallel := false
	request := llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent("frozen system prompt")},
			{Role: llm.RoleUser, Content: []llm.ContentPart{{Type: "text", Text: "question"}, {Type: "image", ImageRef: "asset:stable"}}, Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: "run"}},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "call", Name: "read_resource", Args: map[string]any{"path": "a"}}}, Replay: &llm.AssistantReplay{Source: "provider", Blocks: []json.RawMessage{json.RawMessage(`{"signature":"signed"}`)}}},
			{Role: llm.RoleTool, ToolCallID: "call", Content: llm.TextContent("result")},
		},
		Tools: []llm.ToolSchema{{Name: "read_resource"}}, RequiredTool: "read_resource", ParallelToolCalls: &parallel,
		Continuation:    &llm.ProviderContinuation{Provider: "provider", Model: "model", Opaque: json.RawMessage(`{"cursor":"private"}`)},
		MaxOutputTokens: 2048, PauseOnFallback: true,
	}
	raw, err := EncodeModelRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	policy := &llm.ToolConstraintPolicy{}
	restored, err := DecodeModelRequest(raw, llm.GenerateRequest{OnRetry: func(int) { called = true }, ToolConstraintPolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Messages, request.Messages) || !reflect.DeepEqual(restored.Tools, request.Tools) || !reflect.DeepEqual(restored.Continuation, request.Continuation) || restored.MaxOutputTokens != request.MaxOutputTokens || restored.RequiredTool != request.RequiredTool || restored.ParallelToolCalls == nil || *restored.ParallelToolCalls || !restored.PauseOnFallback {
		t.Fatal("frozen request lost native data or dispatch parameters")
	}
	restored.OnRetry(1)
	if !called || restored.ToolConstraintPolicy != policy {
		t.Fatal("decoding replaced process-local callbacks or constraint state")
	}
}

func TestModelResponsePreservesReplayAndUsage(t *testing.T) {
	response := llm.GenerateResponse{
		Thinking: []string{"reasoning"}, Content: llm.TextContent("answer"),
		ToolCalls:    []llm.ToolCall{{ID: "stable-call", Name: "read_resource", Args: map[string]any{"path": "a"}}},
		Replay:       &llm.AssistantReplay{Source: "provider", Blocks: []json.RawMessage{json.RawMessage(`{"signature":"signed"}`)}},
		Continuation: &llm.ProviderContinuation{Provider: "provider", Model: "model", Opaque: json.RawMessage(`{"cursor":"private"}`)},
		Usage:        llm.Usage{InputTokens: 20, OutputTokens: 10, TotalTokens: 30},
	}
	raw, err := EncodeModelResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodeModelResponse(raw)
	if err != nil || !reflect.DeepEqual(restored, response) {
		t.Fatalf("response replay changed: %+v, %v", restored, err)
	}
}
