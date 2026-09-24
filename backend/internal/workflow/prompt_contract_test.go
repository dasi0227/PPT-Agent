package workflow

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Inspect the final provider-facing manifest, not just source templates.
func TestPromptAssemblyScopeMatrix(t *testing.T) {
	for _, mode := range []model.RunMode{model.ModeChat, model.ModeGrill, model.ModePlan, model.ModeExecute} {
		for _, count := range []int{1, 3} {
			t.Run(fmt.Sprintf("%s/%d", mode, count), func(t *testing.T) {
				pack := testPack(mode, model.ScopeAllPages, false, "scope matrix")
				pack.Command.Scope.SlideIDs = []string{"sli_1"}
				if count > 1 {
					pack.Command.Scope.SlideIDs = append(pack.Command.Scope.SlideIDs, "sli_2", "sli_3")
				}
				prompt := runtimeSystemPromptForRequest(AgentRequest{Mode: mode, Context: pack})
				modules := regexp.MustCompile(`(?s)<prompt_module id="([^"]+)">\n(.*?)\n</prompt_module>`).FindAllStringSubmatch(prompt, -1)
				if len(modules) == 0 {
					t.Fatal("no manifested modules")
				}
				seen := map[string]bool{}
				for _, module := range modules {
					id := module[1]
					if seen[id] {
						t.Fatalf("duplicate module %s", id)
					}
					seen[id] = true
				}
				modeCount, playbookCount := 0, 0
				for id := range seen {
					if strings.HasPrefix(id, "mode.") {
						modeCount++
					}
					if strings.HasPrefix(id, "playbook.") {
						playbookCount++
					}
				}
				if modeCount != 1 || !seen["mode."+string(mode)] {
					t.Fatalf("incorrect mode modules: %v", seen)
				}
				if mode != model.ModeExecute && playbookCount != 0 || mode == model.ModeExecute && (playbookCount != 3 || !seen["playbook.deck"] || !seen["playbook.spec"] || !seen["playbook.slide"]) {
					t.Fatalf("incorrect task modules: %v", seen)
				}
				writableMode := mode == model.ModePlan || mode == model.ModeExecute
				if seen["core.html"] != writableMode || seen["core.structure"] != writableMode {
					t.Fatalf("wrong scoped module set: %v", seen)
				}
				if strings.Contains(prompt, "{{CONTRACTS_JSON}}") {
					t.Fatal("unexpanded contract placeholder")
				}
				if !seen["core.quality"] || seen["runtime.execution"] != (mode == model.ModeExecute) {
					t.Fatalf("quality/evidence modules do not match mode: %v", seen)
				}
				if strings.Contains(prompt, "Authoritative writable model contracts:") || strings.Contains(prompt, "hash=") || strings.Contains(prompt, "path=") {
					t.Fatal("system prompt repeats schemas or debug metadata")
				}
			})
		}
	}
}

func TestPromptUsesOneEffectiveModeAcrossLayers(t *testing.T) {
	for _, mode := range []model.RunMode{model.ModeChat, model.ModeGrill, model.ModePlan, model.ModeExecute} {
		pack := testPack(model.ModeChat, model.ScopeCurrentPage, false, "effective mode")
		prompt, user := compiledPromptForAgentRequest(AgentRequest{Mode: mode, Context: pack})
		if !strings.Contains(prompt, `id="`+modePolicyID(mode)+`"`) || !strings.Contains(user, `"mode":"`+string(mode)+`"`) {
			t.Fatalf("inconsistent mode: %s", mode)
		}
		pack.Command.Mode = mode
		for _, id := range playbookIDs(mode) {
			if !strings.Contains(prompt, `id="`+id+`"`) {
				t.Fatalf("playbook uses stale context mode: %s", mode)
			}
		}
		if mode != model.ModeChat && strings.Contains(user, `"mode":"chat"`) {
			t.Fatal("stale mode in dynamic context")
		}
	}
	pack := testPack(model.ModePlan, model.ScopeCurrentPage, false, "fallback")
	prompt, user := compiledPromptForAgentRequest(AgentRequest{Context: pack})
	if !strings.Contains(prompt, `id="mode.plan"`) || !strings.Contains(user, `"mode":"plan"`) {
		t.Fatal("missing request mode did not use command mode")
	}
}

func TestPromptKeepsOwnershipPolicyStableAcrossPageChanges(t *testing.T) {
	pack := testPack(model.ModeExecute, model.ScopeCurrentPage, false, "update decorations")
	pack.Command.Scope.SlideIDs = []string{"sli_1"}
	before := runtimeSystemPromptForRequest(AgentRequest{Phase: PhaseExecuting, Mode: model.ModeExecute, Context: pack})
	for _, scope := range []model.RunScope{
		model.NewRunScope(model.ScopeAllPages),
		model.NewRunScope(model.ScopeCustomPages, "sli_other", "sli_new"),
		model.NewRunScope(model.ScopeAllPages, "sli_new", "sli_other", "sli_third"),
	} {
		pack.Command.Scope = scope
		pack.Command.Scope.Revision++
		for _, phase := range []RunPhase{PhaseExecuting, PhaseCompletionCheck} {
			after := runtimeSystemPromptForRequest(AgentRequest{Phase: phase, Mode: model.ModeExecute, Context: pack})
			if before != after {
				t.Fatal("page count, selection or transient phase changed static policy")
			}
		}
	}
}
