package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func mutationPack(projectID string, outline spec.Outline) contextengine.ContextPack {
	return contextengine.ContextPack{Project: contextengine.ProjectContext{ID: projectID}, PresentationManifest: contextengine.PresentationManifestContext{Manifest: spec.Manifest{ProjectID: projectID}}, Outline: contextengine.OutlineContext{Outline: outline}, Revisions: contextengine.RevisionRefs{Manifest: 1, Outline: outline.Revision, Design: 1, SlideSpecs: map[string]int{}, SlideHTML: map[string]int{}}}
}

func mutationSchemaOps(schema ToolSchema) []string {
	variants, _ := schema.Parameters["oneOf"].([]any)
	out := make([]string, 0, len(variants))
	for _, value := range variants {
		variant, _ := value.(map[string]any)
		properties, _ := variant["properties"].(map[string]any)
		op, _ := properties["op"].(map[string]any)["const"].(string)
		out = append(out, op)
	}
	return out
}

func TestMutatePPTExposesClosedScopedOperations(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	tool := mutatePPTTool{pack: mutationPack("pro_aaaaaa", empty)}
	if ops := mutationSchemaOps(tool.Schema()); len(ops) != 12 {
		t.Fatalf("ops=%v", ops)
	}
	registry := NewToolRegistry()
	if err := registry.Register(tool, false, CapabilityPPTMutate, RiskMedium, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	schemas := registry.Disclose(PhaseExecuting, model.ModeExecute, model.RunScope{Artifact: model.ArtifactSpec, Level: model.ScopeSlide, SlideID: "sli_aaaaaa"})
	if len(schemas) != 1 {
		t.Fatalf("schemas=%v", schemas)
	}
	ops := mutationSchemaOps(schemas[0])
	if len(ops) != 2 || ops[0] != "slide.spec.write" || ops[1] != "slide.spec.patch" {
		t.Fatalf("scoped ops=%v", ops)
	}
	variants := schemas[0].Parameters["oneOf"].([]any)
	for _, raw := range variants {
		props := raw.(map[string]any)["properties"].(map[string]any)
		if props["slide_id"].(map[string]any)["const"] != "sli_aaaaaa" {
			t.Fatal("slide scope was not bound")
		}
	}
}

func TestMutationPatchSchemaDisclosesTheRuntimePathPolicy(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	variants := mutationSchema(mutationPack("pro_aaaaaa", empty))["oneOf"].([]any)
	deck := variants[0].(map[string]any)
	patch := deck["properties"].(map[string]any)["patch"].(map[string]any)
	if patch["maxItems"] != 32 {
		t.Fatalf("patch schema=%v", patch)
	}
	items := patch["items"].(map[string]any)["oneOf"].([]any)
	if len(items) != 3 {
		t.Fatalf("patch variants=%v", items)
	}
	add := items[0].(map[string]any)
	required := fmt.Sprint(add["required"])
	if !strings.Contains(required, "value") {
		t.Fatalf("add.required=%v", add["required"])
	}
	path := add["properties"].(map[string]any)["path"].(map[string]any)
	raw, _ := json.Marshal(path)
	if !strings.Contains(string(raw), "requirements") {
		t.Fatalf("deck path policy missing from schema: %s", raw)
	}
	remove := items[1].(map[string]any)
	if _, exists := remove["properties"].(map[string]any)["value"]; exists {
		t.Fatal("remove schema disclosed value")
	}
}

func TestSpecDeckScopeNeverDisclosesOrExecutesHTMLMutation(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	tool := mutatePPTTool{pack: mutationPack("pro_aaaaaa", empty)}
	registry := NewToolRegistry()
	if err := registry.Register(tool, false, CapabilityPPTMutate, RiskMedium, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	scope := model.RunScope{Artifact: model.ArtifactSpec, Level: model.ScopeDeck}
	schemas := registry.Disclose(PhaseExecuting, model.ModeExecute, scope)
	if len(schemas) != 1 {
		t.Fatalf("schemas=%v", schemas)
	}
	for _, op := range mutationSchemaOps(schemas[0]) {
		if op == "slide.html.write" || op == "slide.html.patch" {
			t.Fatalf("spec deck disclosed HTML mutation %q", op)
		}
	}
	if operationAllowed(scope, pptmutation.Request{Op: "slide.html.write", SlideID: "sli_aaaaaa"}) {
		t.Fatal("spec deck authorized an HTML mutation")
	}
}

func TestToolSchemasDoNotEmitNullRequired(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	nonEmpty := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Title: "Cover", Role: "cover"}}}}, CreatedAt: 1, UpdatedAt: 1}
	plan := &Plan{ID: "plan_1", Revision: 1, Status: PlanActive}

	for _, outline := range []spec.Outline{empty, nonEmpty} {
		pack := mutationPack("pro_aaaaaa", outline)
		registry := NewToolRegistry()
		if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			name    string
			phase   RunPhase
			mode    model.RunMode
			scope   model.RunScope
			control []ToolSchema
		}{
			{
				name:    "plan deck",
				phase:   PhasePlanning,
				mode:    model.ModePlan,
				scope:   model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck},
				control: controlSchemas(PhasePlanning, model.ModePlan, nil),
			},
			{
				name:    "execute deck",
				phase:   PhaseExecuting,
				mode:    model.ModeExecute,
				scope:   model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck},
				control: controlSchemas(PhaseExecuting, model.ModeExecute, plan),
			},
			{
				name:    "execute slide",
				phase:   PhaseExecuting,
				mode:    model.ModeExecute,
				scope:   model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeSlide, SlideID: "sli_aaaaaa"},
				control: controlSchemas(PhaseExecuting, model.ModeExecute, plan),
			},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				schemas := append(registry.Disclose(test.phase, test.mode, test.scope), test.control...)
				for _, schema := range schemas {
					var wire any
					raw, err := json.Marshal(schema.Parameters)
					if err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(raw, &wire); err != nil {
						t.Fatal(err)
					}
					assertProviderToolSchema(t, schema.Name, wire)
				}
			})
		}
	}
}

func TestDefaultToolDisclosureUsesTheSamePolicyAsExecution(t *testing.T) {
	pack := mutationPack("pro_aaaaaa", spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1})
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		phase RunPhase
		mode  model.RunMode
		scope model.RunScope
		want  []string
	}{
		{"chat", PhaseChat, model.ModeChat, model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck}, []string{"read_ppt", "run_command"}},
		{"plan", PhasePlanning, model.ModePlan, model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck}, []string{"read_ppt", "run_command"}},
		{"execute spec deck", PhaseExecuting, model.ModeExecute, model.RunScope{Artifact: model.ArtifactSpec, Level: model.ScopeDeck}, []string{"mutate_ppt", "read_ppt", "run_command"}},
		{"execute ppt slide", PhaseExecuting, model.ModeExecute, model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeSlide, SlideID: "sli_aaaaaa"}, []string{"mutate_ppt", "read_ppt", "render_slide", "run_command"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			schemas := registry.Disclose(test.phase, test.mode, test.scope)
			names := make([]string, 0, len(schemas))
			for _, schema := range schemas {
				names = append(names, schema.Name)
				desc, ok := registry.Descriptor(schema.Name)
				if !ok || !toolAvailable(desc, test.phase, test.mode, test.scope) {
					t.Fatalf("disclosed tool %q is not executable by the Runtime policy", schema.Name)
				}
			}
			if fmt.Sprint(names) != fmt.Sprint(test.want) {
				t.Fatalf("disclosed=%v want=%v", names, test.want)
			}
		})
	}
}

func TestMutationSchemaDoesNotExposeThemeWrites(t *testing.T) {
	raw, err := json.Marshal(mutationSchema(mutationPack("pro_aaaaaa", spec.Outline{})))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"theme"`) {
		t.Fatalf("mutate_ppt exposed theme write access: %s", raw)
	}
	for _, patchOp := range []string{"add", "remove", "replace"} {
		for _, rule := range pptmutation.PatchPathRules("design.patch", patchOp) {
			if strings.Contains(rule.Pattern, "theme") {
				t.Fatalf("design.patch %s permits theme writes: %+v", patchOp, rule)
			}
		}
	}
}

func TestToolRegistryRejectsIncoherentCapabilityPolicy(t *testing.T) {
	registry := NewToolRegistry()
	tool := mutatePPTTool{pack: mutationPack("pro_aaaaaa", spec.Outline{})}
	if err := registry.Register(tool, false, CapabilityPPTRead, RiskMedium, PhaseExecuting); err == nil {
		t.Fatal("write tool accepted a read-only capability")
	}
	if err := registry.Register(tool, false, ToolCapability("ppt.unknown"), RiskMedium, PhaseExecuting); err == nil {
		t.Fatal("unknown capability was accepted")
	}
}

func TestToolRegistryRejectsScopeOrModeThatDriftsFromRunCommand(t *testing.T) {
	pack := mutationPack("pro_aaaaaa", spec.Outline{})
	pack.Command = model.RunCommand{Scope: model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck}, Mode: model.ModeExecute, Instruction: "read deck"}
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	result := registry.Execute(context.Background(), map[string]bool{"read_ppt": true}, "read_ppt", map[string]any{}, DomainToolInput{
		Context: pack, Scope: model.RunScope{Artifact: model.ArtifactSpec, Level: model.ScopeDeck},
		Mode: model.ModeExecute, Phase: PhaseExecuting,
	})
	if result.Code != ErrCapabilityDenied.Error() {
		t.Fatalf("result=%+v", result)
	}
}

func assertProviderToolSchema(t *testing.T, name string, value any) {
	t.Helper()
	root, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s.parameters encoded as %T, want object", name, value)
	}
	if root["type"] != "object" {
		t.Fatalf("%s.parameters.type=%#v, want object", name, root["type"])
	}
	assertNoNullRequired(t, name, value)
}

func assertNoNullRequired(t *testing.T, path string, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		if required, exists := typed["required"]; exists {
			if required == nil {
				t.Fatalf("%s.required encoded as null", path)
			}
			if _, ok := required.([]any); !ok {
				t.Fatalf("%s.required encoded as %T, want array", path, required)
			}
		}
		for key, child := range typed {
			assertNoNullRequired(t, path+"."+key, child)
		}
	case []any:
		for index, child := range typed {
			assertNoNullRequired(t, fmt.Sprintf("%s[%d]", path, index), child)
		}
	}
}

func TestMutatePPTInitializesOutlineWithRuntimeIDsInRunOverlay(t *testing.T) {
	dir := t.TempDir()
	projectID := "pro_aaaaaa"
	deck := spec.Manifest{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, Canvas: spec.CanvasSettings{AspectRatio: "16:9"}, Numbering: spec.NumberingPolicy{Enabled: true, HiddenRoles: []string{"cover"}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1}
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	design := spec.Design{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Theme: "clean", Direction: "minimal", Density: "medium", Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 1}
	for path, value := range map[string]any{"manifest.json": deck, "outline.json": outline, "design.json": design} {
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(dir, path), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	session, err := NewRunSession(dir, "run_1")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	pack := mutationPack(projectID, outline)
	pack.Command = model.RunCommand{Scope: model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck}, Mode: model.ModeExecute, Instruction: "initialize deck"}
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{
		"op": "outline.init", "structure": []any{map[string]any{"client_ref": "opening", "title": "Opening", "purpose": "Start", "slides": []any{map[string]any{"client_ref": "cover", "title": "Cover", "role": "cover"}}, "subsections": []any{}}},
	}
	result := registry.Execute(context.Background(), map[string]bool{"mutate_ppt": true}, "mutate_ppt", args, DomainToolInput{Args: args, RunID: "run_1", ProjectDir: dir, Session: session, Context: pack, Scope: pack.Command.Scope, Phase: PhaseExecuting, Mode: model.ModeExecute})
	if !result.OK {
		t.Fatalf("result=%+v", result)
	}
	created := result.Data["created"].(map[string]string)
	if created["cover"] == "" || created["cover"][:4] != "sli_" {
		t.Fatalf("created=%v", created)
	}
	if _, err := os.Stat(filepath.Join(dir, "outline.json")); err != nil {
		t.Fatal(err)
	}
	raw, err := session.ReadPath("outline.json")
	if err != nil {
		t.Fatal(err)
	}
	var staged spec.Outline
	if err := json.Unmarshal(raw, &staged); err != nil {
		t.Fatal(err)
	}
	if len(spec.FlattenOutline(staged)) != 1 {
		t.Fatalf("staged=%#v", staged)
	}
}

func TestMutatePPTRejectsAgentSuppliedStableIDs(t *testing.T) {
	dir := t.TempDir()
	projectID := "pro_aaaaaa"
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	for path, value := range map[string]any{
		"manifest.json": spec.Manifest{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, Canvas: spec.CanvasSettings{AspectRatio: "16:9"}, Numbering: spec.NumberingPolicy{Enabled: true, HiddenRoles: []string{}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1},
		"outline.json":  outline,
		"design.json":   spec.Design{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Theme: "clean", Direction: "minimal", Density: "medium", Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 1},
	} {
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(dir, path), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	session, err := NewRunSession(dir, "run_1")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	result := (mutatePPTTool{pack: mutationPack(projectID, outline)}).Execute(context.Background(), DomainToolInput{ProjectDir: dir, Session: session, Scope: model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck}, Args: map[string]any{
		"op": "outline.init", "structure": []any{map[string]any{"id": "sec_agent", "client_ref": "opening", "title": "Opening", "purpose": "Start", "slides": []any{}, "subsections": []any{}}},
	}})
	if result.OK || result.Code != CodeContentInvalid {
		t.Fatalf("result=%+v", result)
	}
}

func TestRuntimeFrameForRenderUsesCurrentOutlineOrdinal(t *testing.T) {
	dir := t.TempDir()
	projectID := "pro_aaaaaa"
	deck := spec.Manifest{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, Canvas: spec.CanvasSettings{AspectRatio: "16:9"}, Numbering: spec.NumberingPolicy{Enabled: true, HiddenRoles: []string{"cover"}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1}
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Title: "Cover", Role: "cover"}, {SlideID: "sli_bbbbbb", Title: "Body", Role: "content"}}, Subsections: []spec.Subsection{}}}, CreatedAt: 1, UpdatedAt: 1}
	design := spec.Design{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Theme: "clean", Direction: "minimal", Density: "medium", Chrome: []spec.ChromeItem{{Type: "page_number", Placement: "bottom-right", Style: "muted"}}, CreatedAt: 1, UpdatedAt: 1}
	for path, value := range map[string]any{"manifest.json": deck, "outline.json": outline, "design.json": design} {
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(filepath.Join(dir, path), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	frame, err := runtimeFrameForRender(mutationPack(projectID, outline), dir, nil, "sli_bbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if frame.Ordinal != 2 || frame.Total != 2 || !frame.Numbering.Visible {
		t.Fatalf("frame=%#v", frame)
	}
}
