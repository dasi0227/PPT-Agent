package contextcompact

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

const validSummary = "## 目标与意图\n目标\n\n## 已完成改动\n完成\n\n## 关键决策\n决策\n\n## 未决问题\n问题\n\n## 下一步\n继续"

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
		{Role: llm.RoleAssistant, Content: llm.TextContent("old analysis")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "one", Name: "read_resource"}}},
		{Role: llm.RoleTool, ToolCallID: "one", Content: llm.TextContent("first result")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "two", Name: "run_command"}}},
		{Role: llm.RoleTool, ToolCallID: "two", Content: llm.TextContent("second result")},
	}
	result, err := New(provider).Compact(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.Requests()) != 1 {
		t.Fatalf("model calls=%d", len(provider.Requests()))
	}
	if provider.Requests()[0].Messages[0].Text() != prompts.MustLoad("command.compact").Body {
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
	if result.Title != "收敛上下文压缩协议" || result.Content != validSummary {
		t.Fatalf("unexpected structured result: %+v", result)
	}
	if len(result.Messages) != 6 || !strings.Contains(result.Messages[0].Text(), "<context_summary>") {
		t.Fatalf("unexpected replacement: %+v", result.Messages)
	}
	if strings.Contains(result.Messages[0].Text(), result.Title) {
		t.Fatalf("timeline title leaked into durable transcript: %q", result.Messages[0].Text())
	}
	if !strings.Contains(result.Messages[1].Text(), "first instruction") {
		t.Fatalf("user instruction was not retained: %+v", result.Messages)
	}
}

func TestCompactorRejectsOversizedUnitWithoutDroppingInput(t *testing.T) {
	provider := &llmtest.FakeProvider{Caps: llm.Capabilities{ContextWindowTokens: 5200}}
	messages := []llm.Message{{Role: llm.RoleAssistant, Content: llm.TextContent(strings.Repeat("old ", 4000))}}
	before, _ := json.Marshal(messages)
	_, err := New(provider).Compact(context.Background(), messages)
	if !errors.Is(err, ErrCompactionUnitTooLarge) || len(provider.Requests()) != 0 {
		t.Fatalf("err=%v calls=%d", err, len(provider.Requests()))
	}
	after, _ := json.Marshal(messages)
	if string(before) != string(after) {
		t.Fatal("failed compaction modified the source history")
	}
}

func TestCompactorProcessesEveryBatchAndCarriesPreviousSummary(t *testing.T) {
	first := llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent("first-fact " + strings.Repeat("data ", 1000))}
	second := llm.Message{Role: llm.RoleAssistant, Content: llm.TextContent("second-fact " + strings.Repeat("data ", 1000))}
	provider := &llmtest.FakeProvider{
		Caps:   llm.Capabilities{ContextWindowTokens: compactRequestTokens([]llm.Message{first}) + maxSummaryTokens + outputSafetyTokens + 500},
		Script: []llm.GenerateResponse{compactResponse("第一批", "first-fact summary"), compactResponse("完整摘要", "first-fact and second-fact")},
	}
	result, err := New(provider).Compact(context.Background(), []llm.Message{first, second})
	if err != nil {
		t.Fatal(err)
	}
	requests := provider.Requests()
	if len(requests) != 2 {
		t.Fatalf("batches=%d", len(requests))
	}
	if !strings.Contains(requests[0].Messages[1].Text(), "first-fact") || !strings.Contains(requests[1].Messages[1].Text(), "first-fact summary") || !strings.Contains(requests[1].Messages[1].Text(), "second-fact") {
		t.Fatal("a batch or its preceding summary was skipped")
	}
	if result.Content != "first-fact and second-fact" {
		t.Fatal("wrong combined summary", result.Content)
	}
}

func TestCompactorPreservesUnresolvedToolRound(t *testing.T) {
	provider := &llmtest.FakeProvider{Caps: llm.Capabilities{ContextWindowTokens: 65536}}
	messages := []llm.Message{
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "question", Name: "ask_user"}}},
		{Role: llm.RoleUser, Content: llm.TextContent("new input")},
	}
	result, err := New(provider).Compact(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.Requests()) != 0 || len(result.Messages) != len(messages) || result.Messages[0].ToolCalls[0].ID != "question" {
		t.Fatal("pending interaction was compacted")
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
				Caps:   llm.Capabilities{ContextWindowTokens: 65536},
				Script: []llm.GenerateResponse{test.response},
			}
			_, err := New(provider).Compact(context.Background(), []llm.Message{{
				Role: llm.RoleAssistant, Content: llm.TextContent("old context"),
			}})
			if err == nil || len(provider.Requests()) != 1 {
				t.Fatalf("err=%v calls=%d", err, len(provider.Requests()))
			}
		})
	}
}

func TestCompactorSkipsModelForEmptyCompressionRegion(t *testing.T) {
	provider := &llmtest.FakeProvider{Caps: llm.Capabilities{ContextWindowTokens: 65536}}
	result, err := New(provider).Compact(context.Background(), []llm.Message{{
		Role:    llm.RoleUser,
		Content: llm.TextContent("keep"), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: "r1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != fallbackTitle || result.Content != emptySummary() || len(provider.Requests()) != 0 {
		t.Fatalf("unexpected empty result: %+v calls=%d", result, len(provider.Requests()))
	}
}

func TestCompactRequestTokensIncludeLocalToolSchema(t *testing.T) {
	withoutTool := contextengine.EstimateTextTokens(prompts.MustLoad("command.compact").Body) + 16
	if got := compactRequestTokens(nil); got <= withoutTool {
		t.Fatalf("request tokens=%d without tool=%d", got, withoutTool)
	}
}
