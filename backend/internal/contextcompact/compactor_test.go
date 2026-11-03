package contextcompact

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
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

func TestCompactorProjectsHistoryWithoutMutatingSource(t *testing.T) {
	provider := &llmtest.FakeProvider{
		Caps:   llm.Capabilities{ContextWindowTokens: 65536},
		Script: []llm.GenerateResponse{compactResponse("完整摘要", validSummary)},
	}
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("<compact_summary>previous decisions</compact_summary>"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "summary"}},
		{Role: llm.RoleAssistant, Content: llm.TextContent(strings.Repeat("old ", 16000))},
		{Role: llm.RoleAssistant, Content: llm.TextContent("latest evidence")},
	}
	before, _ := json.Marshal(messages)
	result, err := New(provider).Compact(context.Background(), messages, Options{MainWindowTokens: 65536, OutputReserveTokens: 16384})
	requests := provider.Requests()
	if err != nil || len(requests) != 1 {
		t.Fatalf("err=%v calls=%d", err, len(requests))
	}
	want := "<previous_summary>\nprevious decisions\n</previous_summary>\n\n" +
		"<compressed_context>\n<assistant>\n" + messages[1].Text() + "\n</assistant>\n</compressed_context>\n\n" +
		"<retained_context>\n<assistant>\nlatest evidence\n</assistant>\n</retained_context>"
	if requests[0].Messages[1].Text() != want || requests[0].MaxOutputTokens != 0 {
		t.Fatal("compaction lost context boundaries or applied a command token limit")
	}
	if !result.Changed || len(result.Messages) != 2 || !reflect.DeepEqual(result.Messages[1], messages[2]) {
		t.Fatalf("previous summary and old history were not replaced together: %+v", result)
	}
	after, _ := json.Marshal(messages)
	if string(before) != string(after) {
		t.Fatal("compaction modified the source history")
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

func TestCompactorRejectsNonToolAndInvalidToolResponses(t *testing.T) {
	tests := []struct {
		name     string
		response llm.GenerateResponse
	}{
		{name: "plain text", response: llm.GenerateResponse{Content: llm.TextContent(validSummary)}},
		{name: "wrong tool", response: llm.GenerateResponse{ToolCalls: []llm.ToolCall{{Name: "context_compaction", Args: map[string]any{"title": "标题", "content": validSummary}}}}},
		{name: "multiple tools", response: llm.GenerateResponse{ToolCalls: []llm.ToolCall{{Name: compactContextToolName}, {Name: compactContextToolName}}}},
		{name: "invalid title", response: compactResponse("# invalid\ntitle", validSummary)},
		{name: "missing content", response: compactResponse("有效标题", " ")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &llmtest.FakeProvider{
				Caps: llm.Capabilities{ContextWindowTokens: 65536},
			}
			for i := range 3 {
				next := test.response
				next.ToolCalls = append([]llm.ToolCall(nil), test.response.ToolCalls...)
				if llm.CanReplaySubmission(test.response) {
					for index := range next.ToolCalls {
						next.ToolCalls[index].ID += "-" + strconv.Itoa(i)
					}
				}
				provider.Script = append(provider.Script, next)
			}
			_, err := New(provider).Compact(context.Background(), []llm.Message{{
				Role: llm.RoleAssistant, Content: llm.TextContent(strings.Repeat("old context ", 8000)),
			}}, Options{MainWindowTokens: 65536, OutputReserveTokens: 16384})
			expected := 1
			if llm.CanReplaySubmission(test.response) {
				expected = 3
			}
			if err == nil || len(provider.Requests()) != expected {
				t.Fatalf("err=%v calls=%d", err, len(provider.Requests()))
			}
		})
	}
}

func TestCompactorSkipsModelForEmptyCompressionRegion(t *testing.T) {
	provider := &llmtest.FakeProvider{Caps: llm.Capabilities{ContextWindowTokens: 65536}}
	messages := []llm.Message{{
		Role:    llm.RoleUser,
		Content: llm.TextContent("keep"), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: "r1"},
	}}
	result, err := New(provider).Compact(context.Background(), messages, Options{MainWindowTokens: 65536, OutputReserveTokens: 16384})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed || !reflect.DeepEqual(result.Messages, messages) || result.Title != fallbackTitle || result.Content != emptySummary() || len(provider.Requests()) != 0 {
		t.Fatalf("unexpected empty result: %+v calls=%d", result, len(provider.Requests()))
	}
}
