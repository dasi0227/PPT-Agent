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
	case command.Scope.Object == model.ScopeObjectSpec && command.Scope.IsSinglePage():
		id = ProfileSpecSlide
	case command.Scope.Object == model.ScopeObjectSpec:
		id = ProfileSpecDeck
	case command.Scope.AllowsHTML() && command.Scope.IsSinglePage():
		id = ProfilePPTSlide
	case command.Scope.AllowsHTML() || command.Scope.AllowsGlobal():
		id = ProfilePPTDeck
	default:
		return ContextProfile{}, fmt.Errorf("unsupported context profile: %s", command.Scope.Object)
	}
	p := ContextProfile{ID: id, Required: map[SegmentKind]bool{
		SegmentPolicy: true, SegmentRunCommand: true, SegmentPresentationManifest: true, SegmentOutline: true, SegmentDesign: true,
		SegmentMemory: true, SegmentTarget: command.Scope.IsSinglePage(),
	}, Forbidden: map[SegmentKind]bool{}}
	if id == ProfileSpecDeck || id == ProfileSpecSlide {
		p.Forbidden[SegmentSlideHTML] = true
		p.Forbidden[SegmentTheme] = true
	} else {
		p.Required[SegmentTheme] = true
	}
	return p, nil
}
