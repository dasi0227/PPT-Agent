package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

func TestManifestToolPreservesFieldDescriptionsAndPageRequirement(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "manifest-pages")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	tool := resourceEditTool{pack: pack, name: "edit_manifest"}
	authority, err := pptschema.RuntimeContract(pptschema.ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	props := tool.Schema().Parameters["properties"].(map[string]any)
	for _, field := range []string{"pages", "requirements", "prohibitions"} {
		property := props[field].(map[string]any)
		description := authority["properties"].(map[string]any)[field].(map[string]any)["description"]
		if description == nil || property["description"] != description {
			t.Fatalf("model-facing %s description was lost: %+v", field, property)
		}
		if field != "pages" && (property["type"] != "array" || property["maxItems"] != float64(32)) {
			t.Fatalf("resolved list constraints were lost: %+v", property)
		}
	}
	input := DomainToolInput{ProjectDir: dir, Session: session, Scope: pack.Command.Scope,
		Args: map[string]any{"pages": "11-12"}}
	result := tool.Execute(context.Background(), input)
	if !result.OK || !reflect.DeepEqual(result.Data["changed_fields"], []string{"pages"}) {
		t.Fatalf("edit=%+v", result)
	}
	input.Args = map[string]any{"resource": "manifest"}
	read := (pptReadTool{pack: pack}).Execute(context.Background(), input)
	if !read.OK {
		t.Fatalf("read=%+v", read)
	}
	var observed struct {
		Content spec.Manifest `json:"content"`
	}
	if err := json.Unmarshal([]byte(read.Observation), &observed); err != nil {
		t.Fatal(err)
	}
	if observed.Content.Pages != "11-12" || observed.Content.Goal != pack.PresentationManifest.Manifest.Goal {
		t.Fatalf("saved content=%+v", observed.Content)
	}
	for _, invalid := range []any{12, "", nil} {
		input.Args = map[string]any{"pages": invalid}
		if rejected := tool.Execute(context.Background(), input); rejected.OK {
			t.Fatalf("accepted invalid pages: %v", invalid)
		}
	}
	input.Args = map[string]any{"resource": "outline"}
	outline := (pptReadTool{pack: pack}).Execute(context.Background(), input)
	var saved spec.Outline
	if !outline.OK || json.Unmarshal([]byte(outline.Data["content"].(string)), &saved) != nil || len(spec.FlattenOutline(saved)) != 1 {
		t.Fatalf("page requirement changed the actual outline: %+v", outline)
	}
}

func TestResourceToolLifecycleAndExactOutlineRead(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	if err := os.Remove(filepath.Join(dir, ".outline.json")); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "resource-tools")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	registry := NewToolRegistry()
	if err = (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	schemas := registry.Disclose(PhaseExecuting, model.ModeExecute, pack.Command.Scope)
	names := schemasByName(discloseOutlineState(schemas, dir, session))
	if !names["init_outline"] || names["arrange_outline"] || names["mutate_ppt"] || names["read_ppt"] {
		t.Fatalf("tools=%v", names)
	}
	input := DomainToolInput{ProjectDir: dir, Session: session, Context: pack, Scope: pack.Command.Scope}
	input.Args = map[string]any{"content": `{"sections":[{"title":"Intro","purpose":"Explain","slides":[],"subsections":[]}]}`}
	initialized := (resourceEditTool{pack: pack, name: "init_outline"}).Execute(context.Background(), input)
	if !initialized.OK {
		t.Fatalf("init=%+v", initialized)
	}
	names = schemasByName(discloseOutlineState(schemas, dir, session))
	if names["init_outline"] || !names["arrange_outline"] {
		t.Fatalf("tools did not switch: %v", names)
	}
	again := (resourceEditTool{pack: pack, name: "init_outline"}).Execute(context.Background(), input)
	if again.OK {
		t.Fatal("stale init overwrote outline")
	}
	input.Args = map[string]any{"resource": "outline"}
	read := (pptReadTool{pack: pack}).Execute(context.Background(), input)
	if !read.OK {
		t.Fatalf("read=%+v", read)
	}
	source, ok := read.Data["content"].(string)
	if !ok || source != initialized.Data["content"] {
		t.Fatal("read did not return exact saved source")
	}
	input.Messages = nil
	readAgain := (pptReadTool{pack: pack}).Execute(context.Background(), input)
	if readAgain.Data["content"] != source {
		t.Fatal("explicit read omitted source")
	}
	input.Args = map[string]any{"edits": []any{map[string]any{"old_text": source, "new_text": `{"sections":[]}`}}, "expected_hash": read.Data["content_hash"]}
	cleared := (resourceEditTool{pack: pack, name: "arrange_outline"}).Execute(context.Background(), input)
	if !cleared.OK {
		t.Fatalf("clear=%+v", cleared)
	}
	names = schemasByName(discloseOutlineState(schemas, dir, session))
	if names["init_outline"] || !names["arrange_outline"] {
		t.Fatal("empty existing outline exposed initialization")
	}
	for _, args := range []map[string]any{{"resource": "spec"}, {"resource": "outline", "slide_id": generationSlide}, {"resource": map[string]any{"kind": "outline"}}} {
		if _, err := parseResource(args); err == nil {
			t.Fatalf("accepted invalid locator: %v", args)
		}
	}
}

func TestResourceEditReturnsFullSpecWithAuthoritativeEvidence(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	session, err := NewRunSession(dir, "edit-spec")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	result := (resourceEditTool{pack: pack, name: "edit_spec"}).Execute(context.Background(), DomainToolInput{ProjectDir: dir, Session: session, Scope: pack.Command.Scope, Args: map[string]any{"slide_id": generationSlide, "key_message": "Changed"}})
	if !result.OK {
		t.Fatalf("result=%+v", result)
	}
	raw, _, err := readArtifact(dir, session, specSlideRef(generationSlide))
	if err != nil {
		t.Fatal(err)
	}
	var saved spec.SlideSpec
	_ = json.Unmarshal(raw, &saved)
	if saved.KeyMessage != "Changed" || len(result.Evidence) != 1 || result.Evidence[0].SourceHash != hashBytes(raw) {
		t.Fatal("evidence did not use saved entry bytes")
	}
	if _, ok := result.Data["spec"].(map[string]any)["elements"]; !ok {
		t.Fatal("partial result omitted unchanged fields")
	}
	denied := (resourceEditTool{pack: pack, name: "edit_spec"}).Execute(context.Background(), DomainToolInput{ProjectDir: dir, Session: session, Scope: pack.Command.Scope, Args: map[string]any{"slide_id": "sli_other", "key_message": "Denied"}})
	if denied.OK || denied.Code != CodeTargetOutOfScope {
		t.Fatalf("scope=%+v", denied)
	}
}

func TestRuntimeSwitchesOutlineToolsAfterSuccessfulCommit(t *testing.T) {
	dir, _, existing := generationPackFixture(t)
	if err := os.Remove(filepath.Join(dir, ".outline.json")); err != nil {
		t.Fatal(err)
	}
	writeGenerationFile(t, dir, model.SpecCollectionPath, []byte(`{}`))
	pack := testPack(model.ModeExecute, model.ScopeAllPages, true, "创建一页目录")
	pack.Project = existing.Project
	pack.PresentationManifest = existing.PresentationManifest
	pack.Design = existing.Design
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("initialize", "init_outline", map[string]any{"content": `{"sections":[{"title":"Intro","purpose":"Explain","slides":[{"title":"Page"}],"subsections":[]}]}`}),
		toolCall("read", "read_resource", map[string]any{"resource": "outline"}),
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{RunID: "outline-lifecycle", ProjectDir: dir, Context: pack})
	if outcome.Status != StatusCompleted {
		t.Fatalf("outcome=%+v", outcome)
	}
	if len(agent.requests) != 3 {
		t.Fatalf("requests=%d", len(agent.requests))
	}
	first, second := schemasByName(agent.requests[0].Tools), schemasByName(agent.requests[1].Tools)
	if !first["init_outline"] || first["arrange_outline"] || second["init_outline"] || !second["arrange_outline"] {
		t.Fatalf("tool transition: %v -> %v", first, second)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	var outline spec.Outline
	_ = json.Unmarshal(raw, &outline)
	if len(spec.FlattenOutline(outline)) != 1 || outline.Sections[0].Slides[0].SlideID == "" {
		t.Fatal("initialization did not persist generated identities")
	}
	if len(outcome.Scope.SlideIDs) != 1 || outcome.Scope.SlideIDs[0] != outline.Sections[0].Slides[0].SlideID {
		t.Fatalf("new page not authorized for subsequent tools: %+v", outcome.Scope)
	}
}
