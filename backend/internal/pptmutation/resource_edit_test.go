package pptmutation

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestResourceFieldsMergeValidateAndPreserveOtherPages(t *testing.T) {
	service, workspace := mutationFixture(t)
	seed, err := service.Apply(Request{Op: "outline.init", Structure: []DraftSection{{ClientRef: "section", Title: "Section", Purpose: "Explain", Slides: []DraftSlide{{ClientRef: "a", Title: "A"}, {ClientRef: "b", Title: "B"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	a, b := seed.Created["a"], seed.Created["b"]
	for _, id := range []string{a, b} {
		_, err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: id, Fields: map[string]any{"key_message": "Original", "elements": []any{}, "role": "content"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	beforeB, _ := spec.ReadSlideSpec(workspace.Read, b)
	result, err := service.EditResource(ResourceEdit{Resource: "spec", SlideID: a, Fields: map[string]any{"key_message": "Changed", "role": nil}})
	if err != nil {
		t.Fatal(err)
	}
	var slide map[string]any
	_ = json.Unmarshal(result.Content, &slide)
	if _, ok := slide["role"]; ok || slide["key_message"] != "Changed" || !reflect.DeepEqual(result.ChangedFields, []string{"key_message", "role"}) {
		t.Fatalf("result=%+v", result)
	}
	afterB, _ := spec.ReadSlideSpec(workspace.Read, b)
	if string(beforeB) != string(afterB) {
		t.Fatal("overwrote other page")
	}
	before := append([]byte{}, workspace[model.SpecCollectionPath]...)
	if _, err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: a, Fields: map[string]any{"key_message": "Lost", "elements": "wrong"}}); err == nil {
		t.Fatal("accepted invalid array")
	}
	if string(before) != string(workspace[model.SpecCollectionPath]) {
		t.Fatal("failed edit persisted")
	}
	original, _ := workspace.Read(".design.json")
	changed, err := service.EditResource(ResourceEdit{Resource: "design", Fields: map[string]any{"decorations": map[string]any{"page_number": "top-right"}}})
	if err != nil {
		t.Fatal(err)
	}
	var oldDesign, newDesign spec.Design
	_ = json.Unmarshal(original, &oldDesign)
	_ = json.Unmarshal(changed.Content, &newDesign)
	if newDesign.Decorations.SectionTitle != oldDesign.Decorations.SectionTitle || newDesign.Decorations.PageNumber != "top-right" {
		t.Fatal("decoration merge lost fields")
	}
	if _, err = service.EditResource(ResourceEdit{Resource: "design", ExpectedHash: "sha256:stale", Fields: map[string]any{"direction": "stale"}}); err != ErrContentConflict {
		t.Fatalf("missing conflict: %v", err)
	}
}

func TestOutlineSourceLifecycleIdentityAndDeletion(t *testing.T) {
	service, workspace := mutationFixture(t)
	delete(workspace, ".outline.json")
	content := `{"sections":[{"title":"Section","purpose":"Explain","slides":[{"title":"A"},{"title":"B"}],"subsections":[]}]}`
	result, err := service.EditResource(ResourceEdit{Resource: "outline", Initialize: true, Content: content})
	if err != nil {
		t.Fatal(err)
	}
	var outline spec.Outline
	_ = json.Unmarshal(result.Content, &outline)
	a, b := outline.Sections[0].Slides[0], outline.Sections[0].Slides[1]
	if a.SlideID == "" || b.SlideID == "" || a.SlideID == b.SlideID {
		t.Fatal("IDs not assigned")
	}
	if _, err = service.EditResource(ResourceEdit{Resource: "outline", Initialize: true, Content: content}); err == nil {
		t.Fatal("reinitialized existing outline")
	}
	for _, slide := range []spec.SlideNode{a, b} {
		_, err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: slide.SlideID, Fields: map[string]any{"key_message": slide.Title, "elements": []any{}}})
		if err != nil {
			t.Fatal(err)
		}
		workspace[model.SlideHTMLPath(slide.SlideID)] = []byte("<html>" + slide.Title + "</html>")
	}
	outline.Sections[0].Slides = []spec.SlideNode{b, a}
	candidate, _ := json.Marshal(outline)
	moved, err := service.EditResource(ResourceEdit{Resource: "outline", Edits: []Edit{{OldText: string(result.Content), NewText: string(candidate)}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(workspace[model.SlideHTMLPath(a.SlideID)]) != "<html>A</html>" {
		t.Fatal("move changed HTML")
	}
	// Invalid candidates never write or delete artifacts; normalize only after strict parsing.
	for _, bad := range []string{strings.Replace(string(moved.Content), a.SlideID, "sli_invented", 1), `{"sections":[],"sections":[]}`} {
		buffer := NewBuffer(workspace)
		engine := Service{Workspace: buffer}
		if _, err = engine.EditResource(ResourceEdit{Resource: "outline", Edits: []Edit{{OldText: string(moved.Content), NewText: bad}}}); err == nil || buffer.HasChanges() {
			t.Fatal("invalid outline staged changes")
		}
	}
	buffer := NewBuffer(workspace)
	engine := Service{Workspace: buffer}
	cleared, err := engine.EditResource(ResourceEdit{Resource: "outline", Edits: []Edit{{OldText: string(moved.Content), NewText: `{"sections":[]}`}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace[model.SlideHTMLPath(a.SlideID)]) == 0 {
		t.Fatal("buffer deleted before commit")
	}
	if err = buffer.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, exists := workspace[model.SlideHTMLPath(a.SlideID)]; exists {
		t.Fatal("HTML not removed")
	}
	entries, _ := spec.ReadCollection(workspace.Read)
	if len(entries) != 0 {
		t.Fatal("Specs not removed")
	}
	if string(workspace[".outline.json"]) != string(cleared.Content) {
		t.Fatal("returned source differs from saved source")
	}
	if err = buffer.Rollback(); err != nil {
		t.Fatal(err)
	}
	if string(workspace[".outline.json"]) != string(moved.Content) || len(workspace[model.SlideHTMLPath(a.SlideID)]) == 0 {
		t.Fatal("rollback did not restore directory and artifacts")
	}
}
