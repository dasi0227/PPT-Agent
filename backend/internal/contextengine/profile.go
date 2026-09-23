package contextengine

import (
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
	if command.Scope.IsSinglePage() {
		id = ProfilePPTSlide
	} else {
		id = ProfilePPTDeck
	}
	p := ContextProfile{ID: id, Required: map[SegmentKind]bool{
		SegmentPolicy: true, SegmentRunCommand: true, SegmentPresentationManifest: true, SegmentOutline: true, SegmentDesign: true,
		SegmentTarget: command.Scope.IsSinglePage(),
	}, Forbidden: map[SegmentKind]bool{}}
	p.Required[SegmentTheme] = true
	return p, nil
}
