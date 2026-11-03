package pptmutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"io/fs"
	"testing"
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
	write(".outline.json", spec.Outline{Sections: []spec.Section{}})
	write(".meta.json", spec.Meta{Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN", Count: "待明确"})
	write(".constraints.json", spec.Constraints{Content: spec.ConstraintSet{Requirements: []string{}, Prohibitions: []string{}}, Visual: spec.ConstraintSet{Requirements: []string{"minimal"}, Prohibitions: []string{}}})
	sequence := 0
	service := &Service{Workspace: workspace, NewID: func(prefix string) string { sequence++; return fmt.Sprintf("%s_%06d", prefix, sequence) }, ValidateHTML: func(raw []byte) error {
		if len(raw) == 0 {
			return errors.New("empty")
		}
		return nil
	}}
	return service, workspace
}

func TestTypedMutationsUseStableAnchorsAndAtomicPatchValidation(t *testing.T) {
	service, workspace := mutationFixture(t)
	init, err := service.Apply(Request{Op: "outline.init", Structure: []DraftSection{{ClientRef: "sec", Title: "Section", Slides: []DraftSlide{{ClientRef: "one", Title: "One"}, {ClientRef: "two", Title: "Two"}}, Subsections: []DraftSubsection{}}}})
	if err != nil {
		t.Fatal(err)
	}
	one, two, section := init.Created["one"], init.Created["two"], init.Created["sec"]
	if _, err = service.Apply(Request{Op: "outline.move", NodeID: two, Position: Position{ParentID: section, BeforeID: one}}); err != nil {
		t.Fatal(err)
	}
	var outline spec.Outline
	raw, _ := workspace.Read(".outline.json")
	_ = json.Unmarshal(raw, &outline)
	if got := spec.FlattenOutline(outline); got[0].Slide.ID != two {
		t.Fatalf("move order=%v", got)
	}

	write := Request{Op: "slide.spec.write", SlideID: one, Spec: json.RawMessage(`{"purpose":"content_explanation","core":"Message","elements":[{"type":"text","intent":"Explain"}],"layout":"hero"}`)}
	if _, err = service.Apply(write); err != nil {
		t.Fatal(err)
	}
	before, _ := spec.ReadSlideSpec(workspace.Read, one)
	_, err = service.Apply(Request{Op: "slide.spec.patch", SlideID: one, Patch: []Patch{{Op: "replace", Path: "/core", Value: "Changed"}, {Op: "replace", Path: "/slide_id", Value: "forbidden"}}})
	if err == nil {
		t.Fatal("runtime-managed patch path must fail")
	}
	after, _ := spec.ReadSlideSpec(workspace.Read, one)
	if string(before) != string(after) {
		t.Fatal("failed patch was not atomic")
	}
}

func TestContentHashPreconditionAndNoop(t *testing.T) {
	service, workspace := mutationFixture(t)
	original := string(workspace[".meta.json"])
	hash := spec.ResourceBytesHash([]byte(original))
	request := Request{Op: "meta.patch", ExpectedHash: hash, Patch: []Patch{{Op: "replace", Path: "/title", Value: "Deck"}}}
	result, err := service.Apply(request)
	if err != nil || result.Hashes["meta"] != hash || string(workspace[".meta.json"]) != original {
		t.Fatalf("no-op changed content: %+v %v", result, err)
	}
	request.Patch[0].Value = "New title"
	result, err = service.Apply(request)
	if err != nil || result.Hashes["meta"] == hash {
		t.Fatalf("content change missing: %+v %v", result, err)
	}
	saved := string(workspace[".meta.json"])
	request.Patch[0].Value = "Stale title"
	if _, err = service.Apply(request); !errors.Is(err, ErrContentConflict) {
		t.Fatalf("stale write accepted: %v", err)
	}
	if string(workspace[".meta.json"]) != saved {
		t.Fatal("rejected write changed content")
	}
}
