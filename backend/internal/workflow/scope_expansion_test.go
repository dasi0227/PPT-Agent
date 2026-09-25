package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type committedScopePrompter struct{ approvingPrompter }

func (*committedScopePrompter) AskScopeExpansion(context.Context, model.ScopeExpansionRequestedPayload) (model.ScopeExpansionAnswer, error) {
	return model.ScopeExpansionAnswer{}, errors.New("committed scope must not request approval again")
}
func (*committedScopePrompter) ResumeScopeExpansion(_ context.Context, request model.ScopeExpansionRequestedPayload) (model.ScopeExpansionAnswer, error) {
	return model.ScopeExpansionAnswer{InteractionID: request.InteractionID, CallID: request.CallID, BaseRevision: request.BaseRevision, Decision: "approve"}, nil
}
func (*committedScopePrompter) ResumeAfterScopeExpansion(context.Context) {}

func TestCommittedScopeRecoveryOnlyPublishesAndConsumesAnswer(t *testing.T) {
	previous := model.NewRunScope(model.ScopeCustomPages, "sli_one")
	scope := model.NewRunScope(model.ScopeCustomPages, "sli_one", "sli_two")
	scope.Revision = 2
	request := model.ScopeExpansionRequestedPayload{PublicEventBase: publicBase("scope-recovery"), InteractionID: "scope-answer", CallID: "scope-call", BaseRevision: 1, CurrentScope: previous, ProposedScope: scope}
	state := &RunState{
		runID: "scope-recovery", mode: model.ModeExecute, phase: PhaseExecuting, scope: scope,
		pack: scopeExpansionPack(), ledger: NewEvidenceLedger(), activeSkills: &ActiveSkillSet{},
		pendingScopeExpansion: &PendingScopeExpansion{Request: request, ResumePhase: PhaseExecuting, Applied: true},
	}
	checkpoints := &checkpointRecorder{}
	events := &eventRecorder{}
	input := RuntimeInput{Prompter: &committedScopePrompter{}, Checkpoint: checkpoints, Emitter: events}
	if outcome, stop := NewRuntime(nil).awaitScopeExpansion(context.Background(), input, state, request, nil, ""); stop {
		t.Fatalf("scope recovery failed: %+v", outcome)
	}
	if state.scope.Revision != 2 || state.pendingScopeExpansion != nil || events.count(model.EventScopeExpansionAnswered) != 1 || events.count(model.EventScopeUpdated) != 1 {
		t.Fatalf("scope was reapplied or publication was lost: %+v", state.scope)
	}
	for _, event := range events.events {
		if event.kind == model.EventScopeUpdated {
			updated := event.payload.(model.ScopeUpdatedPayload)
			if !updated.PreviousScope.Equal(previous) || !updated.Scope.Equal(scope) || updated.InteractionID != request.InteractionID {
				t.Fatalf("restored scope update is wrong: %+v", updated)
			}
		}
	}
	last := checkpoints.checkpoints[len(checkpoints.checkpoints)-1]
	if last.PendingScopeExpansion != nil || last.Scope.Revision != 2 {
		t.Fatalf("scope consumption was not checkpointed: %+v", last)
	}
}

func scopeExpansionPack() contextengine.ContextPack {
	return contextengine.ContextPack{Outline: contextengine.OutlineContext{Summaries: []contextengine.SlideSummary{
		{ID: "sli_one", Ordinal: 1}, {ID: "sli_two", Ordinal: 2}, {ID: "sli_three", Ordinal: 3},
	}}}
}

func TestProposeScopeExpansionMergesPages(t *testing.T) {
	current := model.NewRunScope(model.ScopeCustomPages, "sli_one")
	next, addition, err := proposeScopeExpansion(current, scopeExpansionPack(), []string{"sli_three"})
	if err != nil {
		t.Fatal(err)
	}
	if next.Revision != 2 {
		t.Fatalf("unexpected revision: %#v", next)
	}
	if len(next.SlideIDs) != 2 || next.SlideIDs[0] != "sli_one" || next.SlideIDs[1] != "sli_three" {
		t.Fatalf("unexpected slides: %#v", next.SlideIDs)
	}
	if len(addition.SlideIDs) != 1 || addition.SlideIDs[0] != "sli_three" {
		t.Fatalf("unexpected addition: %#v", addition)
	}
}

func TestProposeScopeExpansionNoopsForContainedPrivileges(t *testing.T) {
	current := model.NewRunScope(model.ScopeCustomPages, "sli_one")
	next, _, err := proposeScopeExpansion(current, scopeExpansionPack(), []string{"sli_one"})
	if err != nil {
		t.Fatal(err)
	}
	if !next.Equal(current) {
		t.Fatalf("contained request changed scope: %#v", next)
	}
}

func TestAdjustedScopeCannotShrinkCurrentPrivileges(t *testing.T) {
	current := model.NewRunScope(model.ScopeCustomPages, "sli_one", "sli_two")
	candidate := model.NewRunScope(model.ScopeCustomPages, "sli_one")
	if scopeContains(candidate, current) {
		t.Fatal("shrinking pages must be rejected")
	}
}
