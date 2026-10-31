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
	for _, mode := range []model.RunMode{model.ModeChat, model.ModeAsk, model.ModePlan, model.ModeCreate} {
		for _, count := range []int{1, 3} {
			t.Run(fmt.Sprintf("%s/%d", mode, count), func(t *testing.T) {
				pack := testPack(mode, model.ScopeAllPages, false, "scope matrix")
				pack.Command.Scope.SlideIDs = []string{"sli_1"}
				if count > 1 {
					pack.Command.Scope.SlideIDs = append(pack.Command.Scope.SlideIDs, "sli_2", "sli_3")
				}
				prompt := runtimeSystemPromptForRequest(AgentRequest{Mode: mode, Context: pack})
				system, _, closed := strings.Cut(prompt, "\n</system_prompt>")
				if !closed || !strings.HasPrefix(system, "<system_prompt>\n") || strings.Count(prompt, "<system_prompt>") != 1 {
					t.Fatal("modules must share one system_prompt root")
				}
				modules := regexp.MustCompile(`(?s)<([a-z][a-z0-9_]*) desc="([^"]*)">\n(.*?)\n</([a-z][a-z0-9_]*)>`).FindAllStringSubmatch(system, -1)
				if len(modules) == 0 {
					t.Fatal("no tagged modules")
				}
				seen := map[string]bool{}
				for _, module := range modules {
					id := module[1]
					expected := loadPromptModule(id)
					if html.UnescapeString(module[2]) != expected.Desc || module[3] != expected.Body || module[4] != id {
						t.Fatalf("description or body lost during assembly for %s", id)
					}
					if seen[id] {
						t.Fatalf("duplicate module %s", id)
					}
					seen[id] = true
				}
				want := []string{"core_agent", "core_references", "core_protocols", "core_contracts"}
				want = append(want, "playbook_tool", "playbook_spec", "playbook_html", "rule_authoring", "rule_styling", "mode_"+string(mode))
				if len(modules) != len(want) {
					t.Fatalf("wrong module count: %v", seen)
				}
				for i, id := range want {
					if modules[i][1] != id {
						t.Fatalf("module %d: got %s, want %s", i, modules[i][1], id)
					}
				}
				addition := loadPromptModule("constraint_boundaries")
				wantAddition := "<fallible_point>\n" + addition.Render() + "\n</fallible_point>"
				if addition.Path != "prompts/addition/boundaries.md" {
					t.Fatal("reminder tag must use its ID independently of its filename")
				}
				if !strings.HasSuffix(prompt, wantAddition) || strings.Count(prompt, "<fallible_point>") != 1 || strings.Count(prompt, addition.Body) != 1 {
					t.Fatal("requirements reminder must appear once in a trailing fallible_point block")
				}
				if strings.Contains(prompt, "{{CONTRACTS_JSON}}") {
					t.Fatal("unexpanded contract placeholder")
				}
				if strings.Contains(prompt, "Authoritative writable model contracts:") || strings.Contains(prompt, "hash=") || strings.Contains(prompt, "path=") {
					t.Fatal("system prompt repeats schemas or debug metadata")
				}
			})
		}
	}
}

func TestPromptUsesOneEffectiveModeAcrossLayers(t *testing.T) {
	for _, mode := range []model.RunMode{model.ModeChat, model.ModeAsk, model.ModePlan, model.ModeCreate} {
		pack := testPack(model.ModeChat, model.ScopeCurrentPage, false, "effective mode")
		prompt, user := requestPromptText(AgentRequest{Mode: mode, Context: pack})
		if !strings.Contains(prompt, `<`+modePolicyID(mode)+` desc="`) || !strings.Contains(user, `"run_mode": "`+string(mode)+`"`) {
			t.Fatalf("inconsistent mode: %s", mode)
		}
		pack.Command.Mode = mode
		for _, id := range []string{"playbook_tool", "playbook_spec", "playbook_html"} {
			if !strings.Contains(prompt, `<`+id+` desc="`) {
				t.Fatalf("shared playbook %s missing from mode %s", id, mode)
			}
		}
		for _, id := range []string{"core_contracts", "rule_authoring", "rule_styling"} {
			if !strings.Contains(prompt, `<`+id+` desc="`) {
				t.Fatalf("shared module %s missing from mode %s", id, mode)
			}
		}
		if !strings.Contains(prompt, "<fallible_point>") {
			t.Fatalf("requirements reminder missing from mode %s", mode)
		}
		if mode != model.ModeChat && strings.Contains(user, `"run_mode": "chat"`) {
			t.Fatal("stale mode in dynamic context")
		}
	}
	pack := testPack(model.ModePlan, model.ScopeCurrentPage, false, "fallback")
	prompt, user := requestPromptText(AgentRequest{Context: pack})
	if !strings.Contains(prompt, `<mode_plan desc="`) || !strings.Contains(user, `"run_mode": "plan"`) {
		t.Fatal("missing request mode did not use command mode")
	}
}

func TestPromptKeepsOwnershipPolicyStableAcrossPageChanges(t *testing.T) {
	pack := testPack(model.ModeCreate, model.ScopeCurrentPage, false, "update decorations")
	pack.Command.Scope.SlideIDs = []string{"sli_1"}
	before := runtimeSystemPromptForRequest(AgentRequest{Phase: PhaseExecuting, Mode: model.ModeCreate, Context: pack})
	for _, scope := range []model.RunScope{
		model.NewRunScope(model.ScopeAllPages),
		model.NewRunScope(model.ScopeCustomPages, "sli_other", "sli_new"),
		model.NewRunScope(model.ScopeAllPages, "sli_new", "sli_other", "sli_third"),
	} {
		pack.Command.Scope = scope
		pack.Command.Scope.Revision++
		for _, phase := range []RunPhase{PhaseExecuting, PhaseCompletionCheck} {
			after := runtimeSystemPromptForRequest(AgentRequest{Phase: phase, Mode: model.ModeCreate, Context: pack})
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
