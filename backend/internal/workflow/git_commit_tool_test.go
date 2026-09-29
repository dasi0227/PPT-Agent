package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestGitCommitDisclosedOnlyDuringExecutionWithoutCompactTool(t *testing.T) {
	registry := NewToolRegistry()
	provider := DefaultDomainToolProvider{GitCommit: func(context.Context, string, map[string]any) (map[string]any, error) { return nil, nil }}
	if err := provider.RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []model.RunMode{model.ModeChat, model.ModeGrill, model.ModePlan, model.ModeExecute} {
		for _, phase := range []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting} {
			found := false
			for _, schema := range registry.Disclose(phase, mode, model.NewRunScope(model.ScopeAllPages, "sli_aaaaaa")) {
				if schema.Name == "compact_context" {
					t.Fatal("compact_context must not be exposed")
				}
				found = found || schema.Name == "git_commit"
			}
			if found != (mode == model.ModeExecute && phase == PhaseExecuting) {
				t.Fatalf("git_commit disclosed=%t mode=%s phase=%s", found, mode, phase)
			}
		}
	}
}

func TestGitCommitReturnsReceiptAndDoesNotAutoRetryFailure(t *testing.T) {
	tool := gitCommitTool{execute: func(_ context.Context, callID string, args map[string]any) (map[string]any, error) {
		if callID != "commit-1" || args["title"] != "feat: 完善演示" {
			t.Fatal("lost commit identity or input")
		}
		return map[string]any{"hash": "abc1234", "branch": "main", "files_changed": 2}, nil
	}}
	result := tool.Execute(context.Background(), DomainToolInput{CallID: "commit-1", Args: map[string]any{"title": "feat: 完善演示"}})
	if !result.OK || result.Data["hash"] != "abc1234" {
		t.Fatalf("result=%+v", result)
	}
	tool.execute = func(context.Context, string, map[string]any) (map[string]any, error) {
		return nil, errors.New("unconfirmed receipt")
	}
	result = tool.Execute(context.Background(), DomainToolInput{})
	if result.OK || result.Retryable || result.Code != "GIT_COMMIT_FAILED" {
		t.Fatalf("result=%+v", result)
	}
}
