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
		for _, object := range []model.ScopeObject{model.ScopeObjectSpec, model.ScopeObjectHTML, model.ScopeObjectPresentation, model.ScopeObjectGlobal} {
			for _, count := range []int{1, 3} {
				t.Run(fmt.Sprintf("%s/%s/%d", mode, object, count), func(t *testing.T) {
					pack := testPack(mode, object, model.ScopeAllPages, false, "scope matrix")
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
					writableMode := mode == model.ModePlan || mode == model.ModeExecute
					if seen["html_authoring"] != (writableMode && pack.Command.Scope.AllowsHTML()) || seen["resource_contracts"] != writableMode {
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
					switch object {
					case model.ScopeObjectGlobal:
						want = []string{pptschema.DesignName, pptschema.ManifestName, pptschema.OutlineName, pptschema.SlideSpecName}
					case model.ScopeObjectSpec, model.ScopeObjectPresentation:
						want = []string{pptschema.SlideSpecName}
					}
					sort.Strings(got)
					sort.Strings(want)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("writable contracts = %v, want %v", got, want)
					}
				})
			}
		}
	}
}

func TestPromptUsesOneEffectiveModeAcrossLayers(t *testing.T) {
	for _, mode := range []model.RunMode{model.ModeChat, model.ModeGrill, model.ModePlan, model.ModeExecute} {
		pack := testPack(model.ModeChat, model.ScopeObjectPresentation, model.ScopeCurrentPage, false, "effective mode")
		prompt, user := compiledPromptForAgentRequest(AgentRequest{Mode: mode, Context: pack})
		if !strings.Contains(prompt, `id="`+modePolicyID(mode)+`"`) || !strings.Contains(user, `"mode":"`+string(mode)+`"`) {
			t.Fatalf("inconsistent mode: %s", mode)
		}
		pack.Command.Mode = mode
		if !strings.Contains(prompt, `id="`+playbookID(pack)+`"`) {
			t.Fatalf("playbook uses stale context mode: %s", mode)
		}
		if mode != model.ModeChat && strings.Contains(user, `"mode":"chat"`) {
			t.Fatal("stale mode in dynamic context")
		}
	}
	pack := testPack(model.ModePlan, model.ScopeObjectSpec, model.ScopeCurrentPage, false, "fallback")
	prompt, user := compiledPromptForAgentRequest(AgentRequest{Context: pack})
	if !strings.Contains(prompt, `id="playbook_read_only_planning"`) || !strings.Contains(user, `"mode":"plan"`) {
		t.Fatal("missing request mode did not use command mode")
	}
}

func TestPromptRefreshesContractsAfterScopeExpansion(t *testing.T) {
	pack := testPack(model.ModeExecute, model.ScopeObjectHTML, model.ScopeCurrentPage, false, "update chrome")
	before, _ := resourceContractsModule(pack)
	pack.Command.Scope.Object = model.ScopeObjectGlobal
	pack.Command.Scope.Revision++
	after, _ := resourceContractsModule(pack)
	if before.Hash == after.Hash || strings.Contains(before.Body, `"name":"design"`) || !strings.Contains(after.Body, `"name":"design"`) {
		t.Fatal("scope expansion did not refresh model contracts/hash")
	}
}
