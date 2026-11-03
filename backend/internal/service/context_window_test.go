package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

const compactTestSummary = "## 目标与约束\n继续任务\n\n## 工作进展\n已读取页面\n\n## 关键决策\n保持设计\n\n## 未决事项\n无\n\n## 下一步\n继续"

func TestManualContextCompactCommitsContextResultAndCommandTogether(t *testing.T) {
	fixture := newBriefingFixture(t)
	fixture.provider.Caps = llm.Capabilities{ToolCalls: true, ContextWindowTokens: 65536}
	fixture.provider.Script = []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{
		ID: "compact-1", Name: "compact_context", Args: map[string]any{
			"title":   "收敛上下文协议与实现",
			"content": compactTestSummary,
		},
	}}}}
	transcripts := contextengine.NewContextStore(fixture.store)
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("first"), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: "r1"}},
		{Role: llm.RoleAssistant, Content: llm.TextContent(strings.Repeat("analysis ", 16_000))},
	}
	seedContext(t, fixture, messages)

	service := NewContextWindowService(
		fixture.store, fixture.registry, fixture.locks, transcripts, contextWindowRuns(fixture),
	)
	result, err := service.Compact(context.Background(), fixture.thread.ID, "Briefing")
	if err != nil {
		t.Fatal(err)
	}
	if result.Compaction.Trigger != model.ContextCompactionManual ||
		result.Compaction.Title != "收敛上下文协议与实现" ||
		result.Compaction.Content == "" ||
		result.Snapshot.Status != "idle" ||
		result.Snapshot.CompactThresholdTokens != contextcompact.MinimumCompactableTokens {
		t.Fatalf("unexpected compact result: %+v", result)
	}
	before, err := transcripts.Open(context.Background(), fixture.thread.ID, nil)
	if err != nil || before.Snapshot.Revision != 1 {
		t.Fatalf("preparation changed context: %+v %v", before, err)
	}
	accepted, _, err := fixture.store.AcceptCommand(context.Background(), fixture.thread.ID, model.CommandRequest{RequestKey: "compact", Kind: "compact", Input: json.RawMessage(`{}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	command := accepted.Command
	command.Status = "completed"
	command.Result, _ = json.Marshal(result)
	if _, err := fixture.store.CompleteCommand(context.Background(), command, sessionstate.CommandEffects{Context: result.PendingContext, Compaction: &result.Compaction}); err != nil {
		t.Fatal(err)
	}
	loaded, err := transcripts.Open(context.Background(), fixture.thread.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	current := loaded.Messages()
	if loaded.Snapshot.Revision != 2 || len(current) != 2 || !strings.Contains(current[0].Text(), "<compact_summary>") || current[1].Text() != messages[0].Text() {
		t.Fatalf("unexpected committed context: %+v", current)
	}
	var stored struct {
		BeforeRevision, AfterRevision int64
		ResultJSON                    string
	}
	if err := fixture.db.Table("context_compactions").Where("id = ?", result.Compaction.ID).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.BeforeRevision != 1 || stored.AfterRevision != 2 || !strings.Contains(stored.ResultJSON, "收敛上下文协议与实现") {
		t.Fatalf("compaction was not committed with context: %+v", stored)
	}
}

func TestManualContextCompactRejectsTranscriptBelowThreshold(t *testing.T) {
	fixture := newBriefingFixture(t, "unused")
	fixture.provider.Caps = llm.Capabilities{ToolCalls: true, ContextWindowTokens: 65536}
	transcripts := contextengine.NewContextStore(fixture.store)
	seedContext(t, fixture, []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("short request")}, {Role: llm.RoleAssistant, Content: llm.TextContent("short response")}})

	_, err := NewContextWindowService(
		fixture.store, fixture.registry, fixture.locks,
		transcripts, contextWindowRuns(fixture),
	).Compact(context.Background(), fixture.thread.ID, "Briefing")
	agentErr := model.AsAgentError(err, "INTERNAL", "test")
	if agentErr.Code != "COMPACT_BELOW_THRESHOLD" {
		t.Fatalf("error=%v", err)
	}
	if len(fixture.provider.Requests()) != 0 {
		t.Fatalf("compactor model was called below threshold: %d requests", len(fixture.provider.Requests()))
	}
}

func TestManualContextCompactRespectsProjectLock(t *testing.T) {
	fixture := newBriefingFixture(t, "unused")
	fixture.provider.Caps = llm.Capabilities{ToolCalls: true, ContextWindowTokens: 65536}
	release, acquired := fixture.locks.TryAcquire(fixture.project.ID)
	if !acquired {
		t.Fatal("failed to acquire fixture lock")
	}
	defer release()
	_, err := NewContextWindowService(
		fixture.store, fixture.registry, fixture.locks,
		contextengine.NewContextStore(fixture.store), contextWindowRuns(fixture),
	).Compact(context.Background(), fixture.thread.ID, "Briefing")
	agentErr := model.AsAgentError(err, "INTERNAL", "test")
	if agentErr.Code != "COMPACT_ACTIVE" {
		t.Fatalf("error=%v", err)
	}
}

// Fixture setup uses the same command transaction as idle compaction.
func seedContext(t *testing.T, f briefingFixture, messages []llm.Message) {
	t.Helper()
	source, err := contextengine.NewContextStore(f.store).Open(context.Background(), f.thread.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := source.Prepare(messages, "fixture-source", "replace")
	if err != nil {
		t.Fatal(err)
	}
	commitContextForTest(t, f, prepared.Update)
}

func contextWindowRuns(fixture briefingFixture) *RunService {
	root := WorkRoot(filepath.Dir(filepath.Dir(model.ProjectRoot(fixture.project.WorkDir))))
	return NewRunService(fixture.store, nil, fixture.registry, root, nil, nil)
}

func TestContextWindowInspectorReadsCurrentContextWithoutStaleCache(t *testing.T) {
	fixture := newBriefingFixture(t)
	fixture.provider.Caps = llm.Capabilities{ToolCalls: true, ContextWindowTokens: 65536}
	transcripts := contextengine.NewContextStore(fixture.store)
	svc := NewContextWindowService(fixture.store, fixture.registry, fixture.locks, transcripts, contextWindowRuns(fixture))
	before, err := svc.Snapshot(context.Background(), fixture.thread.ID, "Briefing")
	if err != nil {
		t.Fatal(err)
	}
	if before.Buckets[contextengine.BucketSystemPrompt] == 0 || before.Buckets[contextengine.BucketRuntime] == 0 {
		t.Fatal("first snapshot only counted history")
	}
	seedContext(t, fixture, []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent(strings.Repeat("new history ", 100))}})
	retained, err := svc.Snapshot(context.Background(), fixture.thread.ID, "Briefing")
	if err != nil || retained.Total <= before.Total {
		t.Fatal("inspector reused a stale estimate", err)
	}
	restarted := NewContextWindowService(fixture.store, fixture.registry, fixture.locks, transcripts, contextWindowRuns(fixture))
	after, err := restarted.Snapshot(context.Background(), fixture.thread.ID, "Briefing")
	if err != nil {
		t.Fatal(err)
	}
	if after.Total != retained.Total || after.Buckets[contextengine.BucketSystemPrompt] != before.Buckets[contextengine.BucketSystemPrompt] {
		t.Fatal("restart did not rebuild stable system and current history")
	}
	if len(fixture.provider.Requests()) != 0 {
		t.Fatal("snapshot reconstruction invoked the model")
	}
}
