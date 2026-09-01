package contextengine

import (
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ContextProfile struct {
	ID        ProfileID
	Required  map[SegmentKind]bool
	Forbidden map[SegmentKind]bool
}

type ContextProfileResolver struct{}

func (ContextProfileResolver) Resolve(command model.RunCommand) (ContextProfile, error) {
	var id ProfileID
	switch {
	case command.Scope.Artifact == model.ArtifactSpec && command.Scope.Level == model.ScopeDeck:
		id = ProfileSpecDeck
	case command.Scope.Artifact == model.ArtifactSpec && command.Scope.Level == model.ScopeSlide:
		id = ProfileSpecSlide
	case command.Scope.Artifact == model.ArtifactPPT && command.Scope.Level == model.ScopeDeck:
		id = ProfilePPTDeck
	case command.Scope.Artifact == model.ArtifactPPT && command.Scope.Level == model.ScopeSlide:
		id = ProfilePPTSlide
	default:
		return ContextProfile{}, fmt.Errorf("unsupported context profile: %s/%s", command.Scope.Artifact, command.Scope.Level)
	}
	p := ContextProfile{ID: id, Required: map[SegmentKind]bool{
		SegmentPolicy: true, SegmentRunCommand: true, SegmentPresentationManifest: true, SegmentOutline: true, SegmentDesign: true,
		SegmentMemory: true, SegmentTarget: command.Scope.Level == model.ScopeSlide,
	}, Forbidden: map[SegmentKind]bool{}}
	if id == ProfileSpecDeck || id == ProfileSpecSlide {
		p.Forbidden[SegmentSlideHTML] = true
		p.Forbidden[SegmentTheme] = true
	} else {
		p.Required[SegmentTheme] = true
	}
	return p, nil
}
