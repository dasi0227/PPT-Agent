package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
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
		if entry.Source == "run_command" && commandexec.ProtectedPath(path) {
			return fmt.Errorf("protected resource cannot be changed by run_command: %s", path)
		}
		ref := refForPath(pack, path)
		if ref.Kind == ArtifactDerived {
			continue
		}
		entry.Ref = ref
		s.artifacts[path] = entry
		if entry.Source != "run_command" && !AllowsArtifact(scope, ref) {
			return ErrTargetOutOfScope
		}
		if entry.Source == "run_command" && entry.AfterNode != nil && entry.AfterNode.Kind != "file" {
			return fmt.Errorf("PPT resources must remain regular files: %s", path)
		}
		if (ref.Kind == ArtifactSlideSpec || ref.Kind == ArtifactSlideHTML) && !entry.Delete {
			resource := resourceForArtifact(ref)
			if _, err := readPageOutline(pack, s.projectDir, s, ref.ID); err != nil {
				if entry.Source == "run_command" {
					return toolResultError(commandResourceFailure(err, resource))
				}
				return toolResultError(resourceMutationFailure(err, resource))
			}
			s.requireSlide(ref.ID)
			if ref.Kind == ArtifactSlideSpec {
				if _, err := spec.ParseStrictSourceJSON(entry.AfterContent, "spec"); err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
			}
		}
		if entry.Source != "run_command" || entry.Delete {
			continue
		}
		if ref.Kind == ArtifactManifest || ref.Kind == ArtifactDesign || ref.Kind == ArtifactOutline {
			if _, err := spec.ParseStrictSourceJSON(entry.AfterContent, resourceForArtifact(ref).Part); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
		if ref.Kind == ArtifactSlideHTML {
			s.requireSlide(ref.ID)
			if _, err := validateHTML(entry.AfterContent); err != nil {
				return fmt.Errorf("%s: %w", path, err)
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
			case ArtifactOutline:
				out.Outline.Outline = spec.Outline{}
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
		if entry := session.artifacts[change.Artifact.Path]; entry.Delete {
			continue
		}
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
