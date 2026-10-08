package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

func TestManualContextCompactRewritesTranscriptAndPersistsEvent(t *testing.T) {
	fixture := newBriefingFixture(t,
		"## 目标与意图\n继续任务\n\n## 已完成改动\n已读取页面\n\n## 关键决策\n保持设计\n\n## 未决问题\n无\n\n## 下一步\n继续",
	)
	fixture.provider.Caps = llm.Capabilities{ToolCalls: true, ContextWindowTokens: 65536}
	fixture.provider.Script = []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{
		ID: "compact-1", Name: "compact_context", Args: map[string]any{
			"title":   "收敛上下文协议与实现",
			"content": "## 目标与意图\n继续任务\n\n## 已完成改动\n已读取页面\n\n## 关键决策\n保持设计\n\n## 未决问题\n无\n\n## 下一步\n继续",
		},
	}}}}
	transcripts := contextengine.NewJournalTranscriptStore(fixture.store)
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent(`<run_user_instruction run_id="r1">first</run_user_instruction>`)},
		{Role: llm.RoleAssistant, Content: llm.TextContent(strings.Repeat("analysis ", 5_000))},
	}
	if err := transcripts.Replace(fixture.project.WorkDir, fixture.thread.ID, messages); err != nil {
		t.Fatal(err)
	}
	windows := contextengine.NewWindowStore()
	service := NewContextWindowService(
		fixture.store, fixture.registry, fixture.locks, transcripts, windows, contextWindowRuns(fixture),
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
	loaded, err := transcripts.Load(fixture.project.WorkDir, fixture.thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || !strings.Contains(loaded[0].Text(), "<context_summary>") {
		t.Fatalf("transcript was not destructively replaced: %+v", loaded)
	}
	assertCompactionJournal(t, fixture.store, result.Compaction)
}

func TestManualContextCompactRejectsTranscriptBelowThreshold(t *testing.T) {
	fixture := newBriefingFixture(t, "unused")
	fixture.provider.Caps = llm.Capabilities{ToolCalls: true, ContextWindowTokens: 65536}
	transcripts := contextengine.NewJournalTranscriptStore(fixture.store)
	if err := transcripts.Replace(fixture.project.WorkDir, fixture.thread.ID, []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("short request")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("short response")},
	}); err != nil {
		t.Fatal(err)
	}

	_, err := NewContextWindowService(
		fixture.store, fixture.registry, fixture.locks,
		transcripts, contextengine.NewWindowStore(), contextWindowRuns(fixture),
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
		contextengine.NewJournalTranscriptStore(fixture.store), contextengine.NewWindowStore(), contextWindowRuns(fixture),
	).Compact(context.Background(), fixture.thread.ID, "Briefing")
	agentErr := model.AsAgentError(err, "INTERNAL", "test")
	if agentErr.Code != "COMPACT_ACTIVE" {
		t.Fatalf("error=%v", err)
	}
}

func TestAutoContextCompactPersistsGeneratedTitle(t *testing.T) {
	fixture := newBriefingFixture(t)
	execution := &workflowExecution{
		pack:    contextengine.ContextPack{Manifest: contextengine.ContextManifest{ThreadID: fixture.thread.ID}},
		project: fixture.project,
		store:   fixture.store,
		runID:   "run_auto",
	}
	compaction, err := execution.recordAutoCompaction(
		context.Background(), "cmp_progress",
		contextcompact.Result{Title: "收敛自动压缩结果", Content: "## 目标与意图\n继续"},
		testWindowSnapshot(1000, map[contextengine.ContextBucket]map[string]int{
			contextengine.BucketChatHistory: {"assistant messages": 600},
		}),
		testWindowSnapshot(1000, map[contextengine.ContextBucket]map[string]int{
			contextengine.BucketRuntime: {"runtime messages": 200},
		}),
		2*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	if compaction.ID != "cmp_progress" || compaction.RunID != "run_auto" || compaction.Title != "收敛自动压缩结果" || compaction.Trigger != model.ContextCompactionAuto {
		t.Fatalf("unexpected auto compaction: %+v", compaction)
	}
	assertCompactionJournal(t, fixture.store, compaction)
}

func testWindowSnapshot(
	max int,
	values map[contextengine.ContextBucket]map[string]int,
) contextengine.WindowSnapshot {
	snapshot := contextengine.WindowSnapshot{
		Max:     max,
		Buckets: map[contextengine.ContextBucket]int{},
		Details: map[contextengine.ContextBucket][]contextengine.WindowBucketDetail{},
	}
	for _, bucket := range contextengine.ContextBuckets {
		for _, name := range contextengine.ContextWindowDetailNames(bucket) {
			snapshot.Details[bucket] = append(snapshot.Details[bucket], contextengine.WindowBucketDetail{Name: name, Tokens: values[bucket][name]})
		}
		for _, detail := range snapshot.Details[bucket] {
			snapshot.Buckets[bucket] += detail.Tokens
		}
		snapshot.Total += snapshot.Buckets[bucket]
	}
	if max > 0 {
		snapshot.Ratio = float64(snapshot.Total) / float64(max)
	}
	return snapshot
}

func assertCompactionJournal(t *testing.T, backend threadjournal.Backend, want model.ContextCompaction) {
	t.Helper()
	events, err := backend.ThreadEvents(context.Background(), want.ThreadID, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range events {
		if event.Type != "context.compaction_result" {
			continue
		}
		count++
		var got model.ContextCompaction
		if err := json.Unmarshal(event.Payload, &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != want.ID || got.Title != want.Title {
			t.Fatalf("compaction=%+v want=%+v", got, want)
		}
	}
	if count != 1 {
		t.Fatalf("compaction event count=%d", count)
	}
}

func contextWindowRuns(fixture briefingFixture) *RunService {
	root := WorkRoot(filepath.Dir(filepath.Dir(model.ProjectRoot(fixture.project.WorkDir))))
	return NewRunService(fixture.store, nil, fixture.registry, root, nil, nil, nil)
}

func TestContextWindowFirstReadRebuildsFullContextAndThenKeepsSnapshot(t *testing.T) {
	fixture := newBriefingFixture(t)
	fixture.provider.Caps = llm.Capabilities{ToolCalls: true, ContextWindowTokens: 65536}
	transcripts := contextengine.NewJournalTranscriptStore(fixture.store)
	windows := contextengine.NewWindowStore()
	svc := NewContextWindowService(fixture.store, fixture.registry, fixture.locks, transcripts, windows, contextWindowRuns(fixture))
	before, err := svc.Snapshot(context.Background(), fixture.thread.ID, "Briefing")
	if err != nil {
		t.Fatal(err)
	}
	if before.Buckets[contextengine.BucketSystemPrompt] == 0 || before.Buckets[contextengine.BucketRuntime] == 0 {
		t.Fatal("first snapshot only counted history")
	}
	if err := transcripts.Replace(fixture.project.WorkDir, fixture.thread.ID, []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent(strings.Repeat("new history ", 100))}}); err != nil {
		t.Fatal(err)
	}
	retained, err := svc.Snapshot(context.Background(), fixture.thread.ID, "Briefing")
	if err != nil || retained.Total != before.Total {
		t.Fatal("snapshot changed outside an update boundary", err)
	}
	restarted := NewContextWindowService(fixture.store, fixture.registry, fixture.locks, transcripts, contextengine.NewWindowStore(), contextWindowRuns(fixture))
	after, err := restarted.Snapshot(context.Background(), fixture.thread.ID, "Briefing")
	if err != nil {
		t.Fatal(err)
	}
	if after.Total <= before.Total || after.Buckets[contextengine.BucketSystemPrompt] != before.Buckets[contextengine.BucketSystemPrompt] {
		t.Fatal("restart did not rebuild stable system and current history")
	}
	if len(fixture.provider.Requests()) != 0 {
		t.Fatal("snapshot reconstruction invoked the model")
	}
}
