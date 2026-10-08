package service

import (
	"context"
	"errors"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextcompact"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type ContextWindowSnapshot struct {
	contextengine.WindowSnapshot
	Status string `json:"status"`
}

type CompactContextResult struct {
	ModelExecution llm.ModelExecution      `json:"model_execution"`
	Snapshot       ContextWindowSnapshot   `json:"snapshot"`
	Compaction     model.ContextCompaction `json:"compaction"`
}

type ContextWindowService struct {
	store       store.Store
	registry    *llm.Registry
	locks       *run.LockManager
	transcripts *contextengine.JournalTranscriptStore
	windows     *contextengine.WindowStore
	runs        *RunService
}

func NewContextWindowService(
	s store.Store,
	registry *llm.Registry,
	locks *run.LockManager,
	transcripts *contextengine.JournalTranscriptStore,
	windows *contextengine.WindowStore,
	runs *RunService,
) *ContextWindowService {
	return &ContextWindowService{
		store: s, registry: registry, locks: locks,
		transcripts: transcripts, windows: windows, runs: runs,
	}
}

func (svc *ContextWindowService) Snapshot(
	ctx context.Context,
	threadID string,
	modelProfile string,
) (ContextWindowSnapshot, error) {
	if snapshot, ok := svc.windows.Snapshot(threadID); ok {
		return ContextWindowSnapshot{WindowSnapshot: snapshot, Status: "idle"}, nil
	}
	thread, project, profile, messages, err := svc.load(ctx, threadID, modelProfile)
	if err != nil {
		return ContextWindowSnapshot{}, err
	}
	snapshot, err := svc.rebuild(ctx, thread, project, profile, messages)
	if err != nil {
		return ContextWindowSnapshot{}, err
	}
	// A live request can publish while the first read is rebuilding. Keep it.
	snapshot = svc.windows.SetInitialSnapshot(threadID, snapshot)
	return ContextWindowSnapshot{WindowSnapshot: snapshot, Status: "idle"}, nil
}

func (svc *ContextWindowService) Compact(
	ctx context.Context,
	threadID string,
	modelProfile string,
) (CompactContextResult, error) {
	if err := commandPhase(ctx, 0); err != nil {
		return CompactContextResult{}, err
	}
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
	if contextcompact.CompactableTokens(messages) < contextcompact.MinimumCompactableTokens {
		return CompactContextResult{}, model.NewAgentError("COMPACT_BELOW_THRESHOLD", "compact", nil)
	}
	side, err := svc.registry.RoutedProfile("compact", "")
	if err != nil {
		return CompactContextResult{}, err
	}

	before, err := svc.rebuild(ctx, thread, project, profile, messages)
	if err != nil {
		return CompactContextResult{}, err
	}
	startedAt := time.Now()
	if err := commandPhase(ctx, 1); err != nil {
		return CompactContextResult{}, err
	}
	result, err := contextcompact.New(side.Adapter()).Compact(ctx, messages)
	if err != nil {
		return CompactContextResult{}, err
	}
	if err := commandPhase(ctx, 2); err != nil {
		return CompactContextResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return CompactContextResult{}, err
	}
	commitCtx, cancelCommit := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelCommit()
	after, err := svc.rebuild(commitCtx, thread, project, profile, result.Messages)
	if err != nil {
		return CompactContextResult{}, err
	}
	if err := svc.transcripts.ReplaceFromContext(commitCtx, project.WorkDir, thread.ID, messages, result.Messages); err != nil {
		return CompactContextResult{}, err
	}
	svc.windows.SetSnapshot(thread.ID, after)
	reclaimed := before.Total - after.Total
	if reclaimed < 0 {
		reclaimed = 0
	}
	compaction := model.ContextCompaction{
		ID: model.MustShortID("cmp"), ThreadID: thread.ID, ProjectID: project.ID,
		Trigger: model.ContextCompactionManual, Title: model.PublicText(result.Title, contextengine.ProjectPublicTextContext(project, contextengine.PublicSourceText(messages))), Content: model.PublicText(result.Content, contextengine.ProjectPublicTextContext(project, contextengine.PublicSourceText(messages))),
		BeforeTokens: before.Total, AfterTokens: after.Total, MaxTokens: before.Max,
		Reclaimed: reclaimed, DurationMS: time.Since(startedAt).Milliseconds(),
		CreatedAt: time.Now().Unix(),
	}
	compactionStore, ok := svc.store.(contextCompactionStore)
	if !ok {
		return CompactContextResult{}, errors.New("context compaction store is required")
	}
	if err := compactionStore.CreateContextCompaction(commitCtx, compaction); err != nil {
		return CompactContextResult{}, err
	}
	return CompactContextResult{
		Snapshot:       ContextWindowSnapshot{WindowSnapshot: after, Status: "idle"},
		Compaction:     compaction,
		ModelExecution: llm.ExecutionOf(side.Adapter()),
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

// Reconstruct from source messages and the latest Run checkpoint, never from
// old token totals. The same path serves first reads after restart and compaction.
func (svc *ContextWindowService) rebuild(
	ctx context.Context, thread model.Thread, project model.Project, profile llm.Profile, messages []llm.Message,
) (contextengine.WindowSnapshot, error) {
	command := model.RunCommand{Mode: model.ModeChat, Scope: model.NewRunScope(model.ScopeAllPages)}
	runID := ""
	events, err := svc.store.ThreadEvents(ctx, thread.ID, 0)
	if err != nil {
		return contextengine.WindowSnapshot{}, err
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].RunID == "" {
			continue
		}
		stored, err := svc.store.GetRun(ctx, events[i].RunID)
		if err != nil {
			return contextengine.WindowSnapshot{}, err
		}
		runID, command = stored.ID, stored.Command
		break
	}
	var checkpoint *workflow.RuntimeCheckpoint
	if reader, ok := svc.store.(interface {
		LatestCheckpoint(context.Context, string) (workflow.RuntimeCheckpoint, error)
	}); ok && runID != "" {
		cp, err := reader.LatestCheckpoint(ctx, runID)
		if err != nil && !errors.Is(err, run.ErrRunNotFound) {
			return contextengine.WindowSnapshot{}, err
		}
		if err == nil {
			checkpoint = &cp
			command.Scope, command.Mode = cp.Scope, cp.Mode
		}
	}
	pack, err := svc.runs.assembler.AssembleSnapshot(ctx, contextengine.ContextRequest{
		RunID: runID, ThreadID: thread.ID, ProjectID: project.ID, Command: command,
	}, project)
	if err != nil {
		return contextengine.WindowSnapshot{}, err
	}
	return workflow.RebuildContextWindow(ctx, workflow.RuntimeInput{
		RunID: runID, ProjectDir: project.WorkDir, Context: pack, ResumeCheckpoint: checkpoint,
		DomainTools: svc.runs.contextDomainTools(pack, project, runID),
	}, messages, profile.Capabilities().ContextWindowTokens)
}
