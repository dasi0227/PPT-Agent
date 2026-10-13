package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// ValidateStagedArtifacts checks page scope and structured command edits before
// committing the staged files.
func (s *RunSession) ValidateStagedArtifacts(pack contextengine.ContextPack) error {
	scope := pack.Command.Scope
	if scope.IncludeRunCreatedSlides {
		if entry, ok := s.artifacts[".outline.json"]; ok && !entry.Delete {
			var outline spec.Outline
			if json.Unmarshal(entry.AfterContent, &outline) == nil && spec.ValidateOutline(outline) == nil {
				scope.SlideIDs = append([]string{}, scope.SlideIDs...)
				for _, loc := range spec.FlattenOutline(outline) {
					scope.SlideIDs = append(scope.SlideIDs, loc.Slide.ID)
				}
			}
		}
	}
	for path, entry := range s.artifacts {
		if path == model.SpecCollectionPath {
			if entry.Delete {
				return fmt.Errorf("cannot delete spec collection")
			}
			entries, err := spec.ParseCollection(entry.AfterContent)
			if err != nil {
				return err
			}
			outlineRaw, err := s.ReadPath(".outline.json")
			if err != nil {
				return toolResultError(resourceReadFailure(err, Resource{Type: "deck", Part: "outline"}))
			}
			parsed, err := spec.ParseStrictSourceJSON(outlineRaw, "outline")
			if err != nil {
				return toolResultError(savedResourceInvalid(err))
			}
			outline := *parsed.(*spec.Outline)
			members := map[string]bool{}
			for _, loc := range spec.FlattenOutline(outline) {
				members[loc.Slide.ID] = true
			}
			for _, change := range s.ChangeSet().All() {
				if change.Artifact.Kind != ArtifactSlideSpec {
					continue
				}
				if !AllowsArtifact(scope, change.Artifact) {
					return ErrTargetOutOfScope
				}
				if _, exists := entries[change.Artifact.ID]; exists && !members[change.Artifact.ID] {
					return fmt.Errorf("spec page %s is not in outline", change.Artifact.ID)
				}
				if _, exists := entries[change.Artifact.ID]; exists {
					s.requireSlide(change.Artifact.ID)
				}
			}
			continue
		}
		ref := refForPath(pack, path)
		if ref.Kind == ArtifactDerived {
			continue
		}
		entry.Ref = ref
		s.artifacts[path] = entry
		if !AllowsArtifact(scope, ref) {
			return ErrTargetOutOfScope
		}
		if entry.Source == "run_command" && ref.Kind == ArtifactOutline && entry.Delete {
			return fmt.Errorf("run_command cannot delete outline; use edit_outline to remove pages")
		}
		if entry.Source != "run_command" || entry.Delete {
			continue
		}
		if ref.Kind == ArtifactSlideHTML {
			s.requireSlide(ref.ID)
			if _, err := validateHTML(entry.AfterContent); err != nil {
				return err
			}
		} else {
			kind := resourceForArtifact(ref).Part
			value, err := spec.ParseStrictSourceJSON(entry.AfterContent, kind)
			if err != nil {
				return fmt.Errorf("invalid %s command edit: %w", kind, err)
			}
			if outline, ok := value.(*spec.Outline); ok {
				if err := spec.ValidateOutline(*outline); err != nil {
					return err
				}
				var before spec.Outline
				if err := json.Unmarshal(entry.BeforeContent, &before); err != nil {
					return fmt.Errorf("cannot check outline page membership before command edit: %w", err)
				}
				members := map[string]bool{}
				for _, loc := range spec.FlattenOutline(*outline) {
					members[loc.Slide.ID] = true
				}
				for _, loc := range spec.FlattenOutline(before) {
					if !members[loc.Slide.ID] {
						return fmt.Errorf("run_command cannot remove page %s from outline; use edit_outline", loc.Slide.ID)
					}
				}
			}
		}
	}
	return nil
}

// authoringContext advances the response's frozen view using staged resource
// writes committed by earlier calls, without rereading later disk content.
func (s *RunSession) authoringContext(pack contextengine.ContextPack) contextengine.ContextPack {
	out := pack
	for path, entry := range s.artifacts {
		ref := refForPath(pack, path)
		if entry.Delete {
			switch ref.Kind {
			case ArtifactManifest:
				out.PresentationManifest.Manifest = spec.Manifest{}
			case ArtifactDesign:
				out.Design.Design = nil
			}
			continue
		}
		if entry.Existed && entry.BeforeHash == entry.AfterHash {
			continue
		}
		switch ref.Kind {
		case ArtifactOutline:
			var value spec.Outline
			if json.Unmarshal(entry.AfterContent, &value) == nil {
				out.Outline.Outline = value
			}
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
