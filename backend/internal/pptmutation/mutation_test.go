package pptmutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type memoryWorkspace map[string][]byte

func (m memoryWorkspace) Read(path string) ([]byte, error) {
	raw, ok := m[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), raw...), nil
}
func (m memoryWorkspace) Write(path string, raw []byte) error {
	m[path] = append([]byte(nil), raw...)
	return nil
}
func (m memoryWorkspace) Delete(path string) error { delete(m, path); return nil }

func mutationFixture(t *testing.T) (*Service, memoryWorkspace) {
	t.Helper()
	workspace := memoryWorkspace{}
	write := func(path string, value any) { raw, _ := json.Marshal(value); workspace[path] = raw }
	write("outline.json", spec.Outline{SchemaVersion: spec.SchemaVersion, Revision: 0, ProjectID: "pro_aaaaaa", Sections: []spec.Section{}, CreatedAt: 1, UpdatedAt: 1})
	write("deck.json", spec.Deck{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Requirements: []string{}, Prohibitions: []string{}, Canvas: spec.CanvasSettings{AspectRatio: "16:9"}, Numbering: spec.NumberingPolicy{Enabled: true, HiddenRoles: []string{"cover"}, Format: "number"}, CreatedAt: 1, UpdatedAt: 1})
	write("design.json", spec.Design{SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "pro_aaaaaa", Theme: "clean", Direction: "minimal", Density: "medium", Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 1})
	sequence := 0
	service := &Service{Workspace: workspace, ProjectID: "pro_aaaaaa", Now: func() int64 { return 2 }, NewID: func(prefix string) string { sequence++; return fmt.Sprintf("%s_%06d", prefix, sequence) }, ValidateHTML: func(raw []byte) error {
		if len(raw) == 0 {
			return errors.New("empty")
		}
		return nil
	}}
	return service, workspace
}

func TestOutlineInitAllocatesRuntimeIDsAndPendingLeaves(t *testing.T) {
	service, workspace := mutationFixture(t)
	result, err := service.Apply(Request{Op: "outline.init", Structure: []DraftSection{{ClientRef: "opening", Title: "Opening", Purpose: "Start", Slides: []DraftSlide{{ClientRef: "cover", Label: "Cover", Role: "cover"}}, Subsections: []DraftSubsection{}}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Created["opening"] == "" || result.Created["cover"] == "" || result.Created["cover"][:4] != "sli_" {
		t.Fatalf("created=%v", result.Created)
	}
	var outline spec.Outline
	raw, _ := workspace.Read("outline.json")
	_ = json.Unmarshal(raw, &outline)
	if len(spec.FlattenOutline(outline)) != 1 {
		t.Fatalf("outline=%#v", outline)
	}
	if _, err := workspace.Read("slides/" + result.Created["cover"] + "/spec.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("init must leave slide spec pending")
	}
}

func TestTypedMutationsUseStableAnchorsAndAtomicPatchValidation(t *testing.T) {
	service, workspace := mutationFixture(t)
	init, err := service.Apply(Request{Op: "outline.init", Structure: []DraftSection{{ClientRef: "sec", Title: "Section", Purpose: "P", Slides: []DraftSlide{{ClientRef: "one", Label: "One", Role: "content"}, {ClientRef: "two", Label: "Two", Role: "content"}}, Subsections: []DraftSubsection{}}}})
	if err != nil {
		t.Fatal(err)
	}
	one, two, section := init.Created["one"], init.Created["two"], init.Created["sec"]
	if _, err = service.Apply(Request{Op: "outline.move", NodeID: two, Position: Position{ParentID: section, BeforeID: one}}); err != nil {
		t.Fatal(err)
	}
	var outline spec.Outline
	raw, _ := workspace.Read("outline.json")
	_ = json.Unmarshal(raw, &outline)
	if got := spec.FlattenOutline(outline); got[0].Slide.SlideID != two {
		t.Fatalf("move order=%v", got)
	}

	write := Request{Op: "slide.spec.write", SlideID: one, Spec: json.RawMessage(`{"title":"One","key_message":"Message","elements":[{"type":"text","intent":"Explain"}],"layout":"hero"}`)}
	if _, err = service.Apply(write); err != nil {
		t.Fatal(err)
	}
	before, _ := workspace.Read("slides/" + one + "/spec.json")
	_, err = service.Apply(Request{Op: "slide.spec.patch", SlideID: one, Patch: []Patch{{Op: "replace", Path: "/title", Value: "Changed"}, {Op: "replace", Path: "/slide_id", Value: "forbidden"}}})
	if err == nil {
		t.Fatal("runtime-managed patch path must fail")
	}
	after, _ := workspace.Read("slides/" + one + "/spec.json")
	if string(before) != string(after) {
		t.Fatal("failed patch was not atomic")
	}
}

func TestHTMLExactPatchRejectsAmbiguousAnchorAndStaticPageNumber(t *testing.T) {
	service, _ := mutationFixture(t)
	init, _ := service.Apply(Request{Op: "outline.init", Structure: []DraftSection{{ClientRef: "sec", Title: "S", Purpose: "P", Slides: []DraftSlide{{ClientRef: "one", Label: "One", Role: "content"}}, Subsections: []DraftSubsection{}}}})
	id := init.Created["one"]
	if _, err := service.Apply(Request{Op: "slide.html.write", SlideID: id, HTML: "<main><h1>One</h1></main>"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Apply(Request{Op: "slide.html.patch", SlideID: id, Edits: []Edit{{OldText: "missing", NewText: "x"}}}); err == nil {
		t.Fatal("missing exact anchor must fail")
	}
	if _, err := service.Apply(Request{Op: "slide.html.write", SlideID: id, HTML: `<main data-page-number="1"></main>`}); err == nil {
		t.Fatal("static page number must fail")
	}
}
