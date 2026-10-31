package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

type inputDecisionStore interface {
	SaveInputDecision(context.Context, model.Run, model.InputDecision) (model.InputDecision, error)
}

func buildIntentRequest(source model.ThreadConversationContext, input string) (decision.Request, string, error) {
	rules, err := prompt.Load("jev.intent")
	if err != nil {
		return decision.Request{}, "", err
	}
	history := contextengine.BuildConversationHistory(source.Rounds, contextengine.DecisionHistoryWindow())
	state := "<classification_rules>\n" + rules.Body + "\n</classification_rules>\n\n" + history.Text +
		"\n\n<current_user_input>\n" + contextengine.EscapeCommandContextText(input) + "\n</current_user_input>"
	return decision.IntentRequest(state)
}

func (svc *RunService) classifyInput(ctx context.Context, current model.Run, inputID, content string) (model.InputDecision, error) {
	if err := ctx.Err(); err != nil {
		return model.InputDecision{}, err
	}
	store, ok := svc.store.(inputDecisionStore)
	if !ok {
		return model.InputDecision{}, errors.New("input decision store is required")
	}
	snapshot := decision.Snapshot{}
	if svc.registry != nil {
		registry := svc.registry.Snapshot()
		if pinned, ok := svc.snapshots.Load(current.ID); ok {
			registry = pinned.(*llm.Registry)
		}
		snapshot = registry.DecisionSnapshot()
	}
	result := model.InputDecision{InputID: inputID, ExecutionRevision: current.ExecutionRevision, Identity: snapshot.Identity, Disposition: model.InputDetectionFailed}
	cached, cacheErr := svc.store.GetIdempotency(ctx, "input_decision", current.ID, inputID)
	var saved model.InputDecision
	cacheValid := cacheErr == nil && cached.Status == "completed" && json.Unmarshal([]byte(cached.ResultJSON), &saved) == nil && saved.InputID == inputID && saved.Fingerprint == cached.RequestHash && saved.Fingerprint != ""
	if cacheErr != nil && !errors.Is(cacheErr, run.ErrRunNotFound) {
		return result, cacheErr
	}
	// Stops and already published fixed replies are immutable, including after
	// restart or a model/rule update. Unpublished passed results are revalidated.
	if cacheValid && (saved.StopsRun() || saved.ReplyEventSeq > 0) {
		return saved, nil
	}
	source, historyErr := svc.store.LoadThreadConversationContextBeforeInput(ctx, current.ThreadID, current.ID, inputID)
	req, requestHash, requestErr := buildIntentRequest(source, content)
	result.Fingerprint = decision.IntentFingerprint(snapshot.Identity, requestHash)
	if historyErr == nil && requestErr == nil {
		if cacheValid && saved.Fingerprint == result.Fingerprint && saved.Identity == snapshot.Identity &&
			(saved.Disposition == model.InputPPT || saved.Disposition == model.InputGreeting || saved.Disposition == model.InputIdentity) {
			svc.scheduleInputNaming(current, saved)
			return saved, nil
		}
		classified := decision.ClassifyIntent(ctx, snapshot, req)
		result.Disposition, result.Failure = classified.Disposition, classified.Failure
	} else {
		result.Failure = "context_unavailable"
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	saved, err := store.SaveInputDecision(ctx, current, result)
	if err == nil {
		svc.scheduleInputNaming(current, saved)
	}
	return saved, err
}

func (svc *RunService) scheduleInputNaming(current model.Run, result model.InputDecision) {
	if svc.naming != nil && result.Disposition == model.InputPPT {
		svc.naming.RecordInput(current, result.InputID)
	}
}
