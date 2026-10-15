package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type SlideState struct {
	Spec   string `json:"spec"`
	HTML   string `json:"html"`
	Render string `json:"render"`
}
type ProjectChange struct {
	Target string `json:"target"`
	Type   string `json:"type"`
}
type ProjectState struct {
	Outline  string                `json:"outline"`
	Manifest string                `json:"manifest"`
	Design   string                `json:"design"`
	Slides   map[string]SlideState `json:"slides"`
	Changes  []ProjectChange       `json:"changes"`
	// Current saved versions only; no source bodies are retained or injected.
	// Request preparation rechecks availability after history projection.
	sourceVersions map[string]string
}

func (s ProjectState) withAvailableResources(messages []llm.Message) ProjectState {
	if s.sourceVersions == nil {
		return s
	}
	available := availableResourceHashes(messages, s.sourceVersions)
	status := func(resource Resource, previous string) string {
		if previous == "missing" {
			return previous
		}
		key := "ppt/" + resource.Key()
		if hash := s.sourceVersions[key]; hash != "" && available[key] == hash {
			return "fresh"
		}
		return "stale"
	}
	s.Outline = status(Resource{Type: "deck", Part: "outline"}, s.Outline)
	s.Manifest = status(Resource{Type: "deck", Part: "manifest"}, s.Manifest)
	s.Design = status(Resource{Type: "deck", Part: "design"}, s.Design)
	slides := make(map[string]SlideState, len(s.Slides))
	for id, slide := range s.Slides {
		slide.Spec = status(Resource{Type: "slide", Part: "spec", SlideID: id}, slide.Spec)
		slide.HTML = status(Resource{Type: "slide", Part: "html", SlideID: id}, slide.HTML)
		slides[id] = slide
	}
	s.Slides = slides
	return s
}

// Sources are internal: no source body or hash is projected into project_state.
// The same run-start baseline is reused after resume, never reset to current disk.
type projectSources struct {
	files    map[string][]byte
	outline  spec.Outline
	manifest spec.Manifest
	design   spec.Design
	specs    map[string]json.RawMessage
	slides   map[string]spec.SlideNode
}

func readProjectSources(read func(string) ([]byte, error)) (*projectSources, error) {
	s := &projectSources{files: map[string][]byte{}, specs: map[string]json.RawMessage{}, slides: map[string]spec.SlideNode{}}
	for _, part := range []string{"outline", "manifest", "design"} {
		name := "." + part + ".json"
		raw, err := read(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := spec.ParseStrictSourceJSON(raw, part); err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		s.files[name] = raw
		switch part {
		case "outline":
			err = json.Unmarshal(raw, &s.outline)
			if err == nil {
				err = spec.ValidateOutline(s.outline)
			}
		case "manifest":
			err = json.Unmarshal(raw, &s.manifest)
		case "design":
			err = json.Unmarshal(raw, &s.design)
		}
		if err != nil {
			return nil, err
		}
	}
	for _, loc := range spec.FlattenOutline(s.outline) {
		id := loc.Slide.ID
		s.slides[id] = loc.Slide
		specPath := model.SlideSpecPath(id)
		specRaw, err := read(specPath)
		if err == nil {
			// Availability is per page, including damaged sources that read_resource
			// will report as invalid; another page must remain accessible.
			s.specs[id] = specRaw
			s.files[specPath] = specRaw
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read %s: %w", specPath, err)
		}
		name := model.SlideHTMLPath(id)
		raw, err := read(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		s.files[name] = raw
	}
	return s, nil
}

func (s *projectSources) state(images []RenderedImageContext, baseline *projectSources, available map[string]string) ProjectState {
	status := func(resource Resource, name string) string {
		raw, exists := s.files[name]
		if !exists {
			return "missing"
		}
		hash := resourceContentHash(resource, raw)
		if hash != "" && available["ppt/"+resource.Key()] == hash {
			return "fresh"
		}
		return "stale"
	}
	out := ProjectState{
		Outline:  status(Resource{Type: "deck", Part: "outline"}, ".outline.json"),
		Manifest: status(Resource{Type: "deck", Part: "manifest"}, ".manifest.json"),
		Design:   status(Resource{Type: "deck", Part: "design"}, ".design.json"),
		Slides:   map[string]SlideState{}, Changes: []ProjectChange{},
	}
	rendered := map[string]string{}
	for _, image := range images {
		rendered[image.SlideID] = "fresh"
		if image.Stale {
			rendered[image.SlideID] = "stale"
		}
	}
	for id := range s.slides {
		render := rendered[id]
		if render == "" {
			render = "missing"
		}
		out.Slides[id] = SlideState{
			Spec:   status(Resource{Type: "slide", Part: "spec", SlideID: id}, model.SlideSpecPath(id)),
			HTML:   status(Resource{Type: "slide", Part: "html", SlideID: id}, model.SlideHTMLPath(id)),
			Render: render,
		}
	}
	if baseline != nil {
		out.Changes = projectNetChanges(baseline, s)
	}
	return out
}

func projectNetChanges(before, after *projectSources) []ProjectChange {
	changes := []ProjectChange{}
	kind := func(had, has bool) string {
		if !had {
			return "created"
		}
		if !has {
			return "deleted"
		}
		return "updated"
	}
	// Compare JSON values, so formatting-only round trips do not invent changes.
	equalJSON := func(a, b []byte) bool {
		var old, next any
		if len(a) == 0 || len(b) == 0 {
			return len(a) == len(b)
		}
		if json.Unmarshal(a, &old) != nil || json.Unmarshal(b, &next) != nil {
			return string(a) == string(b)
		}
		return reflect.DeepEqual(old, next)
	}
	for _, part := range []string{"outline", "manifest", "design"} {
		name := "." + part + ".json"
		a, had := before.files[name]
		b, has := after.files[name]
		if had != has || !equalJSON(a, b) {
			changes = append(changes, ProjectChange{part, kind(had, has)})
		}
	}
	ids := map[string]bool{}
	for id := range before.slides {
		ids[id] = true
	}
	for id := range after.slides {
		ids[id] = true
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		a, had := before.slides[id]
		b, has := after.slides[id]
		name := model.SlideHTMLPath(id)
		_, oldSpec := before.specs[id]
		_, newSpec := after.specs[id]
		_, oldHTML := before.files[name]
		_, newHTML := after.files[name]
		if had != has || !reflect.DeepEqual(a, b) || oldSpec != newSpec || !equalJSON(before.specs[id], after.specs[id]) || oldHTML != newHTML || string(before.files[name]) != string(after.files[name]) {
			changes = append(changes, ProjectChange{id, kind(had, has)})
		}
	}
	return changes
}

func (state *RunState) refreshProjectState() error {
	if state.projectBaseline == nil {
		if state.reviewBaselineError != "" {
			return errors.New(state.reviewBaselineError)
		}
		raw, err := os.ReadFile(reviewBaselinePath(state.projectDir, state.runID))
		if err != nil {
			return err
		}
		var files map[string]reviewSourceFile
		if err := json.Unmarshal(raw, &files); err != nil {
			return err
		}
		baseline, err := readProjectSources(func(name string) ([]byte, error) {
			if file, ok := files[name]; ok {
				return []byte(file.Content), nil
			}
			return nil, os.ErrNotExist
		})
		if err != nil {
			return fmt.Errorf("run-start project sources: %w", err)
		}
		state.projectBaseline = baseline
	}
	release := pptmutation.ReadLockProject(state.projectDir)
	sources, err := readProjectSources(func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(state.projectDir, name)) })
	release()
	if err != nil {
		return err
	}
	// Refresh actual source material before render proof checks.
	state.pack.Outline.Outline = sources.outline
	state.pack.PresentationManifest.Manifest = sources.manifest
	state.pack.Design.Design = nil
	if _, exists := sources.files[".design.json"]; exists {
		state.pack.Design.Design = &sources.design
	}
	contextengine.RefreshPageContext(&state.pack, state.projectDir, nil, true)
	state.renderedImages, err = latestRenderedImages(state.pack, state.projectDir, state.tx)
	if err != nil {
		return err
	}
	state.readImages = currentRunImages(state.readImages, state.renderedImages)
	current := map[string]string{}
	for path, raw := range sources.files {
		for _, stamp := range commandVersionMetadata(path, raw).Resources {
			current[stamp.Key] = stamp.Hash
		}
	}
	state.seenVersions = availableResourceHashes(state.messages, current)
	state.projectState = sources.state(state.renderedImages, state.projectBaseline, state.seenVersions)
	state.projectState.sourceVersions = current
	return nil
}
