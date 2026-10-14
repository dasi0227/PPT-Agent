package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
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
		if ref.Kind == ArtifactSlideSpec && !entry.Delete {
			if _, err := spec.ParseStrictSourceJSON(entry.AfterContent, "spec"); err != nil {
				return err
			}
			outlineRaw, err := s.ReadPath(".outline.json")
			if err != nil {
				if entry.Source == "edit_spec" {
					return &pptmutation.ResourceSourceError{Resource: "outline", Err: err}
				}
				return toolResultError(resourceReadFailure(err, Resource{Type: "deck", Part: "outline"}))
			}
			parsed, err := spec.ParseStrictSourceJSON(outlineRaw, "outline")
			if err != nil {
				if entry.Source == "edit_spec" {
					return &pptmutation.ResourceSourceError{Resource: "outline", Err: err}
				}
				return toolResultError(savedResourceInvalid(err))
			}
			if _, exists := spec.FindSlide(*parsed.(*spec.Outline), ref.ID); !exists {
				return toolResultError(resourceMutationFailure(pptmutation.ErrSlideNotFound, resourceForArtifact(ref)))
			}
			s.requireSlide(ref.ID)
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
