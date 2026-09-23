package workflow

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
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
				modules := regexp.MustCompile(`(?s)<prompt_module id="([^"]+)"[^>]*hash="([^"]+)">\n(.*?)\n</prompt_module>`).FindAllStringSubmatch(prompt, -1)
				if len(modules) == 0 {
					t.Fatal("no manifested modules")
				}
				seen := map[string]bool{}
				for _, module := range modules {
					id, hash, body := module[1], module[2], module[3]
					if seen[id] {
						t.Fatalf("duplicate module %s", id)
					}
					seen[id] = true
					if want := fmt.Sprintf("%x", sha256.Sum256([]byte(body))); hash != want {
						t.Fatalf("%s hash does not describe injected content", id)
					}
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
				if !writableMode {
					return
				}
				prefix := "Authoritative writable model contracts:\n"
				body := strings.SplitN(prompt, prefix, 2)[1]
				body = strings.SplitN(body, "\n</prompt_module>", 2)[0]
				var contracts map[string]json.RawMessage
				if err := json.Unmarshal([]byte(body), &contracts); err != nil {
					t.Fatal(err)
				}
				got, want := []string{}, []string{}
				for name := range contracts {
					got = append(got, name)
				}
				want = []string{pptschema.DesignName, pptschema.ManifestName, pptschema.OutlineName, pptschema.SlideSpecName}
				sort.Strings(got)
				sort.Strings(want)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("writable contracts = %v, want %v", got, want)
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

func TestPromptKeepsContractsAfterScopeExpansion(t *testing.T) {
	pack := testPack(model.ModeExecute, model.ScopeCurrentPage, false, "update chrome")
	before, _ := resourceContractsModule(pack)
	pack.Command.Scope.SlideIDs = []string{"sli_1", "sli_2"}
	pack.Command.Scope.Revision++
	after, _ := resourceContractsModule(pack)
	if before.Hash != after.Hash || !strings.Contains(before.Body, `"name":"design"`) {
		t.Fatal("page expansion changed resource contracts/hash")
	}
}
