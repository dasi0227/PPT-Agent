package contextcompact

import (
	"context"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
)

func TestCompactorUsesOneCallAndRetainsUsersAndRecentToolRounds(t *testing.T) {
	provider := &llmtest.FakeProvider{
		Caps: llm.Capabilities{ContextWindowTokens: 65536},
		Script: []llm.GenerateResponse{{Content: llm.TextContent(
			"## 目标与意图\n目标\n\n## 已完成改动\n完成\n\n## 关键决策\n决策\n\n## 未决问题\n问题\n\n## 下一步\n继续",
		)}},
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
	if len(result.Messages) != 6 || !strings.Contains(result.Messages[0].Text(), "<context_summary>") {
		t.Fatalf("unexpected replacement: %+v", result.Messages)
	}
	if !strings.Contains(result.Messages[1].Text(), "first instruction") {
		t.Fatalf("user instruction was not retained: %+v", result.Messages)
	}
}

func TestCompactorTruncatesOldestInputWithoutRecursion(t *testing.T) {
	provider := &llmtest.FakeProvider{
		Caps: llm.Capabilities{ContextWindowTokens: 5200},
		Script: []llm.GenerateResponse{{Content: llm.TextContent(
			"## 目标与意图\n目标\n## 已完成改动\n无\n## 关键决策\n无\n## 未决问题\n无\n## 下一步\n继续",
		)}},
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
