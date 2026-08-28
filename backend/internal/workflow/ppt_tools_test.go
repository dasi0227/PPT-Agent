package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func mutationPack(projectID string, outline spec.Outline) contextengine.ContextPack {
	return contextengine.ContextPack{Project: contextengine.ProjectContext{ID: projectID}, Deck: contextengine.DeckContext{Deck: spec.Deck{ProjectID: projectID}}, Outline: contextengine.OutlineContext{Outline: outline}, Revisions: contextengine.RevisionRefs{Deck: 1, Outline: outline.Revision, Design: 1, SlideSpecs: map[string]int{}, SlideHTML: map[string]int{}}}
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
	if err := registry.Register(tool, false, "ppt.mutate", RiskMedium, PhaseExecuting); err != nil {
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

func TestToolSchemasDoNotEmitNullRequired(t *testing.T) {
	empty := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	nonEmpty := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Label: "Cover", Role: "cover"}}}}, CreatedAt: 1, UpdatedAt: 1}
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
					assertNoNullRequired(t, schema.Name, wire)
				}
			})
		}
	}
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
	deck := spec.Deck{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, Canvas: spec.CanvasSettings{AspectRatio: "16:9"}, Numbering: spec.NumberingPolicy{Enabled: true, HiddenRoles: []string{"cover"}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1}
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1}
	design := spec.Design{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Theme: "clean", Direction: "minimal", Density: "medium", Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 1}
	for path, value := range map[string]any{"deck.json": deck, "outline.json": outline, "design.json": design} {
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
	result := (mutatePPTTool{pack: pack}).Execute(context.Background(), DomainToolInput{RunID: "run_1", ProjectDir: dir, Session: session, Scope: model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck}, Args: map[string]any{
		"op": "outline.init", "structure": []any{map[string]any{"client_ref": "opening", "title": "Opening", "purpose": "Start", "slides": []any{map[string]any{"client_ref": "cover", "label": "Cover", "role": "cover"}}, "subsections": []any{}}},
	}})
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
		"deck.json":    spec.Deck{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, Canvas: spec.CanvasSettings{AspectRatio: "16:9"}, Numbering: spec.NumberingPolicy{Enabled: true, HiddenRoles: []string{}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1},
		"outline.json": outline,
		"design.json":  spec.Design{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Theme: "clean", Direction: "minimal", Density: "medium", Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 1},
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
	deck := spec.Deck{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, Canvas: spec.CanvasSettings{AspectRatio: "16:9"}, Numbering: spec.NumberingPolicy{Enabled: true, HiddenRoles: []string{"cover"}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1}
	outline := spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Opening", Purpose: "Start", Slides: []spec.SlideNode{{SlideID: "sli_aaaaaa", Label: "Cover", Role: "cover"}, {SlideID: "sli_bbbbbb", Label: "Body", Role: "content"}}, Subsections: []spec.Subsection{}}}, CreatedAt: 1, UpdatedAt: 1}
	design := spec.Design{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Theme: "clean", Direction: "minimal", Density: "medium", Chrome: []spec.ChromeItem{{Type: "page_number", Placement: "bottom-right", Style: "muted"}}, CreatedAt: 1, UpdatedAt: 1}
	for path, value := range map[string]any{"deck.json": deck, "outline.json": outline, "design.json": design} {
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
