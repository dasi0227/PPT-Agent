package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type toolDependencies struct {
	success []int
	wait    []int
	reads   []Resource
	writes  []Resource
	files   []string
	barrier bool
	// nil means the outline's page effects could not be established reliably.
	outlinePages map[string]bool
}

func planToolDependencies(directory string, calls []llm.ToolCall, decisions []*ToolDecision) []toolDependencies {
	plan := make([]toolDependencies, len(calls))
	release := pptmutation.ReadLockProject(directory)
	outlineSource, _ := os.ReadFile(filepath.Join(directory, ".outline.json"))
	release()
	for i, call := range calls {
		p := &plan[i]
		resource := resourceForTool(call.Name, stringValue(call.Args["slide_id"]))
		switch {
		case isResourceEditTool(call.Name):
			p.writes = []Resource{resource}
			ref, _ := refForResource(contextengine.ContextPack{}, resource)
			p.files = []string{ref.Path}
			if resource.Part == "outline" {
				var candidate []byte
				if initial, ok := call.Args["init"]; ok {
					candidate, _ = json.Marshal(initial)
				} else {
					var edits []pptmutation.Edit
					raw, _ := json.Marshal(call.Args["edits"])
					_ = json.Unmarshal(raw, &edits)
					candidate, _ = pptmutation.ApplyTextEdits(outlineSource, edits)
				}
				p.outlinePages = outlineAffectedPages(outlineSource, candidate)
				// The collection is serialized for all outline preparations; page
				// failure dependencies still use the actual affected identities.
				p.files = append(p.files, model.SpecCollectionPath)
				for id := range p.outlinePages {
					p.files = append(p.files, model.SlideHTMLPath(id))
				}
				if p.outlinePages != nil {
					outlineSource = candidate
				}
			}
		case call.Name == "read_resource":
			if target, err := parseResource(call.Args); err == nil {
				p.reads = []Resource{target}
			}
		case call.Name == "render_slide":
			id := stringValue(call.Args["slide_id"])
			p.reads = []Resource{{Type: "deck", Part: "manifest"}, {Type: "deck", Part: "design"}, {Type: "deck", Part: "outline"},
				{Type: "slide", Part: "spec", SlideID: id}, {Type: "slide", Part: "html", SlideID: id}}
		case call.Name == "run_command" && decisions[i] != nil && !decisions[i].Mutates && decisions[i].Outcome == "allow":
			// Read command graphs may inspect arbitrary files. Keep them ordered
			// with writes; homogeneous read-command batches remain concurrent.
			p.barrier = !batchIsAllowedCommands(calls, decisions, make([]bool, len(calls)))
		default:
			// Lifecycle tools and unanalyzable writes retain a serial boundary.
			p.barrier = true
		}
		for j := 0; j < i; j++ {
			previous := plan[j]
			if p.barrier || previous.barrier {
				p.wait = append(p.wait, j)
				continue
			}
			requiresSuccess := false
			for _, written := range previous.writes {
				for _, target := range p.writes {
					if written.Key() == target.Key() || target.Type == "slide" &&
						(written.Part == "manifest" || written.Part == "design" ||
							written.Part == "spec" && target.Part == "html" && written.SlideID == target.SlideID ||
							written.Part == "outline" && previous.outlinePages[target.SlideID]) {
						requiresSuccess = true
					}
					if written.Part == "outline" && target.Type == "slide" && previous.outlinePages == nil {
						p.wait = append(p.wait, j)
					}
				}
				for _, target := range p.reads {
					if written.Part == "outline" && call.Name == "render_slide" && target.Part == "outline" {
						p.wait = append(p.wait, j)
						continue
					}
					if written.Key() == target.Key() || written.Part == "outline" &&
						(target.Type == "slide" && (previous.outlinePages == nil || previous.outlinePages[target.SlideID])) {
						if call.Name == "render_slide" && previous.outlinePages != nil || call.Name == "render_slide" && written.Part != "outline" {
							requiresSuccess = true
						} else {
							p.wait = append(p.wait, j)
						}
					}
				}
			}
			// A read preceding a write must finish before the write starts, but
			// a failed diagnostic read does not forbid an otherwise known edit.
			for _, read := range previous.reads {
				for _, written := range p.writes {
					if read.Key() == written.Key() || written.Part == "outline" && read.Type == "slide" &&
						(p.outlinePages == nil || p.outlinePages[read.SlideID]) {
						p.wait = append(p.wait, j)
					}
				}
			}
			if sharesFile(previous.files, p.files) {
				p.wait = append(p.wait, j)
			}
			// Unknown outline edits also wait for earlier page writes, even
			// when their physical paths cannot yet be enumerated.
			if resource.Part == "outline" && p.outlinePages == nil {
				for _, target := range previous.writes {
					if target.Type == "slide" {
						p.wait = append(p.wait, j)
					}
				}
			}
			if requiresSuccess {
				p.success = append(p.success, j)
			}
		}
		slices.Sort(p.wait)
		p.wait = slices.Compact(p.wait)
	}
	return plan
}

func sharesFile(a, b []string) bool {
	for _, path := range a {
		if slices.Contains(b, path) {
			return true
		}
	}
	return false
}

// Preview text replacements without assigning any new identity or staging data.
func outlineAffectedPages(before, after []byte) map[string]bool {
	var old, next spec.Outline
	if len(before) == 0 {
		before = []byte(`{"sections":[]}`)
	}
	if json.Unmarshal(before, &old) != nil || json.Unmarshal(after, &next) != nil || next.Sections == nil {
		return nil
	}
	out := map[string]bool{}
	for _, loc := range spec.FlattenOutline(old) {
		id := loc.Slide.SlideID
		if spec.SemanticSlideNodeHash(old, id) != spec.SemanticSlideNodeHash(next, id) {
			out[id] = true
		}
	}
	for _, loc := range spec.FlattenOutline(next) {
		id := loc.Slide.SlideID
		if id != "" && spec.SemanticSlideNodeHash(old, id) != spec.SemanticSlideNodeHash(next, id) {
			out[id] = true
		}
	}
	return out
}

func dependencyFailure(call, predecessor llm.ToolCall, resources []Resource) ToolResult {
	return detailedToolFailure(CodeDependencyFailed, "This call was not executed because its prerequisite did not commit successfully.", map[string]any{
		"dependency_call_id": predecessor.ID, "dependency_tool": predecessor.Name,
		"dependency_resources": resources,
		"next_action":          "Inspect the failed call " + predecessor.ID + " (" + predecessor.Name + "), repair its resource using current content, then regenerate this dependent call.",
	})
}

// Runtime refreshes replace or mutate nested maps; workers get a separate view.
func cloneAuthoringPack(pack contextengine.ContextPack) contextengine.ContextPack {
	raw, _ := json.Marshal(pack)
	var out contextengine.ContextPack
	_ = json.Unmarshal(raw, &out)
	out.Project.ThemeID = pack.Project.ThemeID
	for index := range out.Command.Components {
		out.Command.Components[index].LocalPath = pack.Command.Components[index].LocalPath
	}
	out.RefResolver = pack.RefResolver
	out.GenerationInputs = map[string]*spec.GenerationInputs{}
	out.GenerationBaselines = map[string]*spec.GenerationInputs{}
	for id, value := range pack.GenerationInputs {
		if value != nil {
			out.GenerationInputs[id] = value.Clone()
		}
	}
	for id, value := range pack.GenerationBaselines {
		if value != nil {
			out.GenerationBaselines[id] = value.Clone()
		}
	}
	return out
}

func rememberResultVersions(versions map[string]string, result ToolResult) {
	if result.OK && result.ObservationMetadata != nil {
		for _, stamp := range result.ObservationMetadata.Resources {
			versions[stamp.Key] = stamp.Hash
		}
	}
}

func (s *RunSession) generatedOutlineIdentities() bool {
	entry, exists := s.artifacts[".outline.json"]
	if !exists {
		return false
	}
	identities := func(raw []byte) map[string]bool {
		var outline spec.Outline
		_ = json.Unmarshal(raw, &outline)
		ids := map[string]bool{}
		for _, section := range outline.Sections {
			ids[section.ID] = true
			for _, sub := range section.Subsections {
				ids[sub.ID] = true
			}
		}
		for _, page := range spec.FlattenOutline(outline) {
			ids[page.Slide.SlideID] = true
		}
		return ids
	}
	before := identities(entry.BeforeContent)
	for id := range identities(entry.AfterContent) {
		if !before[id] {
			return true
		}
	}
	return false
}

// A durable replay advances authoring requirements from the result actually
// returned to the model, never by rereading newer requirements from disk.
func replayAuthoringView(directory, runID string, call llm.ToolCall, result ToolResult) *RunSession {
	if !result.OK || !isResourceEditTool(call.Name) || call.Name == "edit_html" {
		return nil
	}
	resource := resourceForTool(call.Name, stringValue(call.Args["slide_id"]))
	ref, _ := refForResource(contextengine.ContextPack{}, resource)
	raw, _ := json.Marshal(result.Data["content"])
	if resource.Part == "outline" {
		raw = []byte(stringValue(result.Data["content"]))
	}
	if resource.Part == "spec" {
		raw, _ = json.Marshal(map[string]json.RawMessage{resource.SlideID: raw})
	}
	return &RunSession{projectDir: directory, runID: runID, artifacts: map[string]sessionArtifact{
		ref.Path: {Ref: ref, Relative: ref.Path, Source: call.Name, AfterContent: raw, AfterHash: hashBytes(raw)},
	}}
}
