package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
	calibration := contextengine.NewCalibrationStore()
	service := NewContextWindowService(
		fixture.store, fixture.registry, fixture.locks, transcripts, calibration,
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
	if len(loaded) != 2 || !strings.Contains(loaded[0].Text(), "<context_summary>") {
		t.Fatalf("transcript was not destructively replaced: %+v", loaded)
	}
	records, err := fixture.store.ListThreadContextCompactions(context.Background(), fixture.thread.ID)
	if err != nil || len(records) != 1 || records[0].Title != result.Compaction.Title {
		t.Fatalf("compaction record missing: records=%+v err=%v", records, err)
	}
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
		transcripts, contextengine.NewCalibrationStore(),
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
		contextengine.NewJournalTranscriptStore(fixture.store), contextengine.NewCalibrationStore(),
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
		context.Background(),
		contextcompact.Result{Title: "收敛自动压缩结果", Content: "## 目标与意图\n继续"},
		testWindowSnapshot(1000, map[contextengine.ContextBucket]map[string]int{
			contextengine.BucketChatHistory: {"assistant messages": 600},
		}),
		testWindowSnapshot(1000, map[contextengine.ContextBucket]map[string]int{
			contextengine.BucketChatHistory: {"context summary": 200},
		}),
		2*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	if compaction.Title != "收敛自动压缩结果" || compaction.Trigger != model.ContextCompactionAuto {
		t.Fatalf("unexpected auto compaction: %+v", compaction)
	}
	records, err := fixture.store.ListThreadContextCompactions(context.Background(), fixture.thread.ID)
	if err != nil || len(records) != 1 || records[0].Title != compaction.Title {
		t.Fatalf("auto compaction was not persisted: records=%+v err=%v", records, err)
	}
}

func TestReplaceTranscriptSnapshotPreservesFixedDetailsWithoutLayerLabels(t *testing.T) {
	base := testWindowSnapshot(100, map[contextengine.ContextBucket]map[string]int{
		contextengine.BucketSystemPrompt: {"system prompts": 20},
		contextengine.BucketChatHistory:  {"assistant messages": 30},
	})
	before := testWindowSnapshot(100, map[contextengine.ContextBucket]map[string]int{
		contextengine.BucketChatHistory: {"assistant messages": 30},
	})
	after := testWindowSnapshot(100, map[contextengine.ContextBucket]map[string]int{
		contextengine.BucketChatHistory: {"user messages": 10},
	})

	got := replaceTranscriptSnapshot(base, before, after)
	if got.Total != 30 || got.Ratio != 0.3 || got.Buckets[contextengine.BucketSystemPrompt] != 20 ||
		got.Buckets[contextengine.BucketChatHistory] != 10 {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
	if detailTokens(got.Details[contextengine.BucketSystemPrompt], "system prompts") != 20 ||
		detailTokens(got.Details[contextengine.BucketChatHistory], "assistant messages") != 0 ||
		detailTokens(got.Details[contextengine.BucketChatHistory], "user messages") != 10 {
		t.Fatalf("unexpected details: %+v", got.Details)
	}
}

func TestReplaceTranscriptSnapshotReRanksDynamicCommandDetails(t *testing.T) {
	base := testWindowSnapshot(1000, map[contextengine.ContextBucket]map[string]int{
		contextengine.BucketRunCommand: {
			"ls": 100, "rg": 80, "git": 60, "other command": 40,
		},
	})
	before := testWindowSnapshot(1000, map[contextengine.ContextBucket]map[string]int{
		contextengine.BucketRunCommand: {"rg": 80, "other command": 40},
	})
	after := testWindowSnapshot(1000, map[contextengine.ContextBucket]map[string]int{
		contextengine.BucketRunCommand: {"pwd": 20},
	})

	got := replaceTranscriptSnapshot(base, before, after)
	details := got.Details[contextengine.BucketRunCommand]
	if len(details) != 3 || details[0].Name != "ls" || details[0].Tokens != 100 ||
		details[1].Name != "git" || details[1].Tokens != 60 ||
		details[2].Name != "pwd" || details[2].Tokens != 20 ||
		got.Buckets[contextengine.BucketRunCommand] != 180 {
		t.Fatalf("unexpected dynamic command replacement: details=%+v bucket=%d", details, got.Buckets[contextengine.BucketRunCommand])
	}
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
		if bucket == contextengine.BucketRunCommand {
			candidates := make([]contextengine.WindowBucketDetail, 0, len(values[bucket]))
			for name, tokens := range values[bucket] {
				candidates = append(candidates, contextengine.WindowBucketDetail{Name: name, Tokens: tokens})
			}
			snapshot.Details[bucket] = contextengine.NormalizeWindowDetails(bucket, candidates)
		} else {
			for _, name := range contextengine.ContextWindowDetailNames(bucket) {
				snapshot.Details[bucket] = append(snapshot.Details[bucket], contextengine.WindowBucketDetail{
					Name: name, Tokens: values[bucket][name],
				})
			}
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
