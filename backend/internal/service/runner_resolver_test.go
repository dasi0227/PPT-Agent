package service

import (
	"context"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

type resolvedRunner struct{}

func (resolvedRunner) Run(context.Context, harness.Emitter, harness.Checkpointer, run.Prompter) harness.Outcome {
	return harness.Outcome{Status: harness.OutcomeFinished}
}

type outcomeRunner struct{ outcome harness.Outcome }

func (r outcomeRunner) Run(context.Context, harness.Emitter, harness.Checkpointer, run.Prompter) harness.Outcome {
	return r.outcome
}

type promptRecorder struct {
	calls  int
	answer string
}

func (p *promptRecorder) NeedsInput(context.Context, string, string, []string) (string, error) {
	p.calls++
	return p.answer, nil
}

type discardEmitter struct{}

func (discardEmitter) Emit(model.EventType, any) {}

func TestRunnerResolverFourTargetCombinations(t *testing.T) {
	called := map[TargetKey]int{}
	builders := map[TargetKey]TargetRunnerBuilder{}
	for _, key := range []TargetKey{
		{model.ArtifactBlueprint, model.TargetDeck},
		{model.ArtifactBlueprint, model.TargetSlide},
		{model.ArtifactPresentation, model.TargetDeck},
		{model.ArtifactPresentation, model.TargetSlide},
	} {
		key := key
		builders[key] = func(model.Run, model.CreateRunParams, model.Project, contextengine.ContextPack) run.Runner {
			called[key]++
			return resolvedRunner{}
		}
	}
	resolver := NewRunnerResolver(builders)
	for key := range builders {
		runModel := model.Run{WorkSpec: model.WorkSpec{Target: model.RunTarget{Artifact: key.Artifact, Level: key.Level}}}
		if _, err := resolver.Resolve(runModel, model.CreateRunParams{}, model.Project{}, contextengine.ContextPack{}); err != nil {
			t.Fatalf("resolve %+v: %v", key, err)
		}
		if called[key] != 1 {
			t.Fatalf("builder %+v called %d times", key, called[key])
		}
	}
}

func TestClarificationPolicies(t *testing.T) {
	for _, tc := range []struct {
		name       string
		intent     model.InteractionIntent
		policy     model.ClarificationPolicy
		answer     string
		wantCalls  int
		wantStatus harness.OutcomeStatus
	}{
		{"before apply confirms", model.IntentApply, model.ClarifyBeforeApply, "继续", 1, harness.OutcomeFinished},
		{"before apply cancels", model.IntentApply, model.ClarifyBeforeApply, "取消", 1, harness.OutcomeCanceled},
		{"when blocked continues", model.IntentApply, model.ClarifyWhenBlocked, "", 0, harness.OutcomeFinished},
		{"never continues", model.IntentApply, model.ClarifyNever, "", 0, harness.OutcomeFinished},
		{"consult never confirms writes", model.IntentConsult, model.ClarifyBeforeApply, "", 0, harness.OutcomeFinished},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &promptRecorder{answer: tc.answer}
			runner := &workSpecRunner{
				inner: resolvedRunner{},
				spec: model.WorkSpec{
					Target:      model.RunTarget{Artifact: model.ArtifactBlueprint, Level: model.TargetDeck},
					Interaction: model.RunInteraction{Intent: tc.intent, Clarification: tc.policy},
					Instruction: "test",
				},
			}
			got := runner.Run(context.Background(), discardEmitter{}, nil, recorder)
			if recorder.calls != tc.wantCalls || got.Status != tc.wantStatus {
				t.Fatalf("calls/status = %d/%s, want %d/%s", recorder.calls, got.Status, tc.wantCalls, tc.wantStatus)
			}
		})
	}
}

func TestRevisionTrackingSkipsFailedAndCanceledRuns(t *testing.T) {
	for _, status := range []harness.OutcomeStatus{harness.OutcomeLLMError, harness.OutcomeCanceled} {
		want := harness.Outcome{Status: status}
		runner := &revisionTrackingRunner{
			inner: outcomeRunner{outcome: want},
			spec:  model.WorkSpec{Interaction: model.RunInteraction{Intent: model.IntentApply}},
			// A non-successful outcome must return before touching these nil services.
		}
		if got := runner.Run(context.Background(), discardEmitter{}, nil, &promptRecorder{}); got.Status != status {
			t.Fatalf("status %s changed to %s", status, got.Status)
		}
	}
}

func TestContextPackRunnerUpdatesMemoryOnlyOnSuccess(t *testing.T) {
	for _, tc := range []struct {
		name         string
		status       harness.OutcomeStatus
		wantRevision int
	}{
		{"success", harness.OutcomeFinished, 1},
		{"failed", harness.OutcomeLLMError, 0},
		{"canceled", harness.OutcomeCanceled, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			pack := contextengine.ContextPack{
				SchemaVersion: contextengine.SchemaVersion,
				WorkSpec:      model.WorkSpec{Instruction: "confirmed change"},
				Manifest:      contextengine.ContextManifest{RunID: "r1", ThreadID: "t1", ProjectID: "p1", Profile: contextengine.ProfileBlueprintDeck, Warnings: []string{}},
			}
			runner := &contextPackRunner{inner: outcomeRunner{outcome: harness.Outcome{Status: tc.status}}, pack: pack, project: model.Project{ID: "p1", WorkDir: workDir}}
			runner.Run(context.Background(), discardEmitter{}, nil, &promptRecorder{})
			memory, _, err := (contextengine.ThreadMemoryStore{}).Load(workDir, "t1")
			if err != nil {
				t.Fatal(err)
			}
			if memory.Revision != tc.wantRevision {
				t.Fatalf("memory revision=%d want=%d", memory.Revision, tc.wantRevision)
			}
		})
	}
}
