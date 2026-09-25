package run

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Prompter 是 Runtime 请求用户输入的挂钩（被动应答 HITL）。
// question.asked → waiting → question.answered → running。
type Prompter interface {
	Ask(ctx context.Context, question model.QuestionAskedPayload) (model.QuestionAnswer, string, error)
}

// checkpoint 实现 Prompter：桥接 Bus（发事件）+ InputQueue（登记/等待应答）+ engine（改状态）。
type checkpoint struct {
	engine *Engine
	runID  string
	bus    *Bus
	queue  *InputQueue
}

func (c *checkpoint) Ask(
	ctx context.Context,
	question model.QuestionAskedPayload,
) (model.QuestionAnswer, string, error) {
	c.queue.MarkQuestion(question)
	c.engine.setStatus(ctx, c.runID, model.RunWaiting)
	if err := c.emitInteraction(ctx, model.EventQuestionAsked, question.QuestionID, question); err != nil {
		return model.QuestionAnswer{}, "", err
	}
	if err := c.replayAnswer(ctx, "question", question.QuestionID); err != nil {
		return model.QuestionAnswer{}, "", err
	}
	select {
	case reply := <-c.queue.ReplySignal():
		if err := c.emitInteraction(ctx, model.EventQuestionAnswered, reply.QuestionID, model.QuestionAnsweredPayload{
			PublicEventBase: model.NewPublicEventBase(c.runID),
			QuestionID:      reply.QuestionID, Answer: reply.Answer, DisplayText: reply.DisplayText,
		}); err != nil {
			return model.QuestionAnswer{}, "", err
		}
		c.engine.setStatus(ctx, c.runID, model.RunRunning)
		return reply.Answer, reply.DisplayText, nil
	case <-ctx.Done():
		return model.QuestionAnswer{}, "", ctx.Err()
	}
}

func (c *checkpoint) AskPlanApproval(ctx context.Context, payload model.PlanApprovalRequestedPayload) (model.PlanApprovalAnswer, error) {
	c.queue.MarkPlanApproval(payload)
	c.engine.setStatus(ctx, c.runID, model.RunWaiting)
	if err := c.emitInteraction(ctx, model.EventPlanApprovalRequested, payload.InteractionID, payload); err != nil {
		return model.PlanApprovalAnswer{}, err
	}
	if err := c.replayAnswer(ctx, "plan", payload.InteractionID); err != nil {
		return model.PlanApprovalAnswer{}, err
	}
	select {
	case answer := <-c.queue.PlanApprovalSignal():
		return answer, nil
	case <-ctx.Done():
		return model.PlanApprovalAnswer{}, ctx.Err()
	}
}

func (c *checkpoint) ResumeAfterPlanApproval(ctx context.Context) {
	c.engine.setStatus(ctx, c.runID, model.RunRunning)
}

func (c *checkpoint) AskScopeExpansion(ctx context.Context, payload model.ScopeExpansionRequestedPayload) (model.ScopeExpansionAnswer, error) {
	c.queue.MarkScopeExpansion(payload)
	c.engine.setStatus(ctx, c.runID, model.RunWaiting)
	if err := c.emitInteraction(ctx, model.EventScopeExpansionRequested, payload.InteractionID, payload); err != nil {
		return model.ScopeExpansionAnswer{}, err
	}
	if err := c.replayAnswer(ctx, "scope", payload.InteractionID); err != nil {
		return model.ScopeExpansionAnswer{}, err
	}
	select {
	case answer := <-c.queue.ScopeExpansionSignal():
		return answer, nil
	case <-ctx.Done():
		return model.ScopeExpansionAnswer{}, ctx.Err()
	}
}

func (c *checkpoint) ResumeAfterScopeExpansion(ctx context.Context) {
	c.engine.setStatus(ctx, c.runID, model.RunRunning)
}

func (c *checkpoint) ResumeScopeExpansion(ctx context.Context, payload model.ScopeExpansionRequestedPayload) (model.ScopeExpansionAnswer, error) {
	c.queue.MarkScopeExpansion(payload)
	c.engine.setStatus(ctx, c.runID, model.RunWaiting)
	if err := c.replayAnswer(ctx, "scope", payload.InteractionID); err != nil {
		return model.ScopeExpansionAnswer{}, err
	}
	select {
	case answer := <-c.queue.ScopeExpansionSignal():
		return answer, nil
	case <-ctx.Done():
		return model.ScopeExpansionAnswer{}, ctx.Err()
	}
}

func (c *checkpoint) AskCommandPermission(
	ctx context.Context,
	payload model.CommandPermissionRequestedPayload,
) (model.CommandPermissionAnswer, error) {
	c.queue.MarkCommandPermission(payload)
	c.engine.setStatus(ctx, c.runID, model.RunWaiting)
	if err := c.emitInteraction(ctx, model.EventCommandPermissionRequested, payload.InteractionID, payload); err != nil {
		return model.CommandPermissionAnswer{}, err
	}
	if err := c.replayAnswer(ctx, "command", payload.InteractionID); err != nil {
		return model.CommandPermissionAnswer{}, err
	}
	select {
	case answer := <-c.queue.CommandPermissionSignal():
		if err := c.emitInteraction(ctx, model.EventCommandPermissionAnswered, answer.InteractionID, model.CommandPermissionAnsweredPayload{
			PublicEventBase: model.NewPublicEventBase(c.runID),
			InteractionID:   answer.InteractionID, CallID: answer.CallID,
			CommandHash: answer.CommandHash, Decision: answer.Decision,
		}); err != nil {
			return model.CommandPermissionAnswer{}, err
		}
		c.engine.setStatus(ctx, c.runID, model.RunRunning)
		return answer, nil
	case <-ctx.Done():
		return model.CommandPermissionAnswer{}, ctx.Err()
	}
}
