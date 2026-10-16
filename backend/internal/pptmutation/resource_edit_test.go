package pptmutation

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestResourceFieldsMergeValidateAndPreserveOtherPages(t *testing.T) {
	service, workspace := mutationFixture(t)
	seed, err := service.Apply(Request{Op: "outline.init", Structure: []DraftSection{{ClientRef: "section", Title: "Section", Slides: []DraftSlide{{ClientRef: "a", Title: "A"}, {ClientRef: "b", Title: "B"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	a, b := seed.Created["a"], seed.Created["b"]
	for _, id := range []string{a, b} {
		fields := map[string]any{"purpose": "content", "approach": "explanation"}
		if id == a {
			fields["core"] = "Original"
		} else {
			fields["elements"] = []any{map[string]any{"type": "text", "intent": "Explain B"}}
		}
		err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: id, Fields: fields})
		if err != nil {
			t.Fatal(err)
		}
	}
	var createdA, createdB spec.SlideSpec
	if err := json.Unmarshal(workspace[model.SlideSpecPath(a)], &createdA); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(workspace[model.SlideSpecPath(b)], &createdB); err != nil {
		t.Fatal(err)
	}
	if createdA.Elements == nil || len(createdA.Elements) != 0 || len(createdB.Elements) != 1 || createdB.Elements[0].Intent != "Explain B" {
		t.Fatalf("unexpected created elements: A=%+v B=%+v", createdA.Elements, createdB.Elements)
	}
	beforeB, _ := spec.ReadSlideSpec(workspace.Read, b)
	err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: a, Fields: map[string]any{"core": "Changed", "purpose": nil}})
	if err != nil {
		t.Fatal(err)
	}
	var slide map[string]any
	_ = json.Unmarshal(workspace[model.SlideSpecPath(a)], &slide)
	_, purposeExists := slide["purpose"]
	_, approachExists := slide["approach"]
	if purposeExists || approachExists || slide["core"] != "Changed" {
		t.Fatalf("saved spec=%+v", slide)
	}
	afterB, _ := spec.ReadSlideSpec(workspace.Read, b)
	if string(beforeB) != string(afterB) {
		t.Fatal("overwrote other page")
	}
	err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: a, Fields: map[string]any{"core": nil}})
	if err != nil {
		t.Fatal(err)
	}
	slide = map[string]any{}
	_ = json.Unmarshal(workspace[model.SlideSpecPath(a)], &slide)
	if _, exists := slide["core"]; exists {
		t.Fatal("cleared core was retained")
	}
	err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: b, Fields: map[string]any{"layout": "Two columns"}})
	if err != nil {
		t.Fatal(err)
	}
	slide = map[string]any{}
	_ = json.Unmarshal(workspace[model.SlideSpecPath(b)], &slide)
	elements, ok := slide["elements"].([]any)
	if slide["purpose"] != "content" || slide["approach"] != "explanation" || !ok || len(elements) != 1 {
		t.Fatal("partial edit lost saved fields")
	}
	if err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: b, Fields: map[string]any{"elements": []any{}}}); err != nil {
		t.Fatal(err)
	}
	slide = map[string]any{}
	_ = json.Unmarshal(workspace[model.SlideSpecPath(b)], &slide)
	if elements, ok := slide["elements"].([]any); !ok || len(elements) != 0 {
		t.Fatal("explicit empty elements did not clear the saved list")
	}
	beforeB = append([]byte(nil), workspace[model.SlideSpecPath(b)]...)
	if err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: b, Fields: map[string]any{"approach": nil}}); err == nil {
		t.Fatal("cleared approach on a content page")
	}
	if string(beforeB) != string(workspace[model.SlideSpecPath(b)]) {
		t.Fatal("invalid approach removal changed saved content")
	}
	before := append([]byte{}, workspace[model.SlideSpecPath(a)]...)
	for _, fields := range []map[string]any{{"core": "Lost", "elements": "wrong"}, {"elements": nil}, {"purpose": "content"}} {
		if err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: a, Fields: fields}); err == nil {
			t.Fatal("accepted invalid merged spec")
		}
		if string(before) != string(workspace[model.SlideSpecPath(a)]) {
			t.Fatal("failed edit persisted")
		}
	}
	if err = service.EditResource(ResourceEdit{Resource: "design", ExpectedHash: "sha256:stale", Fields: map[string]any{"requirements": []string{"stale"}}}); err != ErrContentConflict {
		t.Fatalf("missing conflict: %v", err)
	}
	damaged := []byte(`{"core":"Damaged"}`)
	workspace[model.SlideSpecPath(a)] = damaged
	if err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: a, Fields: map[string]any{"layout": "New layout"}}); err == nil {
		t.Fatal("repaired an existing spec missing elements")
	}
	if string(workspace[model.SlideSpecPath(a)]) != string(damaged) {
		t.Fatal("changed an existing damaged spec")
	}
}

func TestDesignEditReplacesSuppliedListsAndPreservesOmittedLists(t *testing.T) {
	service, workspace := mutationFixture(t)
	err := service.EditResource(ResourceEdit{Resource: "design", Fields: map[string]any{"prohibitions": []string{"No gradients"}}})
	if err != nil {
		t.Fatal(err)
	}
	err = service.EditResource(ResourceEdit{Resource: "design", Fields: map[string]any{"requirements": []string{"Open grids"}}})
	if err != nil {
		t.Fatal(err)
	}
	var design spec.Design
	if err := json.Unmarshal(workspace[".design.json"], &design); err != nil {
		t.Fatal(err)
	}
	if len(design.Requirements) != 1 || len(design.Prohibitions) != 1 {
		t.Fatalf("lost guidance: %+v", design)
	}
	before, _ := workspace.Read(".design.json")
	if err := service.EditResource(ResourceEdit{Resource: "design", Fields: map[string]any{"decorations": map[string]any{"logo": "top-right"}}}); err == nil {
		t.Fatal("authoring tool accepted human settings")
	}
	after, _ := workspace.Read(".design.json")
	if string(before) != string(after) {
		t.Fatal("rejected edit changed saved guidance")
	}
}

func TestOutlineSourceLifecycleIdentityAndDeletion(t *testing.T) {
	service, workspace := mutationFixture(t)
	delete(workspace, ".outline.json")
	for _, bad := range []string{
		`{}`,
		`{"sections":[{"title":"Bad"}]}`,
		`{"sections":[{"id":"","title":"Bad"}]}`,
		`{"sections":[{"id":null,"title":"Bad"}]}`,
		`{"sections":[{"id":"sec_invented","title":"Bad"}]}`,
		`{"sections":[{"id":"new","title":"Bad","slides":null}]}`,
		`{"sections":[{"id":"new","title":"Bad","subsections":"wrong"}]}`,
		`{"sections":[{"id":"new","title":"Bad","slides":[{"title":"A"}]}]}`,
		`{"sections":[{"id":"new","title":"Bad","slides":[{"id":"new","title":"A"}],"subsections":[{"id":"new","title":"Sub","slides":[{"id":"new","title":"B"}]}]}]}`,
	} {
		if err := service.EditResource(ResourceEdit{Resource: "outline", Content: bad}); err == nil {
			t.Fatalf("accepted invalid initial outline: %s", bad)
		}
		if _, exists := workspace[".outline.json"]; exists {
			t.Fatal("invalid initial outline was saved")
		}
	}
	content := `{"sections":[{"id":"new","title":"Section","slides":[{"id":"new","title":"A"},{"id":"new","title":"B"}]},{"id":"new","title":"Empty"},{"id":"new","title":"Grouped","subsections":[{"id":"new","title":"Nested"}]}]}`
	err := service.EditResource(ResourceEdit{Resource: "outline", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := workspace.Read(".outline.json")
	if err != nil {
		t.Fatal(err)
	}
	var outline spec.Outline
	_ = json.Unmarshal(initial, &outline)
	if len(outline.Sections) != 3 || len(outline.Sections[2].Subsections) != 1 ||
		outline.Sections[0].Subsections == nil ||
		outline.Sections[1].Slides == nil || outline.Sections[1].Subsections == nil ||
		outline.Sections[2].Slides == nil || outline.Sections[2].Subsections[0].Slides == nil {
		t.Fatalf("initial outline did not save complete child arrays: %+v", outline.Sections)
	}
	a, b := outline.Sections[0].Slides[0], outline.Sections[0].Slides[1]
	if !strings.HasPrefix(a.ID, "sli_") || !strings.HasPrefix(b.ID, "sli_") || a.ID == b.ID || strings.Contains(string(initial), `"new"`) {
		t.Fatal("IDs not assigned")
	}
	for _, section := range outline.Sections {
		if !strings.HasPrefix(section.ID, "sec_") {
			t.Fatalf("section ID=%q", section.ID)
		}
		for _, sub := range section.Subsections {
			if !strings.HasPrefix(sub.ID, "sec_") || sub.ID == section.ID {
				t.Fatalf("subsection ID=%q, section ID=%q", sub.ID, section.ID)
			}
		}
	}
	if err = service.EditResource(ResourceEdit{Resource: "outline", Content: string(initial), ExpectedHash: spec.ContentHash(initial)}); err != nil {
		t.Fatalf("identical content replacement failed: %v", err)
	}
	for _, slide := range []spec.SlideNode{a, b} {
		err = service.EditResource(ResourceEdit{Resource: "spec", SlideID: slide.ID, Fields: map[string]any{"core": slide.Title, "elements": []any{}}})
		if err != nil {
			t.Fatal(err)
		}
		workspace[model.SlideHTMLPath(slide.ID)] = []byte("<html>" + slide.Title + "</html>")
	}
	outline.Sections[0].Slides = []spec.SlideNode{b, a}
	candidate, _ := json.Marshal(outline)
	if err = service.EditResource(ResourceEdit{Resource: "outline", Content: string(candidate), ExpectedHash: "stale"}); !errors.Is(err, ErrContentConflict) {
		t.Fatalf("replacement did not reject a stale version: %v", err)
	}
	if string(workspace[".outline.json"]) != string(initial) {
		t.Fatal("stale replacement changed saved content")
	}
	err = service.EditResource(ResourceEdit{Resource: "outline", Content: string(candidate), ExpectedHash: spec.ContentHash(initial)})
	if err != nil {
		t.Fatal(err)
	}
	moved, err := workspace.Read(".outline.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(workspace[model.SlideHTMLPath(a.ID)]) != "<html>A</html>" {
		t.Fatal("move changed HTML")
	}
	var edited map[string]any
	if err = json.Unmarshal(moved, &edited); err != nil {
		t.Fatal(err)
	}
	edited["sections"] = append(edited["sections"].([]any),
		map[string]any{"id": "new", "title": "Added", "subsections": []any{map[string]any{"id": "new", "title": "New sub", "slides": []any{map[string]any{"id": "new", "title": "Draft"}}}}},
		map[string]any{"id": "new", "title": "Added empty"},
	)
	candidate, _ = json.Marshal(edited)
	if err = service.EditResource(ResourceEdit{Resource: "outline", Edits: []Edit{
		{OldText: string(moved), NewText: string(candidate)},
		{OldText: `{"id":"new","title":"Draft"}`, NewText: `{"id":"new","title":"Added page"}`},
	}}); err != nil {
		t.Fatal(err)
	}
	moved, err = workspace.Read(".outline.json")
	if err != nil {
		t.Fatal(err)
	}
	var expanded spec.Outline
	if err = json.Unmarshal(moved, &expanded); err != nil {
		t.Fatal(err)
	}
	if len(expanded.Sections) != 5 || len(expanded.Sections[3].Subsections) != 1 {
		t.Fatalf("new outline nodes missing: %+v", expanded.Sections)
	}
	added, empty := expanded.Sections[3], expanded.Sections[4]
	if added.ID == "" || added.Slides == nil || added.Subsections[0].ID == "" || added.Subsections[0].Slides == nil ||
		empty.ID == "" || empty.Slides == nil || empty.Subsections == nil {
		t.Fatalf("edited outline did not save complete new nodes: %+v %+v", added, empty)
	}
	if len(added.Subsections[0].Slides) != 1 || added.Subsections[0].Slides[0].Title != "Added page" || strings.Contains(string(moved), `"new"`) {
		t.Fatal("new IDs were not resolved after the complete edit")
	}
	if string(workspace[model.SlideHTMLPath(a.ID)]) != "<html>A</html>" {
		t.Fatal("adding nodes deleted an existing page")
	}
	var malformedExisting map[string]any
	if err = json.Unmarshal(moved, &malformedExisting); err != nil {
		t.Fatal(err)
	}
	delete(malformedExisting["sections"].([]any)[0].(map[string]any), "subsections")
	badExisting, _ := json.Marshal(malformedExisting)
	// Invalid candidates never write or delete artifacts; normalize only after strict parsing.
	for _, bad := range []string{
		strings.NewReplacer(outline.Sections[1].ID, outline.Sections[2].Subsections[0].ID, outline.Sections[2].Subsections[0].ID, outline.Sections[1].ID).Replace(string(moved)),
		strings.Replace(string(moved), a.ID, "sli_invented", 1),
		strings.Replace(string(moved), a.ID, outline.Sections[0].ID, 1),
		strings.Replace(string(moved), a.ID, b.ID, 1),
		strings.Replace(string(moved), `"`+a.ID+`"`, `""`, 1),
		strings.Replace(string(moved), `"`+a.ID+`"`, `null`, 1),
		strings.Replace(string(moved), `"id": "`+a.ID+`",`, "", 1),
		string(badExisting), `{"sections":[],"sections":[]}`,
	} {
		buffer := NewBuffer(workspace)
		engine := Service{Workspace: buffer}
		if err = engine.EditResource(ResourceEdit{Resource: "outline", Edits: []Edit{{OldText: string(moved), NewText: bad}}}); err == nil || buffer.HasChanges() {
			t.Fatal("invalid outline staged changes")
		}
	}
	// Replacing a saved page ID explicitly creates a different page, with no files inherited.
	replacement := NewBuffer(workspace)
	replacer := Service{Workspace: replacement, NewID: service.NewID}
	if err = replacer.EditResource(ResourceEdit{Resource: "outline", Content: strings.Replace(string(moved), `"id": "`+a.ID+`"`, `"id": "new"`, 1), ExpectedHash: spec.ContentHash(moved)}); err != nil {
		t.Fatal(err)
	}
	replaced, err := replacement.Read(".outline.json")
	if err != nil {
		t.Fatal(err)
	}
	var next spec.Outline
	if err = json.Unmarshal(replaced, &next); err != nil {
		t.Fatal(err)
	}
	newPage := next.Sections[0].Slides[1]
	if newPage.ID == a.ID || newPage.ID == "new" || newPage.Title != a.Title || next.Sections[0].Slides[0].ID != b.ID {
		t.Fatal("replacement reused the old identity or changed another page")
	}
	for _, id := range []string{a.ID, newPage.ID} {
		for _, path := range []string{model.SlideSpecPath(id), model.SlideHTMLPath(id)} {
			if _, err := replacement.Read(path); err == nil {
				t.Fatalf("replacement retained or inherited page source: %s", path)
			}
		}
	}
	if string(workspace[".outline.json"]) != string(moved) || len(workspace[model.SlideHTMLPath(a.ID)]) == 0 {
		t.Fatal("uncommitted replacement changed saved artifacts")
	}
	buffer := NewBuffer(workspace)
	engine := Service{Workspace: buffer}
	err = engine.EditResource(ResourceEdit{Resource: "outline", Content: `{"sections":[]}`, ExpectedHash: spec.ContentHash(moved)})
	if err != nil {
		t.Fatal(err)
	}
	cleared, err := buffer.Read(".outline.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace[model.SlideHTMLPath(a.ID)]) == 0 {
		t.Fatal("buffer deleted before commit")
	}
	if err = buffer.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, exists := workspace[model.SlideHTMLPath(a.ID)]; exists {
		t.Fatal("HTML not removed")
	}
	for _, id := range []string{a.ID, b.ID} {
		if _, exists := workspace[model.SlideSpecPath(id)]; exists {
			t.Fatal("Spec not removed", id)
		}
	}
	if string(workspace[".outline.json"]) != string(cleared) {
		t.Fatal("staged source differs from saved source")
	}
	if err = buffer.Rollback(); err != nil {
		t.Fatal(err)
	}
	if string(workspace[".outline.json"]) != string(moved) || len(workspace[model.SlideHTMLPath(a.ID)]) == 0 {
		t.Fatal("rollback did not restore directory and artifacts")
	}
}
