package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

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
