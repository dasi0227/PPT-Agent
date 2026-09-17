package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

type ContextWindowSnapshot struct {
	contextengine.WindowSnapshot
	Status string `json:"status"`
}

type CompactContextResult struct {
	Snapshot   ContextWindowSnapshot   `json:"snapshot"`
	Compaction model.ContextCompaction `json:"compaction"`
}

type ContextWindowService struct {
	store       store.Store
	registry    *llm.Registry
	locks       *run.LockManager
	transcripts *contextengine.FSTranscriptStore
	calibration *contextengine.CalibrationStore
}

func NewContextWindowService(
	s store.Store,
	registry *llm.Registry,
	locks *run.LockManager,
	transcripts *contextengine.FSTranscriptStore,
	calibration *contextengine.CalibrationStore,
) *ContextWindowService {
	return &ContextWindowService{
		store: s, registry: registry, locks: locks,
		transcripts: transcripts, calibration: calibration,
	}
}

func (svc *ContextWindowService) Snapshot(
	ctx context.Context,
	threadID string,
	modelProfile string,
) (ContextWindowSnapshot, error) {
	thread, project, profile, messages, err := svc.load(ctx, threadID, modelProfile)
	if err != nil {
		return ContextWindowSnapshot{}, err
	}
	_ = project
	if snapshot, ok := svc.calibration.Snapshot(thread.ID); ok &&
		snapshot.Max == profile.Capabilities().ContextWindowTokens {
		return ContextWindowSnapshot{WindowSnapshot: snapshot, Status: "idle"}, nil
	}
	if snapshot, ok := loadPersistedWindowSnapshot(ctx, svc.store, thread.ID); ok &&
		snapshot.Max == profile.Capabilities().ContextWindowTokens {
		svc.calibration.SetSnapshot(thread.ID, snapshot)
		return ContextWindowSnapshot{WindowSnapshot: snapshot, Status: "idle"}, nil
	}
	snapshot := svc.transcriptOnlySnapshot(thread.ID, profile, messages)
	svc.calibration.SetSnapshot(thread.ID, snapshot)
	return ContextWindowSnapshot{WindowSnapshot: snapshot, Status: "idle"}, nil
}

type contextWindowSnapshotReader interface {
	LatestThreadContextWindow(context.Context, string) (string, error)
}

func loadPersistedWindowSnapshot(ctx context.Context, value any, threadID string) (contextengine.WindowSnapshot, bool) {
	reader, ok := value.(contextWindowSnapshotReader)
	if !ok {
		return contextengine.WindowSnapshot{}, false
	}
	raw, err := reader.LatestThreadContextWindow(ctx, threadID)
	if err != nil || raw == "" {
		return contextengine.WindowSnapshot{}, false
	}
	var rawPayload map[string]any
	if json.Unmarshal([]byte(raw), &rawPayload) != nil ||
		model.ValidatePublicEvent(model.EventContextWindowUpdated, rawPayload) != nil {
		return contextengine.WindowSnapshot{}, false
	}
	var payload model.ContextWindowUpdatedPayload
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return contextengine.WindowSnapshot{}, false
	}
	buckets := make(map[contextengine.ContextBucket]int, len(payload.Buckets))
	details := make(map[contextengine.ContextBucket][]contextengine.WindowBucketDetail, len(payload.Details))
	for _, bucket := range contextengine.ContextBuckets {
		key := string(bucket)
		buckets[bucket] = payload.Buckets[key]
		for _, detail := range payload.Details[key] {
			details[bucket] = append(details[bucket], contextengine.WindowBucketDetail{
				Name: detail.Name, Tokens: detail.Tokens,
			})
		}
	}
	return contextengine.WindowSnapshot{
		Total: payload.Total, Max: payload.Max, Ratio: payload.Ratio,
		Buckets: buckets, Details: details,
	}, true
}

func (svc *ContextWindowService) Compact(
	ctx context.Context,
	threadID string,
	modelProfile string,
) (CompactContextResult, error) {
	thread, project, profile, messages, err := svc.load(ctx, threadID, modelProfile)
	if err != nil {
		return CompactContextResult{}, err
	}
	if svc.locks == nil {
		return CompactContextResult{}, model.NewAgentError("INTERNAL", "compact", errors.New("lock manager unavailable"))
	}
	release, acquired := svc.locks.TryAcquire(project.ID)
	if !acquired {
		return CompactContextResult{}, model.NewAgentError("COMPACT_ACTIVE", "compact", nil)
	}
	defer release()
	if active, activeErr := svc.store.HasActiveRun(ctx, project.ID); activeErr != nil {
		return CompactContextResult{}, activeErr
	} else if active {
		return CompactContextResult{}, model.NewAgentError("COMPACT_ACTIVE", "compact", nil)
	}
	if active, activeErr := svc.store.HasActiveGitCommit(ctx, project.ID); activeErr != nil {
		return CompactContextResult{}, activeErr
	} else if active {
		return CompactContextResult{}, model.NewAgentError("COMPACT_ACTIVE", "compact", nil)
	}

	before, ok := svc.calibration.Snapshot(thread.ID)
	if !ok || before.Max != profile.Capabilities().ContextWindowTokens {
		before = svc.transcriptOnlySnapshot(thread.ID, profile, messages)
	}
	startedAt := time.Now()
	result, err := contextcompact.New(profile.Adapter()).Compact(ctx, messages)
	if err != nil {
		return CompactContextResult{}, err
	}
	if err := svc.transcripts.Replace(project.WorkDir, thread.ID, result.Messages); err != nil {
		return CompactContextResult{}, err
	}
	after := replaceTranscriptSnapshot(
		before,
		svc.transcriptOnlySnapshot(thread.ID, profile, messages),
		svc.transcriptOnlySnapshot(thread.ID, profile, result.Messages),
	)
	svc.calibration.SetSnapshot(thread.ID, after)
	reclaimed := before.Total - after.Total
	if reclaimed < 0 {
		reclaimed = 0
	}
	compaction := model.ContextCompaction{
		ID: model.MustShortID("cmp"), ThreadID: thread.ID, ProjectID: project.ID,
		Trigger: model.ContextCompactionManual, Summary: result.Summary,
		BeforeTokens: before.Total, AfterTokens: after.Total, MaxTokens: before.Max,
		Reclaimed: reclaimed, DurationMS: time.Since(startedAt).Milliseconds(),
		CreatedAt: time.Now().Unix(),
	}
	compactionStore, ok := svc.store.(contextCompactionStore)
	if !ok {
		return CompactContextResult{}, errors.New("context compaction store is required")
	}
	if err := compactionStore.CreateContextCompaction(ctx, compaction); err != nil {
		return CompactContextResult{}, err
	}
	return CompactContextResult{
		Snapshot:   ContextWindowSnapshot{WindowSnapshot: after, Status: "idle"},
		Compaction: compaction,
	}, nil
}

func (svc *ContextWindowService) load(
	ctx context.Context,
	threadID string,
	modelProfile string,
) (model.Thread, model.Project, llm.Profile, []llm.Message, error) {
	thread, err := svc.store.GetThread(ctx, threadID)
	if err != nil {
		return model.Thread{}, model.Project{}, llm.Profile{}, nil, err
	}
	project, err := svc.store.GetProject(ctx, thread.ProjectID)
	if err != nil {
		return model.Thread{}, model.Project{}, llm.Profile{}, nil, err
	}
	if svc.registry == nil {
		return model.Thread{}, model.Project{}, llm.Profile{}, nil,
			model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "context_window", nil)
	}
	profile, err := svc.registry.Resolve(modelProfile)
	if err != nil || profile.Adapter() == nil || profile.Capabilities().ContextWindowTokens <= 0 {
		return model.Thread{}, model.Project{}, llm.Profile{}, nil,
			model.NewAgentError("MODEL_PROFILE_NOT_FOUND", "context_window", err)
	}
	messages, err := svc.transcripts.Load(project.WorkDir, thread.ID)
	return thread, project, profile, messages, err
}

func (svc *ContextWindowService) transcriptOnlySnapshot(
	threadID string,
	profile llm.Profile,
	messages []llm.Message,
) contextengine.WindowSnapshot {
	return (contextengine.PromptEstimator{}).Estimate(contextengine.PromptEstimateInput{
		Messages: messages, Max: profile.Capabilities().ContextWindowTokens,
		Factor: svc.calibration.Factor(threadID),
	})
}

func replaceTranscriptSnapshot(
	base contextengine.WindowSnapshot,
	before contextengine.WindowSnapshot,
	after contextengine.WindowSnapshot,
) contextengine.WindowSnapshot {
	next := contextengine.WindowSnapshot{Max: base.Max}
	next.Buckets = map[contextengine.ContextBucket]int{}
	next.Details = map[contextengine.ContextBucket][]contextengine.WindowBucketDetail{}
	for _, bucket := range contextengine.ContextBuckets {
		for _, name := range contextengine.ContextWindowDetailNames(bucket) {
			tokens := detailTokens(base.Details[bucket], name) -
				detailTokens(before.Details[bucket], name) + detailTokens(after.Details[bucket], name)
			if tokens < 0 {
				tokens = detailTokens(after.Details[bucket], name)
			}
			next.Details[bucket] = append(next.Details[bucket], contextengine.WindowBucketDetail{
				Name: name, Tokens: tokens,
			})
			next.Buckets[bucket] += tokens
		}
		next.Total += next.Buckets[bucket]
	}
	if next.Max > 0 {
		next.Ratio = float64(next.Total) / float64(next.Max)
	}
	return next
}

func detailTokens(details []contextengine.WindowBucketDetail, name string) int {
	for _, detail := range details {
		if detail.Name == name {
			return detail.Tokens
		}
	}
	return 0
}
