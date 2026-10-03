package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestPolishCorrectionPreservesDraftAndAppliesOnlyValidResult(t *testing.T) {
	f := newBriefingFixture(t)
	f.provider.Script = []llm.GenerateResponse{
		{ToolCalls: []llm.ToolCall{{ID: "bad", Name: "polish_instruction", Args: map[string]any{"title": "修改表达"}}}},
		{ToolCalls: []llm.ToolCall{{ID: "valid", Name: "polish_instruction", Args: map[string]any{"title": "原文清晰，保持不变", "content": "你好"}}}},
	}
	result, err := NewPolishService(f.store, f.registry).Polish(context.Background(), f.project.ID, PolishParams{Instruction: "你好", Feedback: "保持原意", ThreadID: f.thread.ID, Mode: model.ModeChat, ScopeInput: model.CreateRunScopeInput{Selection: model.ScopeSelectionInput{Kind: model.ScopeAllPages}}})
	if err != nil || result.Content != "你好" || result.Changed {
		t.Fatalf("result=%v err=%v", result, err)
	}
	requests := f.provider.Requests()
	if len(requests) != 2 || requests[0].Messages[1].Text() != requests[1].Messages[1].Text() || requests[1].Messages[3].ToolCallID != "bad" || requests[0].RequiredTool != "polish_instruction" {
		t.Fatal("polish correction lost its original input or failed call pairing")
	}
}

func TestCommitCorrectionCreatesOneIntentAndReusesReceipt(t *testing.T) {
	f := newBriefingFixture(t)
	f.provider.Caps = llm.Capabilities{ToolCalls: true}
	f.provider.Script = []llm.GenerateResponse{
		{Content: llm.TextContent("not submitted")},
		{ToolCalls: []llm.ToolCall{{ID: "valid", Name: "git_commit", Args: map[string]any{"title": "feat: 保存演示内容", "items": []any{"保存项目源文件"}}}}},
	}
	git := NewGitCommitService(f.store, f.registry, f.locks)
	commands := NewCommandService(f.store, git.ExecuteCommand)
	request := model.CommandRequest{RequestKey: "corrected-commit", Kind: "commit", Input: json.RawMessage(`{}`)}
	accepted, err := commands.Accept(context.Background(), f.thread.ID, request, 0)
	if err != nil {
		t.Fatal(err)
	}
	completed := waitCommand(t, f.store, accepted.CommandID)
	if completed.Status != "completed" || len(f.provider.Requests()) != 2 {
		t.Fatalf("commit=%v requests=%d", completed, len(f.provider.Requests()))
	}
	events, err := f.store.ThreadEvents(context.Background(), f.thread.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	intents := 0
	for _, event := range events {
		if event.Type == "commit.intent" {
			intents++
		}
	}
	if intents != 1 {
		t.Fatalf("Git was applied repeatedly: %d intents", intents)
	}
	replay, err := commands.Accept(context.Background(), f.thread.ID, request, 0)
	if err != nil || replay.CommandID != accepted.CommandID || len(f.provider.Requests()) != 2 {
		t.Fatal("accepted command re-executed instead of returning its receipt")
	}
}
