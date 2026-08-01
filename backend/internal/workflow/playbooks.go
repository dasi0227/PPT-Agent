package workflow

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type fixedPlaybook struct {
	key       string
	artifact  model.Artifact
	level     model.TargetLevel
	kinds     map[StepKind]bool
	caps      map[Capability]bool
	verifiers []string
	fallback  func(contextengine.ContextPack, Operation) WorkflowPlan
}

func (p fixedPlaybook) Key() string                              { return p.key }
func (p fixedPlaybook) AllowedKinds() map[StepKind]bool          { return cloneKinds(p.kinds) }
func (p fixedPlaybook) AllowedCapabilities() map[Capability]bool { return cloneCapabilities(p.caps) }
func (p fixedPlaybook) Verifiers() []string                      { return append([]string{}, p.verifiers...) }
func (p fixedPlaybook) Fallback(pack contextengine.ContextPack, op Operation) WorkflowPlan {
	return p.fallback(pack, op)
}

func PlaybookFor(spec model.WorkSpec) (Playbook, error) {
	switch {
	case spec.Target.Artifact == model.ArtifactBlueprint && spec.Target.Level == model.TargetDeck:
		return blueprintDeckPlaybook(), nil
	case spec.Target.Artifact == model.ArtifactBlueprint && spec.Target.Level == model.TargetSlide:
		return blueprintSlidePlaybook(), nil
	case spec.Target.Artifact == model.ArtifactPresentation && spec.Target.Level == model.TargetDeck:
		return presentationDeckPlaybook(), nil
	case spec.Target.Artifact == model.ArtifactPresentation && spec.Target.Level == model.TargetSlide:
		return presentationSlidePlaybook(), nil
	default:
		return nil, fmt.Errorf("unsupported playbook %s/%s", spec.Target.Artifact, spec.Target.Level)
	}
}

func DeriveOperation(pack contextengine.ContextPack) Operation {
	spec := pack.WorkSpec
	if spec.Interaction.Intent == model.IntentConsult {
		return OperationConsult
	}
	if spec.Target.Artifact == model.ArtifactBlueprint {
		if spec.Target.Level == model.TargetDeck && len(pack.Deck.Deck.SlideOrder) == 0 {
			return OperationCreate
		}
		return OperationRevise
	}
	if spec.Target.Level == model.TargetDeck {
		if len(pack.Presentation.Summaries) == 0 {
			return OperationMaterialize
		}
		return OperationRevise
	}
	if pack.Target.Materialization == nil || pack.Target.Materialization.State == string(model.MaterializationNotMaterialized) {
		return OperationMaterialize
	}
	switch pack.Target.Materialization.State {
	case string(model.MaterializationBlueprintStale), string(model.MaterializationDesignStale), string(model.MaterializationUnknown):
		return OperationRebuild
	default:
		return OperationRevise
	}
}

func blueprintDeckPlaybook() Playbook {
	return fixedPlaybook{
		key:       "blueprint/deck",
		kinds:     kindSet(StepAnalyze, StepReplaceDeck, StepPatchBlueprint, StepValidate, StepRepair, StepCommit, StepDeliver),
		caps:      capabilitySet(CapabilityReadContext, CapabilityReadBlueprint, CapabilityWriteBlueprint, CapabilityValidateBlueprint, CapabilityControl),
		verifiers: []string{"blueprint"},
		fallback: func(pack contextengine.ContextPack, op Operation) WorkflowPlan {
			if op == OperationConsult {
				return consultPlan(pack, "Analyze the deck blueprint and answer without modifying artifacts")
			}
			deck := deckRef(pack)
			slides := slideRefs(pack)
			if op == OperationCreate && len(slides) == 0 {
				count := pack.WorkSpec.Options.DesiredSlideCount
				if count <= 0 {
					count = 6
				}
				for index := 0; index < count; index++ {
					slides = append(slides, blueprintSlideRef("slide-"+uuid.NewString()))
				}
			}
			affected := append([]ArtifactRef{deck}, slides...)
			return newPlan(pack, op, affected, []WorkflowStep{
				step("analyze-deck", StepAnalyze, "Analyze narrative", pack.WorkSpec.Instruction, affected, nil,
					CapabilityReadContext, CapabilityReadBlueprint),
				step("replace-deck", StepReplaceDeck, "Create or revise deck blueprint", pack.WorkSpec.Instruction, affected, []string{"analyze-deck"},
					CapabilityReadContext, CapabilityReadBlueprint, CapabilityWriteBlueprint),
				step("patch-slides", StepPatchBlueprint, "Create or revise slide blueprints", pack.WorkSpec.Instruction, slides, []string{"replace-deck"},
					CapabilityReadContext, CapabilityReadBlueprint, CapabilityWriteBlueprint),
			}, "A coherent, schema-valid deck blueprint is committed")
		},
	}
}

func blueprintSlidePlaybook() Playbook {
	return fixedPlaybook{
		key:       "blueprint/slide",
		kinds:     kindSet(StepAnalyze, StepPatchBlueprint, StepValidate, StepRepair, StepCommit, StepDeliver),
		caps:      capabilitySet(CapabilityReadContext, CapabilityReadBlueprint, CapabilityWriteBlueprint, CapabilityValidateBlueprint, CapabilityControl),
		verifiers: []string{"blueprint"},
		fallback: func(pack contextengine.ContextPack, op Operation) WorkflowPlan {
			if op == OperationConsult {
				return consultPlan(pack, "Analyze the target slide blueprint and its neighbors")
			}
			target := blueprintSlideRef(pack.WorkSpec.Target.SlideID)
			return newPlan(pack, op, []ArtifactRef{target}, []WorkflowStep{
				step("analyze-slide", StepAnalyze, "Analyze target and neighbors", pack.WorkSpec.Instruction, []ArtifactRef{target}, nil,
					CapabilityReadContext, CapabilityReadBlueprint),
				step("patch-slide", StepPatchBlueprint, "Revise target slide blueprint", pack.WorkSpec.Instruction, []ArtifactRef{target}, []string{"analyze-slide"},
					CapabilityReadContext, CapabilityReadBlueprint, CapabilityWriteBlueprint),
			}, "The target slide remains coherent with its section and neighbors")
		},
	}
}

func presentationDeckPlaybook() Playbook {
	return fixedPlaybook{
		key:       "presentation/deck",
		kinds:     kindSet(StepAnalyze, StepApplyDesign, StepMaterializeSlide, StepReviseSlide, StepValidate, StepRepair, StepCommit, StepDeliver),
		caps:      presentationCapabilities(),
		verifiers: []string{"presentation_static", "browser", "cross_slide"},
		fallback: func(pack contextengine.ContextPack, op Operation) WorkflowPlan {
			if op == OperationConsult {
				return consultPlan(pack, "Analyze the presentation deck and answer without changing files")
			}
			presentations := presentationRefs(pack)
			design := designRef(pack)
			affected := append([]ArtifactRef{design}, presentations...)
			renderKind := StepMaterializeSlide
			title := "Materialize slides"
			if op == OperationRevise {
				renderKind, title = StepReviseSlide, "Revise affected slides"
			}
			return newPlan(pack, op, affected, []WorkflowStep{
				step("analyze-presentation", StepAnalyze, "Analyze deck and global intent", pack.WorkSpec.Instruction, affected, nil,
					CapabilityReadContext, CapabilityReadBlueprint, CapabilityReadDesignSpec, CapabilityReadPresentation),
				step("apply-design", StepApplyDesign, "Apply Design Director principles", pack.WorkSpec.Instruction, []ArtifactRef{design}, []string{"analyze-presentation"},
					CapabilityReadContext, CapabilityReadDesignSpec, CapabilityWriteDesignSpec, CapabilitySearchAssets, CapabilityReadAssets),
				step("render-slides", renderKind, title, pack.WorkSpec.Instruction, presentations, []string{"apply-design"},
					CapabilityReadContext, CapabilityReadBlueprint, CapabilityReadDesignSpec, CapabilityReadPresentation,
					CapabilityWritePresentation, CapabilitySearchAssets, CapabilityReadAssets, CapabilityMountAssets),
			}, "All affected slides pass static, browser and cross-slide verification")
		},
	}
}

func presentationSlidePlaybook() Playbook {
	return fixedPlaybook{
		key:       "presentation/slide",
		kinds:     kindSet(StepAnalyze, StepMaterializeSlide, StepReviseSlide, StepValidate, StepRepair, StepCommit, StepDeliver),
		caps:      presentationCapabilities(),
		verifiers: []string{"presentation_static", "browser", "cross_slide"},
		fallback: func(pack contextengine.ContextPack, op Operation) WorkflowPlan {
			if op == OperationConsult {
				return consultPlan(pack, "Analyze the target presentation slide without changing it")
			}
			target := presentationSlideRef(pack.WorkSpec.Target.SlideID)
			kind, title := StepReviseSlide, "Revise target slide"
			if op == OperationMaterialize || op == OperationRebuild {
				kind, title = StepMaterializeSlide, "Materialize target slide"
			}
			return newPlan(pack, op, []ArtifactRef{target}, []WorkflowStep{
				step("analyze-slide", StepAnalyze, "Analyze target presentation slide", pack.WorkSpec.Instruction, []ArtifactRef{target}, nil,
					CapabilityReadContext, CapabilityReadBlueprint, CapabilityReadDesignSpec, CapabilityReadPresentation),
				step("render-slide", kind, title, pack.WorkSpec.Instruction, []ArtifactRef{target}, []string{"analyze-slide"},
					CapabilityReadContext, CapabilityReadBlueprint, CapabilityReadDesignSpec, CapabilityReadPresentation,
					CapabilityWritePresentation, CapabilitySearchAssets, CapabilityReadAssets, CapabilityMountAssets),
			}, "The target slide passes static and isolated browser verification")
		},
	}
}

func consultPlan(pack contextengine.ContextPack, goal string) WorkflowPlan {
	return newPlan(pack, OperationConsult, []ArtifactRef{}, []WorkflowStep{
		step("analyze", StepAnalyze, "Analyze and respond", pack.WorkSpec.Instruction, targetRefs(pack), nil,
			CapabilityReadContext, CapabilityReadBlueprint, CapabilityReadDesignSpec, CapabilityReadPresentation),
	}, goal)
}

func newPlan(pack contextengine.ContextPack, op Operation, affected []ArtifactRef, steps []WorkflowStep, criterion string) WorkflowPlan {
	return WorkflowPlan{
		ID: "plan_" + uuid.NewString(), Version: 1, Goal: pack.WorkSpec.Instruction,
		Target: pack.WorkSpec.Target, Operation: op, Assumptions: []Assumption{},
		Affected: affected, Steps: steps,
		SuccessCriteria: []Criterion{{
			ID: "criterion-verified", Description: criterion, Required: true,
		}},
		Budget: DefaultBudget(),
	}
}

func step(id string, kind StepKind, title, instruction string, targets []ArtifactRef, deps []string, caps ...Capability) WorkflowStep {
	if targets == nil {
		targets = []ArtifactRef{}
	}
	if deps == nil {
		deps = []string{}
	}
	return WorkflowStep{
		ID: id, Kind: kind, Title: title, Instruction: instruction,
		Targets: targets, DependsOn: deps, Preconditions: []Condition{},
		Capabilities: caps, Verifiers: []string{}, Status: StepPending,
	}
}

func deckRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{Kind: ArtifactDeck, ID: pack.Project.ID, Path: "deck.json", Project: pack.Project.ID}
}

func designRef(pack contextengine.ContextPack) ArtifactRef {
	return ArtifactRef{Kind: ArtifactDesign, ID: pack.Project.ID, Path: "design/design-spec.json", Project: pack.Project.ID}
}

func blueprintSlideRef(id string) ArtifactRef {
	return ArtifactRef{Kind: ArtifactSlide, ID: id, Path: model.SlideJSONPath(id)}
}

func presentationSlideRef(id string) ArtifactRef {
	return ArtifactRef{Kind: ArtifactPresentation, ID: id, Path: model.SlideHTMLPath(id)}
}

func slideRefs(pack contextengine.ContextPack) []ArtifactRef {
	out := make([]ArtifactRef, 0, len(pack.Deck.Deck.SlideOrder))
	for _, id := range pack.Deck.Deck.SlideOrder {
		out = append(out, blueprintSlideRef(id))
	}
	return out
}

func presentationRefs(pack contextengine.ContextPack) []ArtifactRef {
	out := make([]ArtifactRef, 0, len(pack.Deck.Deck.SlideOrder))
	for _, id := range pack.Deck.Deck.SlideOrder {
		out = append(out, presentationSlideRef(id))
	}
	return out
}

func targetRefs(pack contextengine.ContextPack) []ArtifactRef {
	if pack.WorkSpec.Target.Level == model.TargetSlide {
		if pack.WorkSpec.Target.Artifact == model.ArtifactBlueprint {
			return []ArtifactRef{blueprintSlideRef(pack.WorkSpec.Target.SlideID)}
		}
		return []ArtifactRef{presentationSlideRef(pack.WorkSpec.Target.SlideID)}
	}
	if pack.WorkSpec.Target.Artifact == model.ArtifactBlueprint {
		return []ArtifactRef{deckRef(pack)}
	}
	return presentationRefs(pack)
}

func presentationCapabilities() map[Capability]bool {
	return capabilitySet(
		CapabilityReadContext, CapabilityReadBlueprint, CapabilityReadDesignSpec,
		CapabilityWriteDesignSpec, CapabilityReadPresentation, CapabilityWritePresentation,
		CapabilitySearchAssets, CapabilityReadAssets, CapabilityMountAssets,
		CapabilityRenderPreview, CapabilityValidatePresentation, CapabilityControl,
	)
}

func kindSet(values ...StepKind) map[StepKind]bool {
	out := make(map[StepKind]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func capabilitySet(values ...Capability) map[Capability]bool {
	out := make(map[Capability]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func cloneKinds(in map[StepKind]bool) map[StepKind]bool {
	out := make(map[StepKind]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneCapabilities(in map[Capability]bool) map[Capability]bool {
	out := make(map[Capability]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
