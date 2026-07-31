package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

var ErrRunTargetUnsupported = errors.New("run target unsupported")

type TargetKey struct {
	Artifact model.Artifact
	Level    model.TargetLevel
}

type revisionTrackingRunner struct {
	inner            run.Runner
	spec             model.WorkSpec
	project          model.Project
	store            store.Store
	blueprint        *BlueprintService
	runID            string
	legacyDesignOnly bool
}

func (r *revisionTrackingRunner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, prompter run.Prompter) harness.Outcome {
	var before blueprintRevisions
	if r.spec.Interaction.Intent == model.IntentApply && r.store != nil {
		before = r.currentRevisions(ctx)
	}
	outcome := r.inner.Run(ctx, em, cp, prompter)
	if outcome.Status != harness.OutcomeFinished || r.spec.Interaction.Intent != model.IntentApply {
		return outcome
	}
	view, err := r.blueprint.EnsureProject(ctx, r.project.ID)
	if err != nil {
		return harness.Outcome{Status: harness.OutcomeCircuit, Code: "REVISION_COMMIT_FAILED", Message: err.Error()}
	}
	if outcome.Result == nil {
		outcome.Result = map[string]any{}
	}
	outcome.Result["revisions"] = map[string]any{
		"before": before,
		"after":  revisionsOf(view, r.spec.Target.SlideID),
	}
	if r.spec.Target.Artifact != model.ArtifactPresentation {
		if r.spec.Target.Level == model.TargetSlide {
			outcome.Result["materialization_status"] = view.States[r.spec.Target.SlideID].State
		}
		return outcome
	}
	if r.legacyDesignOnly {
		if outcome.Result == nil {
			outcome.Result = map[string]any{}
		}
		outcome.Result["materialization_status"] = "design_stale"
		return outcome
	}
	slides, err := r.store.ListSlides(ctx, r.project.ID)
	if err != nil {
		return harness.Outcome{Status: harness.OutcomeCircuit, Code: "REVISION_COMMIT_FAILED", Message: err.Error()}
	}
	for _, slide := range slides {
		if r.spec.Target.Level == model.TargetSlide && slide.ID != r.spec.Target.SlideID {
			continue
		}
		bp, ok := view.Slides[slide.ID]
		if !ok {
			continue
		}
		presentationRevision := slide.PresentationRevision + 1
		htmlPath := filepath.Join(r.project.WorkDir, filepath.FromSlash(model.SlideHTMLPath(slide.ID)))
		if raw, readErr := os.ReadFile(htmlPath); readErr == nil {
			meta := fmt.Sprintf(
				`<meta name="ppt-presentation-revision" content="%d"><meta name="ppt-source-deck-revision" content="%d"><meta name="ppt-source-blueprint-revision" content="%d"><meta name="ppt-source-design-revision" content="%d"><meta name="ppt-generated-by-run" content="%s"><meta name="ppt-generated-at" content="%d">`,
				presentationRevision, view.Deck.Revision, bp.Revision, view.DesignSpec.Revision, r.runID, time.Now().Unix(),
			)
			html := string(raw)
			if strings.Contains(html, "<head>") {
				html = strings.Replace(html, "<head>", "<head>"+meta, 1)
			}
			if err := atomicWrite(htmlPath, []byte(html)); err != nil {
				return harness.Outcome{Status: harness.OutcomeCircuit, Code: "REVISION_COMMIT_FAILED", Message: err.Error()}
			}
		}
		if err := r.store.UpdateSlideRevisions(ctx, slide.ID, bp.Revision, presentationRevision, view.Deck.Revision, bp.Revision, view.DesignSpec.Revision); err != nil {
			return harness.Outcome{Status: harness.OutcomeCircuit, Code: "REVISION_COMMIT_FAILED", Message: err.Error()}
		}
	}
	committed, err := r.blueprint.EnsureProject(ctx, r.project.ID)
	if err != nil {
		return harness.Outcome{Status: harness.OutcomeCircuit, Code: "REVISION_COMMIT_FAILED", Message: err.Error()}
	}
	outcome.Result["materialization_status"] = "fresh"
	outcome.Result["revisions"] = map[string]any{
		"before": before,
		"after":  revisionsOf(committed, r.spec.Target.SlideID),
	}
	return outcome
}

func (r *revisionTrackingRunner) currentRevisions(ctx context.Context) blueprintRevisions {
	revisions := blueprintRevisions{
		Deck: r.project.DeckRevision, Design: r.project.DesignRevision,
	}
	if r.spec.Target.SlideID == "" {
		return revisions
	}
	if slide, err := r.store.GetSlide(ctx, r.spec.Target.SlideID); err == nil {
		revisions.Slide = slide.BlueprintRevision
		revisions.Presentation = slide.PresentationRevision
	}
	return revisions
}

type blueprintRevisions struct {
	Deck         int `json:"deck"`
	Design       int `json:"design"`
	Slide        int `json:"slide,omitempty"`
	Presentation int `json:"presentation,omitempty"`
}

func revisionsOf(view blueprint.ProjectView, slideID string) blueprintRevisions {
	revisions := blueprintRevisions{Deck: view.Deck.Revision, Design: view.DesignSpec.Revision}
	if slide, ok := view.Slides[slideID]; ok {
		revisions.Slide = slide.Revision
	}
	if state, ok := view.States[slideID]; ok {
		if source, ok := state.Revisions.(model.MaterializationRevisions); ok {
			revisions.Presentation = source.Presentation
		}
	}
	return revisions
}

type TargetRunnerBuilder func(model.Run, model.CreateRunParams, model.Project) run.Runner

type RunnerResolver struct {
	builders map[TargetKey]TargetRunnerBuilder
}

func NewRunnerResolver(builders map[TargetKey]TargetRunnerBuilder) *RunnerResolver {
	copy := make(map[TargetKey]TargetRunnerBuilder, len(builders))
	for key, builder := range builders {
		copy[key] = builder
	}
	return &RunnerResolver{builders: copy}
}

func (r *RunnerResolver) Resolve(runModel model.Run, params model.CreateRunParams, project model.Project) (run.Runner, error) {
	builder, ok := r.builders[TargetKey{Artifact: runModel.WorkSpec.Target.Artifact, Level: runModel.WorkSpec.Target.Level}]
	if !ok {
		return nil, ErrRunTargetUnsupported
	}
	return &workSpecRunner{inner: builder(runModel, params, project), spec: runModel.WorkSpec}, nil
}

type workSpecRunner struct {
	inner run.Runner
	spec  model.WorkSpec
}

func (r *workSpecRunner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, prompter run.Prompter) harness.Outcome {
	if r.spec.Interaction.Clarification == model.ClarifyBeforeApply && r.spec.Interaction.Intent == model.IntentApply {
		answer, err := prompter.NeedsInput(ctx, "before-apply", "已准备好执行变更，是否继续？", []string{"继续", "取消"})
		if err != nil || answer == "取消" {
			return harness.Outcome{Status: harness.OutcomeCanceled}
		}
	}
	outcome := r.inner.Run(ctx, &workSpecEmitter{Emitter: em, spec: r.spec}, cp, prompter)
	if outcome.Result == nil {
		outcome.Result = map[string]any{}
	}
	outcome.Result["artifact"] = r.spec.Target.Artifact
	outcome.Result["level"] = r.spec.Target.Level
	outcome.Result["target"] = r.spec.Target
	outcome.Result["interaction"] = r.spec.Interaction
	if _, ok := outcome.Result["operation"]; !ok {
		outcome.Result["operation"] = inferOperation(r.spec)
	}
	if r.spec.Target.SlideID != "" {
		outcome.Result["affected_slide_ids"] = []string{r.spec.Target.SlideID}
	}
	return outcome
}

type workSpecEmitter struct {
	harness.Emitter
	spec model.WorkSpec
}

func (e *workSpecEmitter) Emit(event model.EventType, payload any) {
	if event == model.EventRunStarted {
		runID := ""
		if started, ok := payload.(harness.RunStartedPayload); ok {
			runID = started.RunID
		}
		e.Emitter.Emit(event, harness.RunStartedPayload{
			RunID: runID, Target: &e.spec.Target, Interaction: &e.spec.Interaction, UserInput: e.spec.Instruction,
		})
		return
	}
	e.Emitter.Emit(event, payload)
}

func inferOperation(spec model.WorkSpec) string {
	if spec.Interaction.Intent == model.IntentConsult {
		return "consult"
	}
	if spec.Target.Artifact == model.ArtifactBlueprint {
		return "revise-blueprint"
	}
	return "materialize-or-revise"
}
