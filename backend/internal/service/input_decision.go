package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	embedded "github.com/dasi0227/PPT-Agent/backend"
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

func buildIntentRequest(source model.ThreadConversationContext) (decision.Request, string, error) {
	if source.Input == nil {
		return decision.Request{}, "", errors.New("intent input is not durably accepted")
	}
	rules, err := prompt.Load("jev_intent")
	if err != nil {
		return decision.Request{}, "", err
	}
	history := contextengine.BuildConversationHistory(source.Rounds, contextengine.DecisionHistoryWindow())
	state := prompt.RenderGroup("system_prompt", rules) + "\n\n" + history.Text +
		"\n\n<has_explicit_references>" + strconv.FormatBool(source.Input.HasExplicitReferences) + "</has_explicit_references>" +
		"\n\n<current_user_input>\n" + contextengine.EscapeCommandContextText(source.Input.Text) + "\n</current_user_input>"
	return decision.IntentRequest(state)
}

func (svc *RunService) resolveInput(ctx context.Context, current model.Run, inputID string) (model.InputDecision, error) {
	if err := ctx.Err(); err != nil {
		return model.InputDecision{}, err
	}
	store, ok := svc.store.(inputDecisionStore)
	if !ok {
		return model.InputDecision{}, errors.New("input decision store is required")
	}
	registry := svc.registry.Snapshot()
	if pinned, ok := svc.snapshots.Load(current.ID); ok {
		registry = pinned.(*llm.Registry)
	}
	snapshot := registry.DecisionSnapshot()
	result := model.InputDecision{InputID: inputID, ExecutionRevision: current.ExecutionRevision, Identity: snapshot.Identity, Disposition: model.InputDetectionFailed}
	cached, cacheErr := svc.store.GetIdempotency(ctx, "input_decision", current.ID, inputID)
	var saved model.InputDecision
	cacheValid := cacheErr == nil && cached.Status == "completed" && json.Unmarshal([]byte(cached.ResultJSON), &saved) == nil && saved.InputID == inputID && saved.Fingerprint == cached.RequestHash && saved.Fingerprint != ""
	if cacheErr != nil && !errors.Is(cacheErr, run.ErrRunNotFound) {
		return result, cacheErr
	}
	// Stops and already published replies are immutable, including after
	// restart or a model/rule update. Unpublished passed results are revalidated.
	if cacheValid && (saved.StopsRun() || saved.ReplyEventSeq > 0) {
		return saved, nil
	}
	source, historyErr := svc.store.LoadThreadConversationContextBeforeInput(ctx, current.ThreadID, current.ID, inputID)
	req, requestHash, requestErr := buildIntentRequest(source)
	result.Fingerprint = decision.IntentFingerprint(snapshot.Identity, requestHash)
	if historyErr == nil && requestErr == nil {
		if cacheValid && saved.Fingerprint == result.Fingerprint && saved.Identity == snapshot.Identity &&
			(saved.Disposition == model.InputTask || saved.Disposition == model.InputChat) {
			svc.scheduleInputNaming(current, saved)
			return saved, nil
		}
		classified := decision.ClassifyIntent(ctx, snapshot, req)
		result.Disposition, result.Failure = classified.Disposition, classified.Failure
		if result.Disposition == model.InputChat && source.Input.HasExplicitReferences {
			result.Disposition = model.InputTask
		}
		if result.Disposition == model.InputChat {
			reply, err := generateChatReply(ctx, registry, source.Input.Text)
			if err != nil {
				result.Failure = model.CodeChatReplyFailed
			}
			result.ReplyText = reply
		}
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
	if svc.naming != nil && result.Disposition == model.InputTask {
		svc.naming.RecordInput(current, result.InputID)
	}
}

func generateChatReply(ctx context.Context, registry *llm.Registry, content string) (string, error) {
	profile, err := registry.RoutedProfile("reply", "")
	if err != nil {
		return "", err
	}
	response, err := profile.Adapter().Generate(ctx, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(embedded.ChatReplyPrompt)},
			{Role: llm.RoleUser, Content: llm.TextContent(content)},
		},
	})
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(response.Text())
	if text == "" || len(response.ToolCalls) != 0 {
		return "", errors.New("chat reply requires user-facing text without tool calls")
	}
	return text, nil
}
