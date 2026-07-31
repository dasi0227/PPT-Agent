package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

var ErrLegacyCommandDeprecated = errors.New("legacy command is deprecated")

type LegacyRunRequest struct {
	Kind        model.Kind
	Scope       model.Scope
	PageIndex   *int
	Mode        model.Mode
	Command     string
	Instruction string
	Brief       string
	SlideCount  int
	Language    string
	Theme       string
}

type LegacyRunAdapter struct {
	store store.Store
	count atomic.Uint64
}

func NewLegacyRunAdapter(s store.Store) *LegacyRunAdapter { return &LegacyRunAdapter{store: s} }

func (a *LegacyRunAdapter) Adapt(ctx context.Context, projectID string, in LegacyRunRequest) (model.WorkSpec, error) {
	if in.Scope == "" {
		in.Scope = model.ScopeCurrent
	}
	if in.Mode == "" {
		in.Mode = model.ModeNormal
	}
	if in.Scope == model.ScopeRepo {
		return model.WorkSpec{}, fmt.Errorf("%w: repo scope moved to the asset API", ErrLegacyCommandDeprecated)
	}
	spec := model.WorkSpec{
		Instruction: strings.TrimSpace(in.Instruction),
		Interaction: model.RunInteraction{Intent: model.IntentApply, Clarification: model.ClarifyWhenBlocked},
		Options:     model.RunOptions{Language: in.Language, ThemeID: in.Theme, DesiredSlideCount: in.SlideCount},
	}
	switch in.Kind {
	case model.KindOutline:
		spec.Target.Artifact = model.ArtifactBlueprint
	case model.KindGenerate, model.KindEdit:
		spec.Target.Artifact = model.ArtifactPresentation
	case model.KindCommand:
		spec.Target.Artifact = model.ArtifactPresentation
	default:
		return model.WorkSpec{}, fmt.Errorf("%w: unsupported kind %q", model.ErrInvalidWorkSpec, in.Kind)
	}
	switch in.Scope {
	case model.ScopeOverview:
		spec.Target.Level = model.TargetDeck
	case model.ScopeCurrent, model.ScopePage:
		if in.Kind == model.KindCommand {
			spec.Target.Level = model.TargetDeck
			break
		}
		if in.Kind == model.KindGenerate && in.PageIndex == nil {
			spec.Target.Level = model.TargetDeck
			break
		}
		if in.Kind == model.KindOutline {
			slides, err := a.store.ListSlides(ctx, projectID)
			if err != nil {
				return model.WorkSpec{}, err
			}
			if len(slides) == 0 {
				spec.Target.Level = model.TargetDeck
				break
			}
		}
		spec.Target.Level = model.TargetSlide
		id, err := a.resolveSlideID(ctx, projectID, in.PageIndex)
		if err != nil {
			return model.WorkSpec{}, err
		}
		spec.Target.SlideID = id
	default:
		return model.WorkSpec{}, fmt.Errorf("%w: unsupported scope %q", model.ErrInvalidWorkSpec, in.Scope)
	}
	switch in.Mode {
	case model.ModeTalk:
		spec.Interaction.Intent = model.IntentConsult
	case model.ModeAsk:
		spec.Interaction.Clarification = model.ClarifyBeforeApply
	case model.ModeNormal:
	default:
		return model.WorkSpec{}, fmt.Errorf("%w: unsupported mode %q", model.ErrInvalidWorkSpec, in.Mode)
	}
	if in.Command == "talk" {
		spec.Interaction.Intent = model.IntentConsult
	}
	if in.Command == "prompt" || in.Command == "recap" || (in.Kind == model.KindCommand && in.Command == "") {
		spec.Interaction.Intent = model.IntentConsult
	}
	if in.Command == "ask" {
		spec.Interaction.Clarification = model.ClarifyBeforeApply
	}
	if err := spec.Validate(); err != nil {
		return model.WorkSpec{}, err
	}
	n := a.count.Add(1)
	log.Printf(`{"event":"legacy_run_deprecated","count":%d,"kind":%q,"scope":%q,"mode":%q}`, n, in.Kind, in.Scope, in.Mode)
	return spec, nil
}

func (a *LegacyRunAdapter) resolveSlideID(ctx context.Context, projectID string, index *int) (string, error) {
	if index == nil {
		return "", ErrInvalidPageIndex
	}
	slides, err := a.store.ListSlides(ctx, projectID)
	if err != nil {
		return "", err
	}
	if *index < 0 || *index >= len(slides) {
		return "", ErrInvalidPageIndex
	}
	return slides[*index].ID, nil
}

func (a *LegacyRunAdapter) DeprecationCount() uint64 { return a.count.Load() }

// legacyPersistenceProjection exists solely for the constraints on the v1
// runs table. It is not used for routing or capability decisions; WorkSpec is
// the authoritative persisted protocol.
func legacyPersistenceProjection(spec model.WorkSpec, pageIndex *int) (model.Kind, model.Scope, model.Mode, *int) {
	kind := model.KindGenerate
	if spec.Target.Artifact == model.ArtifactBlueprint {
		kind = model.KindOutline
	}
	scope := model.ScopeOverview
	if spec.Target.Level == model.TargetSlide {
		scope = model.ScopePage
	}
	mode := model.ModeNormal
	if spec.Interaction.Intent == model.IntentConsult {
		mode = model.ModeTalk
	}
	return kind, scope, mode, pageIndex
}
