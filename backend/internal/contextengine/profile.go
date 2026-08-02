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

func (ContextProfileResolver) Resolve(spec model.WorkSpec) (ContextProfile, error) {
	var id ProfileID
	switch {
	case spec.Target.Artifact == model.ArtifactSpec && spec.Target.Level == model.TargetDeck:
		id = ProfileSpecDeck
	case spec.Target.Artifact == model.ArtifactSpec && spec.Target.Level == model.TargetSlide:
		id = ProfileSpecSlide
	case spec.Target.Artifact == model.ArtifactPresentation && spec.Target.Level == model.TargetDeck:
		id = ProfilePresentationDeck
	case spec.Target.Artifact == model.ArtifactPresentation && spec.Target.Level == model.TargetSlide:
		id = ProfilePresentationSlide
	default:
		return ContextProfile{}, fmt.Errorf("unsupported context profile: %s/%s", spec.Target.Artifact, spec.Target.Level)
	}
	p := ContextProfile{ID: id, Required: map[SegmentKind]bool{
		SegmentPolicy: true, SegmentWorkSpec: true, SegmentOutline: true, SegmentDesign: true,
		SegmentMemory: true, SegmentTarget: spec.Target.Level == model.TargetSlide,
	}, Forbidden: map[SegmentKind]bool{}}
	if id == ProfileSpecDeck || id == ProfileSpecSlide {
		p.Forbidden[SegmentSlideHTML] = true
	}
	return p, nil
}
