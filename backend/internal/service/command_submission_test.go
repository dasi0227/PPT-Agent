package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestPolishCorrectionPreservesDraftAndAppliesOnlyValidResult(t *testing.T) {
	f := newBriefingFixture(t)
	f.provider.Script = []llm.GenerateResponse{
		{ToolCalls: []llm.ToolCall{{ID: "bad", Name: "polish_instruction", Args: map[string]any{"title": "修改表达"}}}},
		{ToolCalls: []llm.ToolCall{{ID: "valid", Name: "polish_instruction", Args: map[string]any{"title": "原文清晰，保持不变", "content": "你好"}}}},
	}
	result, err := NewPolishService(f.store, f.registry).Polish(context.Background(), f.project.ID, PolishParams{Instruction: "您好", PolishContent: "你好", ThreadID: f.thread.ID})
	if err != nil || result.Content != "你好" || result.Changed {
		t.Fatalf("result=%v err=%v", result, err)
	}
	requests := f.provider.Requests()
	want := "<user_draft>\n您好\n</user_draft>\n\n<last_revision>\n你好\n</last_revision>\n<revision_feedback>\n\n</revision_feedback>\n\n<user_instruction>\nProduce the polished text, applying any supplied revision feedback.\n</user_instruction>"
	if len(requests) != 2 || requests[0].Messages[1].Text() != want || requests[0].Messages[1].Text() != requests[1].Messages[1].Text() || requests[1].Messages[3].ToolCallID != "bad" || len(requests[0].Tools) != 1 || requests[0].Tools[0].Name != "polish_instruction" {
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
	accepted, err := commands.Accept(context.Background(), f.thread.ID, request, 1)
	if err != nil {
		t.Fatal(err)
	}
	completed := waitCommand(t, f.store, accepted.Command.CommandID)
	if completed.Status != "completed" || len(f.provider.Requests()) != 2 {
		t.Fatalf("commit=%v requests=%d", completed, len(f.provider.Requests()))
	}
	var intent gitcommit.CommitIntent
	if err := json.Unmarshal(completed.Operation, &intent); err != nil || intent.AttemptID != completed.AttemptID || intent.Tree == "" {
		t.Fatalf("missing intent: %+v %v", intent, err)
	}
	replay, err := commands.Accept(context.Background(), f.thread.ID, request, 1)
	if err != nil || replay.Command.CommandID != accepted.Command.CommandID || len(f.provider.Requests()) != 2 || replay.Receipt.Seq != accepted.Receipt.Seq {
		t.Fatal("accepted command re-executed instead of returning its receipt")
	}
}
