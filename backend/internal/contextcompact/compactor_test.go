package contextcompact

import (
	"context"
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
		Args: map[string]any{"title": title, "summary": summary},
	}}}
}

func TestCompactorUsesOneCallAndRetainsUsersAndRecentToolRounds(t *testing.T) {
	provider := &llmtest.FakeProvider{
		Caps:   llm.Capabilities{ContextWindowTokens: 65536},
		Script: []llm.GenerateResponse{compactResponse("收敛上下文压缩协议", validSummary)},
	}
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("<run_user_instruction run_id=\"r1\">\nfirst instruction\n</run_user_instruction>")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("old analysis")},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "one", Name: "read_ppt"}}},
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
	if properties["title"] == nil || properties["summary"] == nil {
		t.Fatalf("compact_context schema is incomplete: %+v", request.Tools[0])
	}
	if result.Title != "收敛上下文压缩协议" || result.Summary != validSummary {
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

func TestCompactorTruncatesOldestInputWithoutRecursion(t *testing.T) {
	provider := &llmtest.FakeProvider{
		Caps:   llm.Capabilities{ContextWindowTokens: 5200},
		Script: []llm.GenerateResponse{compactResponse("保留最近工具轮次", validSummary)},
	}
	messages := []llm.Message{
		{Role: llm.RoleAssistant, Content: llm.TextContent(strings.Repeat("old ", 4000))},
		{Role: llm.RoleAssistant, Content: llm.TextContent(strings.Repeat("new ", 200))},
	}
	result, err := New(provider).Compact(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}
	if result.DroppedInputTokens == 0 || len(provider.Requests()) != 1 {
		t.Fatalf("expected deterministic truncation and one call: %+v", result)
	}
}

func TestCompactorRejectsNonToolAndInvalidToolResponses(t *testing.T) {
	tests := []struct {
		name     string
		response llm.GenerateResponse
	}{
		{name: "plain text", response: llm.GenerateResponse{Content: llm.TextContent(validSummary)}},
		{name: "wrong tool", response: llm.GenerateResponse{ToolCalls: []llm.ToolCall{{Name: "context_compaction", Args: map[string]any{"title": "标题", "summary": validSummary}}}}},
		{name: "multiple tools", response: llm.GenerateResponse{ToolCalls: []llm.ToolCall{{Name: compactContextToolName}, {Name: compactContextToolName}}}},
		{name: "missing summary", response: compactResponse("有效标题", " ")},
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

func TestCompactorFallsBackForInvalidTitleWithoutRetry(t *testing.T) {
	provider := &llmtest.FakeProvider{
		Caps:   llm.Capabilities{ContextWindowTokens: 65536},
		Script: []llm.GenerateResponse{compactResponse("# invalid\ntitle", validSummary)},
	}
	result, err := New(provider).Compact(context.Background(), []llm.Message{{
		Role: llm.RoleAssistant, Content: llm.TextContent("old context"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != fallbackTitle || len(provider.Requests()) != 1 {
		t.Fatalf("result=%+v calls=%d", result, len(provider.Requests()))
	}
}

func TestCompactorSkipsModelForEmptyCompressionRegion(t *testing.T) {
	provider := &llmtest.FakeProvider{Caps: llm.Capabilities{ContextWindowTokens: 65536}}
	result, err := New(provider).Compact(context.Background(), []llm.Message{{
		Role:    llm.RoleUser,
		Content: llm.TextContent("<run_user_instruction run_id=\"r1\">keep</run_user_instruction>"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != fallbackTitle || result.Summary != emptySummary() || len(provider.Requests()) != 0 {
		t.Fatalf("unexpected empty result: %+v calls=%d", result, len(provider.Requests()))
	}
}

func TestCompactRequestTokensIncludeLocalToolSchema(t *testing.T) {
	withoutTool := contextengine.EstimateTextTokens(prompts.MustLoad("command.compact").Body) + 16
	if got := compactRequestTokens(nil); got <= withoutTool {
		t.Fatalf("request tokens=%d without tool=%d", got, withoutTool)
	}
}
