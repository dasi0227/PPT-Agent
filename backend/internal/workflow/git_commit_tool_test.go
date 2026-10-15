package workflow

import (
	"context"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestManualCommandToolsAreNeverRegisteredDisclosedOrExecuted(t *testing.T) {
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	names := []string{"git_commit", "git_commit.command", "compact_context", "polish_instruction"}
	for _, name := range names {
		if _, exists := registry.Descriptor(name); exists {
			t.Fatalf("manual command tool %s is registered", name)
		}
	}
	scope := model.NewRunScope(model.ScopeAllPages, "sli_aaaaaa")
	for _, mode := range []model.RunMode{model.ModeChat, model.ModeAsk, model.ModePlan, model.ModeExecute} {
		pack := contextengine.ContextPack{Command: model.RunCommand{Scope: scope, Mode: mode, Instruction: "提交项目版本"}}
		for _, phase := range []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting} {
			for _, restored := range []bool{false, true} {
				registry.allowed = nil
				if restored {
					// A persisted decision cannot restore an unregistered tool.
					registry.Limit(names)
				}
				disclosed := schemasByName(registry.Disclose(phase, mode, scope))
				for _, name := range names {
					if disclosed[name] {
						t.Fatalf("%s disclosed in mode=%s phase=%s restored=%t", name, mode, phase, restored)
					}
					result := registry.Execute(context.Background(), map[string]bool{name: true}, name,
						map[string]any{"title": "feat: 完善演示", "items": []any{"保存项目源文件"}},
						DomainToolInput{Context: pack, Scope: scope, Mode: mode, Phase: phase})
					if result.OK || result.Code != ErrToolNotDisclosed.Error() {
						t.Fatalf("forged %s call bypassed registry: %+v", name, result)
					}
				}
			}
		}
	}
}
