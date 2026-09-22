package commandresult

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func TestTextResultContract(t *testing.T) {
	response := func(name string, args map[string]any) llm.GenerateResponse {
		return llm.GenerateResponse{ToolCalls: []llm.ToolCall{{Name: name, Args: args}}}
	}
	for _, name := range []string{"kickoff_thread", "handoff_thread", "polish_instruction", "compact_context"} {
		t.Run(name, func(t *testing.T) {
			got, err := Parse(response(name, map[string]any{"title": " 明确任务目标 ", "content": "\n## 任务\n保留正文\n"}), name, 100)
			if err != nil || got.Title != "明确任务目标" || got.Content != "## 任务\n保留正文" {
				t.Fatalf("title and content were not preserved separately: %+v, %v", got, err)
			}
		})
	}
	name := "kickoff_thread"
	valid := map[string]any{"title": "明确任务目标", "content": "完整正文"}
	tests := map[string]llm.GenerateResponse{
		"plain text":        {Content: llm.TextContent("# 标题\n正文")},
		"wrong tool":        response("handoff_thread", valid),
		"multiple calls":    {ToolCalls: []llm.ToolCall{{Name: name, Args: valid}, {Name: name, Args: valid}}},
		"missing title":     response(name, map[string]any{"content": "正文"}),
		"old summary field": response(name, map[string]any{"title": "标题", "summary": "正文"}),
		"extra field":       response(name, map[string]any{"title": "标题", "content": "正文", "extra": true}),
		"blank title":       response(name, map[string]any{"title": " ", "content": "正文"}),
		"multiline title":   response(name, map[string]any{"title": "标题\n第二行", "content": "正文"}),
		"heading title":     response(name, map[string]any{"title": "# 标题", "content": "正文"}),
		"long title":        response(name, map[string]any{"title": strings.Repeat("字", MaxTitleRunes+1), "content": "正文"}),
		"blank content":     response(name, map[string]any{"title": "标题", "content": " \n"}),
		"wrong type":        response(name, map[string]any{"title": "标题", "content": 1}),
		"long content":      response(name, map[string]any{"title": "标题", "content": strings.Repeat("字", 101)}),
		"text beside call":  {Content: llm.TextContent("额外说明"), ToolCalls: []llm.ToolCall{{Name: name, Args: valid}}},
	}
	for label, input := range tests {
		t.Run(label, func(t *testing.T) {
			if _, err := Parse(input, name, 100); err == nil {
				t.Fatal("invalid model output was accepted")
			}
		})
	}
}
