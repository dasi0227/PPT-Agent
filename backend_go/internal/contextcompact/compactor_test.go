package contextcompact

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"reflect"
	"strings"
	"testing"
)

const validSummary = "## 目标与约束\n目标\n\n## 工作进展\n完成\n\n## 关键决策\n决策\n\n## 未决事项\n问题\n\n## 下一步\n继续"

func compactResponse(title, summary string) llm.GenerateResponse {
	return llm.GenerateResponse{ToolCalls: []llm.ToolCall{{
		ID: "compact-1", Name: compactContextToolName,
		Args: map[string]any{"title": title, "content": summary},
	}}}
}

func TestCompactorUsesOneCallAndRetainsUsersAndRecentToolRounds(t *testing.T) {
	provider := &llmtest.FakeProvider{
		Caps:   llm.Capabilities{ContextWindowTokens: 65536},
		Script: []llm.GenerateResponse{compactResponse("收敛上下文压缩协议", validSummary)},
	}
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("first instruction"), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: "r1"}},
		{Role: llm.RoleAssistant, Content: llm.TextContent(strings.Repeat("old analysis ", 8000))},
		{Role: llm.RoleAssistant, Content: llm.TextContent(""), ToolCalls: []llm.ToolCall{{ID: "one", Name: "read_resource"}}},
		{Role: llm.RoleTool, ToolCallID: "one", Content: llm.TextContent("first result")},
		{Role: llm.RoleAssistant, Content: llm.TextContent(""), ToolCalls: []llm.ToolCall{{ID: "two", Name: "run_command"}}},
		{Role: llm.RoleTool, ToolCallID: "two", Content: llm.TextContent("second result")},
	}
	result, err := New(provider).Compact(context.Background(), messages, Options{MainWindowTokens: 65536, OutputReserveTokens: 16384})
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.Requests()) != 1 {
		t.Fatalf("model calls=%d", len(provider.Requests()))
	}
	if provider.Requests()[0].Messages[0].Text() != prompts.RenderGroup("system_prompt", prompts.MustLoad("command_compact")) {
		t.Fatal("wrong compaction policy")
	}
	request := provider.Requests()[0]
	if len(request.Tools) != 1 || request.Tools[0].Name != compactContextToolName {
		t.Fatalf("unexpected compaction tools: %+v", request.Tools)
	}
	properties, _ := request.Tools[0].Parameters["properties"].(map[string]any)
	if properties["title"] == nil || properties["content"] == nil {
		t.Fatalf("compact_context schema is incomplete: %+v", request.Tools[0])
	}
	if !result.Changed || result.Title != "收敛上下文压缩协议" || result.Content != validSummary {
		t.Fatalf("unexpected structured result: %+v", result)
	}
	if len(result.Messages) != 6 || !strings.Contains(result.Messages[0].Text(), "<compact_summary>") {
		t.Fatalf("unexpected replacement: %+v", result.Messages)
	}
	if strings.Contains(result.Messages[0].Text(), result.Title) {
		t.Fatalf("timeline title leaked into durable transcript: %q", result.Messages[0].Text())
	}
	if !reflect.DeepEqual(result.Messages[1], messages[0]) || !reflect.DeepEqual(result.Messages[2:], messages[2:]) {
		t.Fatalf("user instruction or recent tool rounds changed: %+v", result.Messages)
	}
}

func TestCompactorRejectsUnresolvedToolRoundWithoutMutatingSource(t *testing.T) {
	provider := &llmtest.FakeProvider{Caps: llm.Capabilities{ContextWindowTokens: 65536}}
	messages := []llm.Message{
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "question", Name: "ask_user"}}},
		{Role: llm.RoleUser, Content: llm.TextContent("new input")},
	}
	before, _ := json.Marshal(messages)
	_, err := New(provider).Compact(context.Background(), messages, Options{MainWindowTokens: 65536, OutputReserveTokens: 16384})
	after, _ := json.Marshal(messages)
	if !errors.Is(err, ErrUnclosedHistory) || len(provider.Requests()) != 0 || string(after) != string(before) {
		t.Fatalf("pending interaction was not rejected intact: %v", err)
	}
}
