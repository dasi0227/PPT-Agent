package contextengine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ReadContextRefTool struct {
	resolver                   ContextRefResolver
	runID, threadID, projectID string
	mu                         sync.Mutex
	remaining                  int
}

func NewReadContextRefTool(resolver ContextRefResolver, manifest ContextManifest) *ReadContextRefTool {
	return &ReadContextRefTool{resolver: resolver, runID: manifest.RunID, threadID: manifest.ThreadID,
		projectID: manifest.ProjectID, remaining: manifest.BudgetTokens - manifest.EstimatedTokens}
}

func RefTool(pack *ContextPack) tools.Tool {
	if pack == nil || pack.RefResolver == nil {
		return nil
	}
	return NewReadContextRefTool(*pack.RefResolver, pack.Manifest)
}

func (*ReadContextRefTool) Name() string { return "read_context_ref" }
func (*ReadContextRefTool) Description() string {
	return "Expand an opaque reference from the current run context manifest."
}
func (*ReadContextRefTool) Class() tools.Class    { return tools.ClassRead }
func (*ReadContextRefTool) Scopes() []model.Scope { return nil }
func (*ReadContextRefTool) Capabilities() []tools.Capability {
	return []tools.Capability{tools.CapabilityReadContext}
}
func (*ReadContextRefTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": []any{"ref_id", "detail"}, "properties": map[string]any{
		"ref_id": map[string]any{"type": "string", "pattern": "^ctxref_[a-f0-9]+$"},
		"detail": map[string]any{"type": "string", "enum": []any{"summary", "structure", "full"}},
	}}
}

func (t *ReadContextRefTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	for key := range args {
		if key != "ref_id" && key != "detail" {
			raw, _ := json.Marshal(map[string]any{"ok": false, "error": map[string]string{"code": CodeRefForbidden, "message": "only opaque ref_id and detail are accepted"}})
			return tools.Result{OK: false, Observation: string(raw)}, nil
		}
	}
	refID, _ := args["ref_id"].(string)
	detailRaw, _ := args["detail"].(string)
	if detailRaw != string(DetailSummary) && detailRaw != string(DetailStructure) && detailRaw != string(DetailFull) {
		raw, _ := json.Marshal(map[string]any{"ok": false, "error": map[string]string{"code": CodeRefForbidden, "message": "invalid detail level"}})
		return tools.Result{OK: false, Observation: string(raw)}, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	result, err := t.resolver.Read(ctx, RefReadRequest{RunID: t.runID, ThreadID: t.threadID, ProjectID: t.projectID,
		RefID: refID, Detail: DetailLevel(detailRaw), RemainingBudget: t.remaining})
	if err != nil {
		raw, _ := json.Marshal(map[string]any{"ok": false, "error": err})
		return tools.Result{OK: false, Observation: string(raw)}, nil
	}
	t.remaining -= result.EstimatedTokens
	raw, err := json.Marshal(map[string]any{"ok": true, "content": result.Content, "estimated_tokens": result.EstimatedTokens,
		"revision": result.Revision, "content_hash": result.ContentHash})
	if err != nil {
		return tools.Result{}, fmt.Errorf("encode context ref result: %w", err)
	}
	return tools.Result{OK: true, Observation: string(raw)}, nil
}
