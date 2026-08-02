package contextengine

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
)

type RefKind string

const (
	RefSlideHTML       RefKind = "slide_html"
	RefHistoryEvidence RefKind = "history_evidence"
	RefAssetDetail     RefKind = "asset_detail"
	RefSpec            RefKind = "spec"
	RefToolResult      RefKind = "tool_result"
)

type ContextRef struct {
	ID              string              `json:"id"`
	Kind            RefKind             `json:"kind"`
	RunID           string              `json:"run_id"`
	ThreadID        string              `json:"thread_id"`
	ProjectID       string              `json:"project_id"`
	TargetID        string              `json:"target_id"`
	Revision        int                 `json:"revision"`
	ContentHash     string              `json:"content_hash"`
	Summary         string              `json:"summary"`
	AvailableLevels []DetailLevel       `json:"available_levels"`
	EstimatedTokens map[DetailLevel]int `json:"estimated_tokens"`
}

const (
	CodeRefNotFound    = "CONTEXT_REF_NOT_FOUND"
	CodeRefForbidden   = "CONTEXT_REF_FORBIDDEN"
	CodeRefStale       = "CONTEXT_REF_STALE"
	CodeBudgetExceeded = "CONTEXT_BUDGET_EXCEEDED"
)

type RefError struct{ Code, Message string }

func (e *RefError) Error() string { return e.Code + ": " + e.Message }

type refEntry struct {
	ref  ContextRef
	load func(context.Context, DetailLevel) ([]byte, int, string, error)
}

type RefRegistry struct {
	mu      sync.RWMutex
	entries map[string]refEntry
}

func NewRefRegistry() *RefRegistry { return &RefRegistry{entries: map[string]refEntry{}} }

func (r *RefRegistry) Register(ref ContextRef, load func(context.Context, DetailLevel) ([]byte, int, string, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[ref.ID] = refEntry{ref: ref, load: load}
}

func (r *RefRegistry) UnregisterRun(runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, entry := range r.entries {
		if entry.ref.RunID == runID {
			delete(r.entries, id)
		}
	}
}

func (r ContextRefResolver) CloseRun(runID string) {
	if r.Registry != nil {
		r.Registry.UnregisterRun(runID)
	}
}

type RefReadRequest struct {
	RunID, ThreadID, ProjectID, RefID string
	Detail                            DetailLevel
	RemainingBudget                   int
}

type RefReadResult struct {
	Content         string `json:"content"`
	EstimatedTokens int    `json:"estimated_tokens"`
	Revision        int    `json:"revision"`
	ContentHash     string `json:"content_hash"`
}

type ContextRefResolver struct{ Registry *RefRegistry }

func (r ContextRefResolver) Read(ctx context.Context, req RefReadRequest) (RefReadResult, error) {
	if req.RefID == "" || req.Detail == "" {
		return RefReadResult{}, &RefError{Code: CodeRefNotFound, Message: "ref_id and detail are required"}
	}
	r.Registry.mu.RLock()
	entry, ok := r.Registry.entries[req.RefID]
	r.Registry.mu.RUnlock()
	if !ok {
		return RefReadResult{}, &RefError{Code: CodeRefNotFound, Message: "reference is not in the current manifest"}
	}
	if entry.ref.RunID != req.RunID || entry.ref.ThreadID != req.ThreadID || entry.ref.ProjectID != req.ProjectID {
		return RefReadResult{}, &RefError{Code: CodeRefForbidden, Message: "reference belongs to another run, thread, or project"}
	}
	allowed := false
	for _, level := range entry.ref.AvailableLevels {
		allowed = allowed || level == req.Detail
	}
	if !allowed {
		return RefReadResult{}, &RefError{Code: CodeRefForbidden, Message: "detail level is not available"}
	}
	if entry.ref.EstimatedTokens[req.Detail] > req.RemainingBudget {
		return RefReadResult{}, &RefError{Code: CodeBudgetExceeded, Message: "remaining context budget is insufficient"}
	}
	raw, revision, hash, err := entry.load(ctx, req.Detail)
	if err != nil {
		var re *RefError
		if errors.As(err, &re) {
			return RefReadResult{}, err
		}
		return RefReadResult{}, err
	}
	got := fmt.Sprintf("%x", sha256.Sum256(raw))
	if revision != entry.ref.Revision || hash != entry.ref.ContentHash || (req.Detail == DetailFull && got != hash) {
		return RefReadResult{}, &RefError{Code: CodeRefStale, Message: "source revision or hash changed"}
	}
	return RefReadResult{Content: string(raw), EstimatedTokens: entry.ref.EstimatedTokens[req.Detail], Revision: revision, ContentHash: hash}, nil
}
