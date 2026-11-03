package service

import (
	"context"
	"encoding/json"
	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitCommand(t *testing.T, s CommandStore, id string) model.CommandExecution {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		value, err := s.GetCommand(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		switch value.Status {
		case "accepted", "running", "cancel_requested":
			time.Sleep(time.Millisecond)
		default:
			return value
		}
	}
	t.Fatal("command did not finish")
	return model.CommandExecution{}
}

func TestGitCommitUsesUnifiedCommandAndInterruptsUnstartedAttempt(t *testing.T) {
	fixture := newBriefingFixture(t)
	fixture.provider.Caps = llm.Capabilities{ToolCalls: true}
	fixture.provider.Script = []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{ID: "commit", Name: "git_commit", Args: map[string]any{"title": "feat: create deck", "items": []any{"Record initial content"}}}}}}
	git := NewGitCommitService(fixture.store, fixture.registry, fixture.locks)
	commands := NewCommandService(fixture.store, git.ExecuteCommand)
	accepted, err := commands.Accept(context.Background(), "t1", model.CommandRequest{RequestKey: "commit", Kind: "commit", Input: json.RawMessage(`{}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	result := waitCommand(t, fixture.store, accepted.Command.CommandID)
	if result.Status != "completed" || result.Source != "user" || result.RunID != "" {
		t.Fatalf("manual commit failed or attached to an Agent run: %+v", result)
	}
	requests := fixture.provider.Requests()
	if len(requests) != 1 || len(requests[0].Tools) != 1 || requests[0].Tools[0].Name != "git_commit" || requests[0].RequiredTool != "git_commit" {
		t.Fatalf("manual commit lost its independent submission tool: %+v", requests)
	}
	var commit model.GitCommitResult
	if err := json.Unmarshal(result.Result, &commit); err != nil || commit.Hash == "" {
		t.Fatalf("result=%s err=%v", result.Result, err)
	}
	var intent gitcommit.CommitIntent
	if err := json.Unmarshal(result.Operation, &intent); err != nil || intent.AttemptID != accepted.Command.AttemptID || intent.Tree == "" {
		t.Fatalf("missing durable Git intent: %+v %v", intent, err)
	}
	empty, err := commands.Accept(context.Background(), "t1", model.CommandRequest{RequestKey: "empty", Kind: "commit", Input: json.RawMessage(`{}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	emptyResult := waitCommand(t, fixture.store, empty.Command.CommandID)
	if emptyResult.Status != "completed" || string(emptyResult.Result) != `{"empty":true}` {
		t.Fatalf("empty result: %+v", emptyResult)
	}
	if len(fixture.provider.Requests()) != 1 {
		t.Fatal("empty commit called model")
	}
	project, err := fixture.store.GetProject(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project.WorkDir, "sli_extra.html"), []byte("next"), 0600); err != nil {
		t.Fatal(err)
	}
	pending, _, err := fixture.store.AcceptCommand(context.Background(), "t1", model.CommandRequest{RequestKey: "interrupted", Kind: "commit", Input: json.RawMessage(`{}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	recovered, err := commands.Get(context.Background(), pending.Command.CommandID)
	if err != nil || recovered.Status != "interrupted" {
		t.Fatalf("restart: %+v %v", recovered, err)
	}
	if len(fixture.provider.Requests()) != 1 {
		t.Fatal("restart repeated generation")
	}
}
