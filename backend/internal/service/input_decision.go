package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

type inputDecisionStore interface {
	LoadIntentHistory(context.Context, string, string, string) ([]model.DialogueRound, error)
	SaveInputDecision(context.Context, model.Run, model.InputDecision) (model.InputDecision, error)
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
	// A stop decision is irreversible for this run, including after a server
	// restart or a model/rule update. Only passed results need revalidation.
	if cacheValid && saved.StopsRun() {
		return saved, nil
	}
	history, historyErr := store.LoadIntentHistory(ctx, current.ThreadID, current.ID, inputID)
	req, requestHash, requestErr := decision.IntentRequest(history, content)
	result.Fingerprint = decision.IntentFingerprint(snapshot.Identity, requestHash)
	if historyErr == nil && requestErr == nil {
		if cacheValid && saved.Fingerprint == result.Fingerprint && saved.Identity == snapshot.Identity &&
			(saved.Disposition == model.InputNormal || saved.Disposition == model.InputGreeting || saved.Disposition == model.InputIdentity) {
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
	if svc.naming != nil && result.Disposition == model.InputNormal {
		svc.naming.RecordInput(current, result.InputID)
	}
}
