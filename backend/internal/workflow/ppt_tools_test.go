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
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type successfulScreenshotRenderer struct{}

func (successfulScreenshotRenderer) Render(_ context.Context, request RenderRequest) (RenderDiagnostics, error) {
	if err := os.WriteFile(request.ScreenshotPath, []byte("screenshot"), 0o600); err != nil {
		return RenderDiagnostics{}, err
	}
	return RenderDiagnostics{
		ScreenshotBytes: 10,
		ContentSize:     map[string]int{"width": request.ViewportWidth, "height": request.ViewportHeight},
		Overflow:        map[string]bool{"horizontal": false, "vertical": false},
		FontStatus:      "loaded",
	}, nil
}

type staticThemeLoader struct {
	theme model.Theme
}

func (l staticThemeLoader) Get(string) (model.Theme, error) {
	return l.theme, nil
}

func mutationPack(projectID string, outline spec.Outline) contextengine.ContextPack {
	return contextengine.ContextPack{Project: contextengine.ProjectContext{ID: projectID}, PresentationManifest: contextengine.PresentationManifestContext{Manifest: spec.Manifest{ProjectID: projectID}}, Outline: contextengine.OutlineContext{Outline: outline}}
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
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	tool := mutatePPTTool{pack: mutationPack("pro_aaaaaa", empty)}
	if ops := mutationSchemaOps(tool.Schema()); len(ops) != 12 {
		t.Fatalf("ops=%v", ops)
	}
	registry := NewToolRegistry()
	if err := registry.Register(tool, false, CapabilityPPTMutate, RiskMedium, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	schemas := registry.Disclose(PhaseExecuting, model.ModeExecute, model.NewRunScope(model.ScopeCurrentPage, "sli_aaaaaa"))
	if len(schemas) != 1 {
		t.Fatalf("schemas=%v", schemas)
	}
	ops := mutationSchemaOps(schemas[0])
	if len(ops) != 12 {
		t.Fatalf("scoped ops=%v", ops)
	}
	variants := schemas[0].Parameters["oneOf"].([]any)
	for _, raw := range variants {
		props := raw.(map[string]any)["properties"].(map[string]any)
		idSchema, ok := props["slide_id"].(map[string]any)
		if !ok {
			continue
		}
		if idSchema["enum"] != nil || idSchema["pattern"] == nil {
			t.Fatal("dynamic page IDs leaked into static schema")
		}
	}
}

func TestMutationPatchSchemaDisclosesTheRuntimePathPolicy(t *testing.T) {
	schema := ToolSchema{Parameters: mutationSchema(mutationPack("pro_aaaaaa", spec.Outline{}))}
	for _, tc := range []struct {
		path  string
		value any
		valid bool
	}{
		{"/goal", "明确目标", true}, {"/requirements/-", "用中文", true},
		{"/goal", 42, false}, {"/goal", strings.Repeat("长", 2001), false}, {"/project_id", "other", false},
	} {
		args := map[string]any{"op": "manifest.patch", "patch": []any{map[string]any{"op": "replace", "path": tc.path, "value": tc.value}}}
		if tc.path == "/requirements/-" {
			args["patch"].([]any)[0].(map[string]any)["op"] = "add"
		}
		if err := validateToolArguments(schema, args); (err == nil) != tc.valid {
			t.Fatalf("%s valid=%t error=%v", tc.path, tc.valid, err)
		}
	}
}

func TestOutlineInitSchemaIncludesDirectAndGroupedExamples(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	variants := mutationSchema(mutationPack("pro_aaaaaa", empty))["oneOf"].([]any)
	outlineInit := variants[1].(map[string]any)
	examples, _ := outlineInit["examples"].([]any)
	if len(examples) != 2 {
		t.Fatalf("outline.init examples=%v", examples)
	}
	for _, raw := range examples {
		example, _ := raw.(map[string]any)
		if example["op"] != "outline.init" {
			t.Fatalf("outline.init example=%v", example)
		}
		structure, _ := example["structure"].([]any)
		if len(structure) != 1 {
			t.Fatalf("outline.init example structure=%v", structure)
		}
		section, _ := structure[0].(map[string]any)
		for _, field := range []string{"client_ref", "title", "purpose", "slides", "subsections"} {
			if _, ok := section[field]; !ok {
				t.Fatalf("outline.init example section is missing %s: %v", field, section)
			}
		}
		if err := validateToolArguments(ToolSchema{Parameters: mutationSchema(mutationPack("pro_aaaaaa", empty))}, example); err != nil {
			t.Fatalf("outline.init example does not satisfy its schema: %v", err)
		}
	}

	grouped := examples[1].(map[string]any)["structure"].([]any)[0].(map[string]any)
	subsections, _ := grouped["subsections"].([]any)
	if len(subsections) != 1 {
		t.Fatalf("grouped outline.init example subsections=%v", subsections)
	}
	subsection, _ := subsections[0].(map[string]any)
	for _, field := range []string{"client_ref", "title", "purpose", "slides"} {
		value, ok := subsection[field]
		if !ok || (field == "title" && strings.TrimSpace(fmt.Sprint(value)) == "") {
			t.Fatalf("grouped outline.init example subsection is missing %s: %v", field, subsection)
		}
	}
}

func TestSlideSpecSchemaIncludesLegalComparisonExample(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	schema := mutationSchema(mutationPack("pro_aaaaaa", empty))
	variants := schema["oneOf"].([]any)
	var slideSpecWrite map[string]any
	for _, raw := range variants {
		variant := raw.(map[string]any)
		properties := variant["properties"].(map[string]any)
		op, _ := properties["op"].(map[string]any)["const"].(string)
		if op == "slide.spec.write" {
			slideSpecWrite = variant
			break
		}
	}
	if slideSpecWrite == nil {
		t.Fatal("slide.spec.write schema is missing")
	}
	slideSpec := schema["$defs"].(map[string]any)["spec"].(map[string]any)
	examples, _ := slideSpec["examples"].([]any)
	if len(examples) != 1 {
		t.Fatalf("slide spec examples=%v", examples)
	}
	example := examples[0].(map[string]any)
	elements, _ := example["elements"].([]any)
	if len(elements) == 0 {
		t.Fatalf("comparison slide example has no elements: %v", example)
	}
	for _, raw := range elements {
		element := raw.(map[string]any)
		if element["type"] == "comparison" {
			t.Fatalf("comparison slide example used a slide role as an element type: %v", element)
		}
	}
	args := map[string]any{"op": "slide.spec.write", "slide_id": "sli_aaaaaa", "spec": example}
	if err := validateToolArguments(ToolSchema{Parameters: schema}, args); err != nil {
		t.Fatalf("comparison slide example does not satisfy slide.spec.write schema: %v", err)
	}

	elementSchema := slideSpec["properties"].(map[string]any)["elements"].(map[string]any)["items"].(map[string]any)
	typeDescription, _ := elementSchema["properties"].(map[string]any)["type"].(map[string]any)["description"].(string)
	if !strings.Contains(typeDescription, "comparison is a slide role") {
		t.Fatalf("element type description does not disambiguate comparison: %q", typeDescription)
	}
}

func TestToolRegistryReportsPreciseMissingOutlineFieldBeforeExecution(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	pack := mutationPack("pro_aaaaaa", empty)
	pack.Command = model.RunCommand{
		Scope: model.NewRunScope(model.ScopeAllPages),
		Mode:  model.ModeExecute, Instruction: "initialize outline",
	}
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{
		"op": "outline.init",
		"structure": []any{map[string]any{
			"client_ref": "opening", "purpose": "Start", "slides": []any{}, "subsections": []any{},
		}},
	}
	result := registry.Execute(context.Background(), map[string]bool{"mutate_ppt": true}, "mutate_ppt", args, DomainToolInput{
		Args: args, Context: pack, Scope: pack.Command.Scope, Phase: PhaseExecuting, Mode: model.ModeExecute,
	})
	if result.OK || result.Code != CodeContentInvalid || !strings.Contains(result.Summary, "/structure/0") || !strings.Contains(result.Summary, "title") {
		t.Fatalf("result=%+v", result)
	}
}

func TestToolRegistryReportsPreciseInvalidSlideSpecFieldBeforeExecution(t *testing.T) {
	outline := spec.Outline{
		SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", CreatedAt: 1, UpdatedAt: 1,
		Sections: []spec.Section{{
			ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start",
			Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Title: "Cover", Role: "cover"}},
		}},
	}
	pack := mutationPack("pro_aaaaaa", outline)
	pack.Command = model.RunCommand{
		Scope: model.NewRunScope(model.ScopeCurrentPage, "sli_aaaaaa"),
		Mode:  model.ModeExecute, Instruction: "write slide spec",
	}
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{
		"op": "slide.spec.write", "slide_id": "sli_aaaaaa",
		"spec": map[string]any{
			"key_message": "Compare options",
			"elements":    []any{map[string]any{"type": "comparison", "intent": "Compare A and B"}},
		},
	}
	result := registry.Execute(context.Background(), map[string]bool{"mutate_ppt": true}, "mutate_ppt", args, DomainToolInput{
		Args: args, Context: pack, Scope: pack.Command.Scope, Phase: PhaseExecuting, Mode: model.ModeExecute,
	})
	if result.OK || result.Code != CodeContentInvalid || !strings.Contains(result.Summary, "/spec/elements/0/type") {
		t.Fatalf("result=%+v", result)
	}
}

func TestPageScopeAllowsAllObjectsButRejectsOtherPageWrites(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	tool := mutatePPTTool{pack: mutationPack("pro_aaaaaa", empty)}
	registry := NewToolRegistry()
	if err := registry.Register(tool, false, CapabilityPPTMutate, RiskMedium, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	scope := model.NewRunScope(model.ScopeCurrentPage, "sli_aaaaaa")
	schemas := registry.Disclose(PhaseExecuting, model.ModeExecute, scope)
	if len(schemas) != 1 {
		t.Fatalf("schemas=%v", schemas)
	}
	for _, op := range []string{"manifest.patch", "outline.insert", "design.patch", "slide.spec.write", "slide.html.write"} {
		if !operationAllowed(scope, pptmutation.Request{Op: op, SlideID: "sli_aaaaaa"}) {
			t.Fatalf("selected page/global resource rejected: %s", op)
		}
	}
	for _, op := range []string{"slide.spec.write", "slide.html.write"} {
		if operationAllowed(scope, pptmutation.Request{Op: op, SlideID: "sli_other"}) {
			t.Fatalf("out-of-scope page authorized: %s", op)
		}
	}
}

func TestToolSchemasDoNotEmitNullRequired(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	nonEmpty := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Title: "Cover", Role: "cover"}}}}, CreatedAt: 1, UpdatedAt: 1}
	plan := &Plan{ApprovalID: "approval-test", ID: "plan_1", Status: PlanActive}

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
				scope:   model.NewRunScope(model.ScopeAllPages),
				control: controlSchemas(PhasePlanning, model.ModePlan, nil),
			},
			{
				name:    "execute deck",
				phase:   PhaseExecuting,
				mode:    model.ModeExecute,
				scope:   model.NewRunScope(model.ScopeAllPages),
				control: controlSchemas(PhaseExecuting, model.ModeExecute, plan),
			},
			{
				name:    "execute slide",
				phase:   PhaseExecuting,
				mode:    model.ModeExecute,
				scope:   model.NewRunScope(model.ScopeCurrentPage, "sli_aaaaaa"),
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
	pack := mutationPack("pro_aaaaaa", spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1})
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
		{"chat", PhaseChat, model.ModeChat, model.NewRunScope(model.ScopeAllPages), []string{"read_image", "read_ppt", "run_command"}},
		{"plan", PhasePlanning, model.ModePlan, model.NewRunScope(model.ScopeAllPages), []string{"read_image", "read_ppt", "run_command"}},
		{"execute empty deck", PhaseExecuting, model.ModeExecute, model.NewRunScope(model.ScopeAllPages), []string{"mutate_ppt", "read_image", "read_ppt", "render_slide", "run_command"}},
		{"execute ppt slide", PhaseExecuting, model.ModeExecute, model.NewRunScope(model.ScopeCurrentPage, "sli_aaaaaa"), []string{"mutate_ppt", "read_image", "read_ppt", "render_slide", "run_command"}},
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
	pack.Command = model.RunCommand{Scope: model.NewRunScope(model.ScopeAllPages), Mode: model.ModeExecute, Instruction: "read deck"}
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	result := registry.Execute(context.Background(), map[string]bool{"read_ppt": true}, "read_ppt", map[string]any{}, DomainToolInput{
		Context: pack, Scope: model.NewRunScope(model.ScopeCurrentPage, "sli_other"),
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
	deck := spec.Manifest{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, CreatedAt: 1, UpdatedAt: 1}
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	design := spec.Design{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Theme: "clean", Direction: "minimal", Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 1}
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
	pack.Command = model.RunCommand{Scope: model.NewRunScope(model.ScopeAllPages), Mode: model.ModeExecute, Instruction: "initialize deck"}
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
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	for path, value := range map[string]any{
		"manifest.json": spec.Manifest{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, CreatedAt: 1, UpdatedAt: 1},
		"outline.json":  outline,
		"design.json":   spec.Design{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Theme: "clean", Direction: "minimal", Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 1},
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
	result := (mutatePPTTool{pack: mutationPack(projectID, outline)}).Execute(context.Background(), DomainToolInput{ProjectDir: dir, Session: session, Scope: model.NewRunScope(model.ScopeAllPages), Args: map[string]any{
		"op": "outline.init", "structure": []any{map[string]any{"id": "sec_agent", "client_ref": "opening", "title": "Opening", "purpose": "Start", "slides": []any{}, "subsections": []any{}}},
	}})
	if result.OK || result.Code != CodeContentInvalid {
		t.Fatalf("result=%+v", result)
	}
}

func TestRuntimeFrameForRenderUsesCurrentOutlineOrdinal(t *testing.T) {
	dir, _ := renderThemeFixture(t)
	projectID := "pro_aaaaaa"
	deck := spec.Manifest{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, CreatedAt: 1, UpdatedAt: 1}
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Title: "Cover", Role: "cover"}, {SlideID: "sli_bbbbbb", Title: "Body", Role: "content"}}, Subsections: []spec.Subsection{}}}, CreatedAt: 1, UpdatedAt: 1}
	design := spec.Design{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Theme: "clean", Direction: "minimal", Chrome: []spec.ChromeItem{{Type: "page_number", Placement: "bottom-right", Style: "muted"}}, CreatedAt: 1, UpdatedAt: 1}
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
	if frame.Ordinal != 2 || frame.Total != 2 || frame.Canvas != spec.CanonicalCanvas() {
		t.Fatalf("frame=%#v", frame)
	}
}

func TestRenderSlideUsesHTMLArtifactHashWhenThemeCSSIsPresent(t *testing.T) {
	dir, themeCSS := renderThemeFixture(t)
	projectID := "pro_aaaaaa"
	slideID := "sli_attea2"
	deck := spec.Manifest{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, CreatedAt: 1, UpdatedAt: 1}
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: slideID, Title: "Cover", Role: spec.SlideRoleCover}}, Subsections: []spec.Subsection{}}}, CreatedAt: 1, UpdatedAt: 1}
	design := spec.Design{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, Theme: "clean", Direction: "minimal", Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 1}
	slide := spec.SlideSpec{SchemaVersion: spec.SchemaVersion, ProjectID: projectID, SlideID: slideID, KeyMessage: "Hello", Elements: []spec.Element{}, CreatedAt: 1, UpdatedAt: 1}
	html := []byte(`<!doctype html><html><body><section class="slide-stage"><h1>Hello</h1></section></body></html>`)
	for path, value := range map[string]any{
		"manifest.json":              deck,
		"outline.json":               outline,
		"design.json":                design,
		model.SlideSpecPath(slideID): slide,
	} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	htmlPath := filepath.Join(dir, model.SlideHTMLPath(slideID))
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(htmlPath, html, 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "run_1")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	pack := mutationPack(projectID, outline)
	result := (slideRenderTool{
		pack:     pack,
		renderer: successfulScreenshotRenderer{},
		themes:   staticThemeLoader{theme: model.Theme{ID: "clean", CSS: themeCSS}},
	}).Execute(context.Background(), DomainToolInput{
		Args: map[string]any{"slide_id": slideID}, RunID: "run_1", ProjectDir: dir, Session: session, Context: pack,
		Scope: model.NewRunScope(model.ScopeAllPages, slideID),
	})
	if !result.OK {
		t.Fatalf("render result=%+v", result)
	}
	wantHash := hashBytes(html)
	if result.Data["hash"] != wantHash || len(result.Evidence) != 2 || result.Evidence[0].SourceHash != wantHash || result.Evidence[1].Kind != "static" || result.Evidence[1].SourceHash != wantHash {
		t.Fatalf("render hash=%v evidence=%+v want=%s", result.Data["hash"], result.Evidence, wantHash)
	}
	if result.Evidence[0].Materialization == nil || result.Evidence[0].Materialization.ArtifactHash != wantHash {
		t.Fatalf("materialization=%+v want artifact hash %s", result.Evidence[0].Materialization, wantHash)
	}
	for _, part := range result.ObservationParts {
		if part.Type == "image" {
			t.Fatal("render automatically attached image pixels")
		}
	}
	images := latestRenderedImages(pack, dir, nil)
	if len(images) != 1 || images[0].ImagePath != result.Data["image_path"] || images[0].Stale {
		t.Fatalf("latest render index is missing or stale: %+v", images)
	}
	themePath := filepath.Join(dir, "..", "..", "..", "assets", "themes", "clean", "theme.css")
	if err := os.WriteFile(themePath, []byte(themeCSS+"\n.slide-title {color:red}"), 0644); err != nil {
		t.Fatal(err)
	}
	if changed := latestRenderedImages(pack, dir, nil); len(changed) != 1 || !changed[0].Stale {
		t.Fatal("CSS change under the same theme ID did not invalidate the screenshot")
	}
	if err := os.WriteFile(themePath, []byte(themeCSS), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(htmlPath, append(html, []byte("\n<!-- changed -->")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if updated := latestRenderedImages(pack, dir, nil); len(updated) != 1 || !updated[0].Stale {
		t.Fatalf("changed HTML did not invalidate render freshness: %+v", updated)
	}
}

func TestNoopMutationDoesNotInvalidateRuntimeEvidence(t *testing.T) {
	dir := t.TempDir()
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	raw, _ := json.Marshal(outline)
	if err := os.WriteFile(filepath.Join(dir, "outline.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "noop")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	pack := mutationPack("pro_aaaaaa", outline)
	tool := mutatePPTTool{pack: pack}
	result := tool.Execute(context.Background(), DomainToolInput{ProjectDir: dir, Session: session, Scope: model.NewRunScope(model.ScopeAllPages), Args: map[string]any{"op": "outline.init", "structure": []any{}}})
	if !result.OK || len(result.ChangedTargets) != 0 || len(result.InvalidatedTargets) != 0 {
		t.Fatalf("no-op reported a write: %+v", result)
	}
}

func TestReadProjectionPreservesWriteHashAndDeduplicatesVisibleContent(t *testing.T) {
	dir := t.TempDir()
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, ProjectID: "pro_private", CreatedAt: 1, UpdatedAt: 2,
		Sections: []spec.Section{{ID: "sec_a", Title: "开场", Purpose: "说明目标", Slides: []spec.SlideNode{{SlideID: "sli_a", Title: "业务目标", Role: "cover"}}, Subsections: []spec.Subsection{}}}}
	raw, _ := json.Marshal(outline)
	if err := os.WriteFile(filepath.Join(dir, "outline.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	tool := pptReadTool{pack: mutationPack("pro_private", outline)}
	input := DomainToolInput{ProjectDir: dir, Args: map[string]any{"resource": map[string]any{"kind": "outline"}}}
	first := tool.Execute(context.Background(), input)
	var observation map[string]any
	if err := json.Unmarshal([]byte(first.Observation), &observation); err != nil {
		t.Fatal(err)
	}
	if !first.OK || observation["content_hash"] != spec.ResourceBytesHash(raw) || strings.Contains(first.Observation, "pro_private") || !strings.Contains(first.Observation, `"ordinal":1`) {
		t.Fatalf("incorrect model projection: %s", first.Observation)
	}
	input.Messages = []llm.Message{{Role: llm.RoleTool, Content: llm.TextContent(first.Observation), Metadata: first.ObservationMetadata}}
	second := tool.Execute(context.Background(), input)
	if strings.Contains(second.Observation, `"content":`) || !strings.Contains(second.Observation, `"already_available":true`) {
		t.Fatal(second.Observation)
	}
	input.Messages = nil
	if restored := tool.Execute(context.Background(), input); !strings.Contains(restored.Observation, `"content":`) {
		t.Fatal("lost body was treated as visible")
	}
}

func TestToolSchemasRemainStableAcrossPageScopeAndKeepAuthorization(t *testing.T) {
	tool := mutatePPTTool{pack: mutationPack("pro_a", spec.Outline{})}
	base := tool.Schema()
	before, _ := json.Marshal(base)
	first := model.NewRunScope(model.ScopeCurrentPage, "sli_a")
	second := model.NewRunScope(model.ScopeCustomPages, "sli_b", "sli_c")
	a, _ := scopeToolSchema(base, first, false)
	b, _ := scopeToolSchema(base, second, false)
	aRaw, _ := json.Marshal(a)
	bRaw, _ := json.Marshal(b)
	after, _ := json.Marshal(base)
	if string(aRaw) != string(bRaw) || string(before) != string(after) {
		t.Fatal("scope filtering changed a stable/shared schema")
	}
	if result := tool.Execute(context.Background(), DomainToolInput{Session: &RunSession{}, Scope: first, Args: map[string]any{"op": "slide.spec.write", "slide_id": "sli_b", "spec": map[string]any{}}}); result.Code != CodeTargetOutOfScope {
		t.Fatalf("out-of-scope mutation was not rejected: %+v", result)
	}
}

func renderThemeFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "pro_aaaaaa", "artifacts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "seed", "assets", "themes", "editorial-serif", "theme.css"))
	if err != nil {
		t.Fatal(err)
	}
	themePath := filepath.Join(root, "assets", "themes", "clean", "theme.css")
	if err := os.MkdirAll(filepath.Dir(themePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(themePath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	return dir, string(raw)
}
