package workflow

import (
	"fmt"
	"html"
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
				modules := regexp.MustCompile(`(?s)<system_prompt id="([^"]+)"(?: description="([^"]*)")?>\n(.*?)\n</system_prompt>`).FindAllStringSubmatch(prompt, -1)
				if len(modules) == 0 || len(modules) != strings.Count(prompt, "<system_prompt ") {
					t.Fatal("no manifested modules")
				}
				seen := map[string]bool{}
				for _, module := range modules {
					id := module[1]
					expected := loadPromptModule(id)
					if html.UnescapeString(module[2]) != expected.Description || module[3] != expected.Body {
						t.Fatalf("description or body lost during assembly for %s", id)
					}
					if seen[id] {
						t.Fatalf("duplicate module %s", id)
					}
					seen[id] = true
				}
				want := []string{"core.agent", "core.output", "core.reference", "core.quality", "mode." + string(mode)}
				switch mode {
				case model.ModeChat, model.ModeGrill:
					want = append(want, "workflow.completion")
				case model.ModeExecute:
					want = append(want, "workflow.execution", "workflow.recovery", "workflow.completion", "playbook.init", "playbook.improve")
				}
				if mode == model.ModePlan || mode == model.ModeExecute {
					want = append(want, "playbook.html")
				}
				want = append(want, "playbook.runtime")
				if len(modules) != len(want) {
					t.Fatalf("wrong module count: %v", seen)
				}
				for i, id := range want {
					if modules[i][1] != id {
						t.Fatalf("module %d: got %s, want %s", i, modules[i][1], id)
					}
				}
				if strings.Contains(prompt, "{{CONTRACTS_JSON}}") {
					t.Fatal("unexpanded contract placeholder")
				}
				if !seen["core.quality"] || seen["workflow.execution"] != (mode == model.ModeExecute) {
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
		prompt, user := requestPromptText(AgentRequest{Mode: mode, Context: pack})
		if !strings.Contains(prompt, `id="`+modePolicyID(mode)+`"`) || !strings.Contains(user, `"run_mode": "`+string(mode)+`"`) {
			t.Fatalf("inconsistent mode: %s", mode)
		}
		pack.Command.Mode = mode
		for _, id := range playbookIDs(mode) {
			if !strings.Contains(prompt, `id="`+id+`"`) {
				t.Fatalf("playbook uses stale context mode: %s", mode)
			}
		}
		if mode != model.ModeChat && strings.Contains(user, `"run_mode": "chat"`) {
			t.Fatal("stale mode in dynamic context")
		}
	}
	pack := testPack(model.ModePlan, model.ScopeCurrentPage, false, "fallback")
	prompt, user := requestPromptText(AgentRequest{Context: pack})
	if !strings.Contains(prompt, `id="mode.plan"`) || !strings.Contains(user, `"run_mode": "plan"`) {
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

func requestPromptText(req AgentRequest) (string, string) {
	prepared := prepareAgentRequest(req)
	var parts []string
	for _, message := range prepared.RuntimeContext {
		if message.Metadata != nil && message.Metadata.Origin == "runtime" {
			parts = append(parts, message.Text())
		}
	}
	return runtimeSystemPromptForRequest(prepared), strings.Join(parts, "\n")
}
