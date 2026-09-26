package pptmutation

import (
	"encoding/json"
	"errors"
	"io/fs"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestSharedSpecsKeepPerPageConflictsAndDeleteOnlyTheirPage(t *testing.T) {
	engine, workspace := mutationFixture(t)
	result, err := engine.Apply(Request{Op: "outline.init", Structure: []DraftSection{{ClientRef: "section", Title: "Section", Purpose: "Explain", Slides: []DraftSlide{{ClientRef: "a", Title: "A"}, {ClientRef: "b", Title: "B"}}, Subsections: []DraftSubsection{}}}})
	if err != nil {
		t.Fatal(err)
	}
	a, b := result.Created["a"], result.Created["b"]
	initial := json.RawMessage(`{"key_message":"Initial","elements":[]}`)
	for _, id := range []string{a, b} {
		if _, err := engine.Apply(Request{Op: "slide.spec.write", SlideID: id, Spec: initial}); err != nil {
			t.Fatal(err)
		}
		workspace[model.SlideHTMLPath(id)] = []byte("existing HTML")
	}
	beforeB, _ := spec.ReadSlideSpec(workspace.Read, b)
	initialHash := spec.ResourceBytesHash(initial)
	changed, err := engine.Apply(Request{Op: "slide.spec.patch", SlideID: a, ExpectedHash: initialHash, Patch: []Patch{{Op: "replace", Path: "/key_message", Value: "Changed A"}}})
	if err != nil || len(changed.AffectedSlideIDs) != 1 || changed.AffectedSlideIDs[0] != a {
		t.Fatalf("change: %+v %v", changed, err)
	}
	afterB, _ := spec.ReadSlideSpec(workspace.Read, b)
	if string(beforeB) != string(afterB) {
		t.Fatal("editing A overwrote B")
	}
	if _, err := engine.Apply(Request{Op: "slide.spec.write", SlideID: a, ExpectedHash: initialHash, Spec: initial}); !errors.Is(err, ErrContentConflict) {
		t.Fatalf("stale A accepted: %v", err)
	}
	if _, err := engine.Apply(Request{Op: "slide.spec.patch", SlideID: b, ExpectedHash: initialHash, Patch: []Patch{{Op: "replace", Path: "/key_message", Value: "Changed B"}}}); err != nil {
		t.Fatalf("A incorrectly invalidated B's hash: %v", err)
	}
	if _, err := engine.Apply(Request{Op: "outline.remove", NodeID: a}); err != nil {
		t.Fatal(err)
	}
	if _, err := spec.ReadSlideSpec(workspace.Read, a); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("deleted spec remains: %v", err)
	}
	if _, err := workspace.Read(model.SlideHTMLPath(a)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("deleted HTML remains")
	}
	if _, err := spec.ReadSlideSpec(workspace.Read, b); err != nil {
		t.Fatal("B removed", err)
	}
	if _, err := workspace.Read(model.SlideHTMLPath(b)); err != nil {
		t.Fatal("B HTML removed", err)
	}
}

func TestDamagedSpecCollectionCannotBeOverwrittenAsEmpty(t *testing.T) {
	engine, workspace := mutationFixture(t)
	result, err := engine.Apply(Request{Op: "outline.init", Structure: []DraftSection{{ClientRef: "s", Title: "Section", Purpose: "Explain", Slides: []DraftSlide{{ClientRef: "a", Title: "A"}}, Subsections: []DraftSubsection{}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, damaged := range []string{`null`, `[]`, `{"sli_a":{"elements":[]}}`, `{"sli_a":{},"sli_a":{}}`, `{`} {
		workspace[model.SpecCollectionPath] = []byte(damaged)
		if _, err := engine.Apply(Request{Op: "slide.spec.write", SlideID: result.Created["a"], Spec: json.RawMessage(`{"key_message":"Valid","elements":[]}`)}); err == nil {
			t.Fatalf("accepted damaged collection %s", damaged)
		}
		if string(workspace[model.SpecCollectionPath]) != damaged {
			t.Fatal("damaged data silently replaced")
		}
	}
}

func TestRoleEditsBelongOnlyToSpecAndDoNotRewriteHTML(t *testing.T) {
	engine, workspace := mutationFixture(t)
	created, err := engine.Apply(Request{Op: "outline.init", Structure: []DraftSection{{ClientRef: "section", Title: "Section", Purpose: "Explain", Slides: []DraftSlide{{ClientRef: "page", Title: "Page"}}, Subsections: []DraftSubsection{}}}})
	if err != nil {
		t.Fatal(err)
	}
	id := created.Created["page"]
	outlineBefore := string(workspace[".outline.json"])
	workspace[model.SlideHTMLPath(id)] = []byte("existing HTML")
	if _, err := engine.Apply(Request{Op: "slide.spec.write", SlideID: id, Spec: json.RawMessage(`{"key_message":"Message","elements":[]}`)}); err != nil {
		t.Fatal(err)
	}
	for _, patch := range []Patch{{Op: "add", Path: "/role", Value: "context"}, {Op: "replace", Path: "/role", Value: "conclusion"}, {Op: "remove", Path: "/role"}} {
		before, _ := spec.ReadSlideSpec(workspace.Read, id)
		if _, err := engine.Apply(Request{Op: "slide.spec.patch", SlideID: id, ExpectedHash: spec.ResourceBytesHash(before), Patch: []Patch{patch}}); err != nil {
			t.Fatal(err)
		}
		after, err := spec.ReadSlideSpec(workspace.Read, id)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(after, &fields); err != nil {
			t.Fatal(err)
		}
		if fields["role"] != patch.Value {
			t.Fatalf("role after %s: %v", patch.Op, fields)
		}
		if string(workspace[".outline.json"]) != outlineBefore || string(workspace[model.SlideHTMLPath(id)]) != "existing HTML" {
			t.Fatal("role edit changed Outline or HTML")
		}
	}
	for _, role := range []any{"unknown", "", nil} {
		before := string(workspace[model.SpecCollectionPath])
		if _, err := engine.Apply(Request{Op: "slide.spec.patch", SlideID: id, Patch: []Patch{{Op: "add", Path: "/role", Value: role, ValueSet: true}}}); err == nil {
			t.Fatalf("invalid role accepted: %v", role)
		}
		if string(workspace[model.SpecCollectionPath]) != before {
			t.Fatal("invalid role modified Spec")
		}
	}
	if _, err := engine.Apply(Request{Op: "outline.update", NodeID: id, Changes: map[string]any{"role": "cover"}}); err == nil {
		t.Fatal("Outline still accepts role updates")
	}
}

type failAfterWrite struct {
	Workspace
	fail bool
}

func (w *failAfterWrite) Write(path string, raw []byte) error {
	if err := w.Workspace.Write(path, raw); err != nil {
		return err
	}
	if w.fail {
		w.fail = false
		return errors.New("durability failure after replacement")
	}
	return nil
}

func TestBufferRestoresFilesAfterPartialWriteAndMetadataFailure(t *testing.T) {
	_, workspace := mutationFixture(t)
	before := string(workspace[model.SpecCollectionPath])
	flaky := &failAfterWrite{Workspace: workspace, fail: true}
	buffer := NewBuffer(flaky)
	_ = buffer.Write(model.SpecCollectionPath, []byte(`{"changed":true}`))
	if err := buffer.Commit(); err == nil {
		t.Fatal("expected write failure")
	}
	if string(workspace[model.SpecCollectionPath]) != before {
		t.Fatal("partial replacement was not rolled back")
	}
	workspace["sli_a.html"] = []byte("old HTML")
	buffer = NewBuffer(workspace)
	_ = buffer.Write(model.SpecCollectionPath, []byte(`{}`))
	_ = buffer.Delete("sli_a.html")
	_ = buffer.Write("sli_b.html", []byte("new HTML"))
	if err := buffer.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := buffer.Rollback(); err != nil {
		t.Fatal(err)
	}
	if string(workspace[model.SpecCollectionPath]) != before || string(workspace["sli_a.html"]) != "old HTML" {
		t.Fatal("metadata rollback lost existing files")
	}
	if _, exists := workspace["sli_b.html"]; exists {
		t.Fatal("metadata rollback retained new file")
	}
}
