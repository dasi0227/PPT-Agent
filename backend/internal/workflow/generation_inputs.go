package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// StageGenerationInputs uses the frozen authoring view, never a later render's
// disk reads. It also recognizes domain files edited through run_command.
func (s *RunSession) StageGenerationInputs(pack contextengine.ContextPack) (map[string]json.RawMessage, error) {
	scope := pack.Command.Scope
	if scope.IncludeRunCreatedSlides {
		if entry, ok := s.artifacts["outline.json"]; ok && !entry.Delete {
			var outline spec.Outline
			if json.Unmarshal(entry.AfterContent, &outline) == nil && spec.ValidateOutline(outline) == nil {
				scope.SlideIDs = append([]string{}, scope.SlideIDs...)
				for _, loc := range spec.FlattenOutline(outline) {
					scope.SlideIDs = append(scope.SlideIDs, loc.Slide.SlideID)
				}
			}
		}
	}
	for path, entry := range s.artifacts {
		ref := refForPath(pack, path)
		if ref.Kind == ArtifactDerived {
			continue
		}
		entry.Ref = ref
		s.artifacts[path] = entry
		if !AllowsArtifact(scope, ref) {
			return nil, ErrTargetOutOfScope
		}
		if entry.Source != "run_command" || entry.Delete {
			continue
		}
		if ref.Kind == ArtifactSlideHTML {
			if _, err := validateHTML(entry.AfterContent); err != nil {
				return nil, err
			}
		} else {
			kind := resourceForArtifact(ref).Part
			value, err := spec.ParseStrictSourceJSON(entry.AfterContent, kind)
			if err != nil {
				return nil, fmt.Errorf("invalid %s command edit: %w", kind, err)
			}
			if outline, ok := value.(*spec.Outline); ok {
				if err := spec.ValidateOutline(*outline); err != nil {
					return nil, err
				}
			}
		}
	}
	inputs := map[string]json.RawMessage{}
	changes := s.ChangeSet()
	for _, change := range append(changes.Created, changes.Updated...) {
		if change.Artifact.Kind != ArtifactSlideHTML {
			continue
		}
		id := change.Artifact.ID
		value := pack.GenerationInputs[id]
		if value != nil && value.Valid() {
			inputs[id], _ = json.Marshal(value)
		} else {
			inputs[id] = json.RawMessage("null")
		}
	}
	s.generationInputs = inputs
	return inputs, nil
}

// generationContext advances the request's frozen view using only staged tool
// writes. Disk refreshes are for the next model request, never this response.
func (s *RunSession) generationContext(pack contextengine.ContextPack) contextengine.ContextPack {
	out := pack
	out.GenerationInputs = map[string]*spec.GenerationInputs{}
	for id, value := range pack.GenerationInputs {
		if value != nil {
			out.GenerationInputs[id] = value.Clone()
		}
	}
	for path, entry := range s.artifacts {
		ref := refForPath(pack, path)
		if entry.Delete {
			switch ref.Kind {
			case ArtifactManifest:
				out.PresentationManifest.Manifest = spec.Manifest{}
			case ArtifactDesign:
				out.Design.Design = nil
			case ArtifactSlideSpec:
				delete(out.GenerationInputs, ref.ID)
			}
			continue
		}
		if entry.Existed && entry.BeforeHash == entry.AfterHash {
			continue
		}
		switch ref.Kind {
		case ArtifactManifest:
			var value spec.Manifest
			if json.Unmarshal(entry.AfterContent, &value) == nil {
				out.PresentationManifest.Manifest = value
			}
		case ArtifactDesign:
			var value spec.Design
			if json.Unmarshal(entry.AfterContent, &value) == nil {
				out.Design.Design = &value
			}
		case ArtifactSlideSpec:
			var value spec.SlideSpec
			if json.Unmarshal(entry.AfterContent, &value) == nil {
				out.GenerationInputs[ref.ID] = &spec.GenerationInputs{Spec: value}
			}
		}
	}
	for _, value := range out.GenerationInputs {
		value.Manifest = out.PresentationManifest.Manifest
		value.Design = spec.Design{}
		if out.Design.Design != nil {
			value.Design = *out.Design.Design
		}
	}
	return out
}

func commandDomainEvidence(session *RunSession) []Evidence {
	var evidence []Evidence
	for _, change := range session.ChangeSet().All() {
		target := resourceForArtifact(change.Artifact)
		if target.Type != "deck" && target.Type != "slide" {
			continue
		}
		kind := "schema"
		if change.Artifact.Kind == ArtifactSlideHTML {
			kind = "static"
		}
		evidence = append(evidence, newEvidence(kind, target, change.AfterHash, map[string]any{"valid": true}))
	}
	return evidence
}
