package service

import (
	"context"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestManualContextCompactRewritesTranscriptAndPersistsEvent(t *testing.T) {
	fixture := newBriefingFixture(t,
		"## 目标与意图\n继续任务\n\n## 已完成改动\n已读取页面\n\n## 关键决策\n保持设计\n\n## 未决问题\n无\n\n## 下一步\n继续",
	)
	fixture.provider.Caps = llm.Capabilities{ContextWindowTokens: 65536}
	transcripts := contextengine.NewFSTranscriptStore()
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent(`<run_user_instruction run_id="r1">first</run_user_instruction>`)},
		{Role: llm.RoleAssistant, Content: llm.TextContent(strings.Repeat("analysis ", 100))},
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
		result.Compaction.Summary == "" ||
		result.Snapshot.Status != "idle" {
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
	if err != nil || len(records) != 1 {
		t.Fatalf("compaction record missing: records=%+v err=%v", records, err)
	}
}

func TestManualContextCompactRespectsProjectLock(t *testing.T) {
	fixture := newBriefingFixture(t, "unused")
	fixture.provider.Caps = llm.Capabilities{ContextWindowTokens: 65536}
	release, acquired := fixture.locks.TryAcquire(fixture.project.ID)
	if !acquired {
		t.Fatal("failed to acquire fixture lock")
	}
	defer release()
	_, err := NewContextWindowService(
		fixture.store, fixture.registry, fixture.locks,
		contextengine.NewFSTranscriptStore(), contextengine.NewCalibrationStore(),
	).Compact(context.Background(), fixture.thread.ID, "Briefing")
	agentErr := model.AsAgentError(err, "INTERNAL", "test")
	if agentErr.Code != "COMPACT_ACTIVE" {
		t.Fatalf("error=%v", err)
	}
}

func TestReplaceTranscriptSnapshotPreservesFixedDetailsWithoutLayerLabels(t *testing.T) {
	fixed := contextengine.WindowBucketDetail{Name: "system policy", Source: "system_prompt", Tokens: 20}
	oldHistory := contextengine.WindowBucketDetail{Name: "assistant message 1", Source: "assistant", Tokens: 30}
	newHistory := contextengine.WindowBucketDetail{Name: "user message 1", Source: "user", Tokens: 10}
	base := contextengine.WindowSnapshot{
		Total: 50, Max: 100, Ratio: 0.5,
		Buckets: map[contextengine.ContextBucket]int{
			contextengine.BucketSystemPrompt: 20,
			contextengine.BucketChatHistory:  30,
		},
		Details: map[contextengine.ContextBucket][]contextengine.WindowBucketDetail{
			contextengine.BucketSystemPrompt: {fixed},
			contextengine.BucketChatHistory:  {oldHistory},
		},
	}
	before := contextengine.WindowSnapshot{
		Total: 30, Max: 100,
		Buckets: map[contextengine.ContextBucket]int{contextengine.BucketChatHistory: 30},
		Details: map[contextengine.ContextBucket][]contextengine.WindowBucketDetail{
			contextengine.BucketChatHistory: {oldHistory},
		},
	}
	after := contextengine.WindowSnapshot{
		Total: 10, Max: 100,
		Buckets: map[contextengine.ContextBucket]int{contextengine.BucketUserPrompt: 10},
		Details: map[contextengine.ContextBucket][]contextengine.WindowBucketDetail{
			contextengine.BucketUserPrompt: {newHistory},
		},
	}

	got := replaceTranscriptSnapshot(base, before, after)
	if got.Total != 30 || got.Ratio != 0.3 || got.Buckets[contextengine.BucketSystemPrompt] != 20 ||
		got.Buckets[contextengine.BucketChatHistory] != 0 || got.Buckets[contextengine.BucketUserPrompt] != 10 {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
	if len(got.Details[contextengine.BucketSystemPrompt]) != 1 || got.Details[contextengine.BucketSystemPrompt][0] != fixed ||
		len(got.Details[contextengine.BucketUserPrompt]) != 1 || got.Details[contextengine.BucketUserPrompt][0] != newHistory {
		t.Fatalf("unexpected details: %+v", got.Details)
	}
}
