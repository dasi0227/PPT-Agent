package workflow

import (
	"context"
	"errors"
	"slices"
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

func TestContainedExpansionIgnoresOutlineOrderAndRemovedPages(t *testing.T) {
	current := model.NewRunScope(model.ScopeAllPages, "sli_three", "sli_gone", "sli_one")
	next, addition, err := proposeScopeExpansion(current, scopeExpansionPack(), []string{"sli_one", "sli_three"})
	if err != nil || !next.Equal(current) || len(addition.SlideIDs) != 0 {
		t.Fatalf("contained request needs approval: next=%+v addition=%+v err=%v", next, addition, err)
	}
	next, addition, err = proposeScopeExpansion(current, scopeExpansionPack(), []string{"sli_one", "sli_two"})
	if err != nil || !scopeContains(next, current) || !slices.Equal(addition.SlideIDs, []string{"sli_two"}) || next.Revision != current.Revision+1 {
		t.Fatalf("expansion changed existing privileges: next=%+v addition=%+v err=%v", next, addition, err)
	}
}

func TestResumeRestoresScopeBeforeToolsAndSkipsContainedApproval(t *testing.T) {
	for _, kind := range []model.ScopeSelectionKind{model.ScopeAllPages, model.ScopeCustomPages} {
		t.Run(string(kind), func(t *testing.T) {
			pack := testPack(model.ModeExecute, model.ScopeAllPages, false, "检查当前页")
			// The accepted command still describes the originally empty project.
			scope := model.NewRunScope(kind, "sli_1")
			if kind == model.ScopeCustomPages {
				scope.SlideIDs = append(scope.SlideIDs, "sli_2")
				scope.Revision = 2
			}
			checkpoint := &RuntimeCheckpoint{RunID: "scope-resume", LoopID: "loop-resume", Scope: scope, Mode: model.ModeExecute, Phase: PhaseExecuting}
			if kind == model.ScopeCustomPages {
				checkpoint.PendingScopeExpansion = &PendingScopeExpansion{
					Applied: true, ResumePhase: PhaseExecuting,
					Request: model.ScopeExpansionRequestedPayload{
						PublicEventBase: publicBase(checkpoint.RunID), InteractionID: "saved-approval", CallID: "saved-expansion", BaseRevision: 1,
						CurrentScope: model.NewRunScope(model.ScopeCurrentPage, "sli_1"), ProposedScope: scope,
					},
				}
			}
			agent := &scriptedAgent{responses: []AgentResponse{
				toolCall("already-authorized", "request_privilege", map[string]any{"add_slide_ids": []string{"sli_1"}, "reason": "继续修改"}),
				toolCall("edit", "edit_spec", map[string]any{"slide_id": "sli_1", "key_message": "formal"}),
				finishCall("finish"),
			}}
			events, checkpoints := &eventRecorder{}, &checkpointRecorder{}
			outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
				RunID: checkpoint.RunID, ProjectDir: testProject(t, ArtifactSlideSpec), Context: pack,
				ResumeCheckpoint: checkpoint, Checkpoint: checkpoints, Emitter: events, Prompter: &committedScopePrompter{},
			})
			if outcome.Status != StatusCompleted || len(agent.requests) != 3 || events.count(model.EventScopeExpansionRequested) != 0 {
				t.Fatalf("resume requested approval or failed: outcome=%+v requests=%d events=%+v", outcome, len(agent.requests), events.events)
			}
			if !agent.requests[0].Context.Command.Scope.Equal(scope) || !slices.Equal(agent.requests[0].Context.Target.SlideIDs, scope.SlideIDs) {
				t.Fatalf("agent received stale scope: %+v", agent.requests[0].Context.Command.Scope)
			}
			completed := false
			for _, event := range events.events {
				if event.kind == model.EventToolCompleted {
					payload := event.payload.(model.ToolCompletedPayload)
					completed = payload.CallID == "edit" && payload.Status == "completed"
				}
			}
			if !completed || !checkpoints.checkpoints[0].Scope.Equal(scope) {
				t.Fatalf("restored scope lost or page edit rejected: completed=%v checkpoints=%+v", completed, checkpoints.checkpoints)
			}
		})
	}
}
