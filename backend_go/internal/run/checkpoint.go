package run

import (
	"context"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type Prompter interface {
	Ask(context.Context, model.QuestionAskedPayload) (model.QuestionAnswer, error)
}

// The database checkpoint owns waiting/running state. This bridge only waits
// for an accepted answer; Runtime applies it with the next durable checkpoint.
type checkpoint struct {
	engine *Engine
	runID  string
	queue  *InputQueue
}

func (c *checkpoint) Ask(ctx context.Context, question model.QuestionAskedPayload) (model.QuestionAnswer, error) {
	row, err := c.interaction(ctx, "question", question.QuestionID, &question)
	if err != nil {
		return model.QuestionAnswer{}, err
	}
	c.queue.MarkQuestion(question)
	if err := c.deliverAcceptedAnswer(ctx, row); err != nil {
		return model.QuestionAnswer{}, err
	}
	select {
	case reply := <-c.queue.ReplySignal():
		return reply.Answer, nil
	case <-ctx.Done():
		return model.QuestionAnswer{}, ctx.Err()
	}
}

func (c *checkpoint) AskPlanApproval(ctx context.Context, request model.PlanApprovalRequestedPayload) (model.PlanApprovalAnswer, error) {
	row, err := c.interaction(ctx, "plan", request.InteractionID, &request)
	if err != nil {
		return model.PlanApprovalAnswer{}, err
	}
	c.queue.MarkPlanApproval(request)
	if err := c.deliverAcceptedAnswer(ctx, row); err != nil {
		return model.PlanApprovalAnswer{}, err
	}
	select {
	case answer := <-c.queue.PlanApprovalSignal():
		return answer, nil
	case <-ctx.Done():
		return model.PlanApprovalAnswer{}, ctx.Err()
	}
}

func (c *checkpoint) AskScopeExpansion(ctx context.Context, request model.ScopeExpansionRequestedPayload) (model.ScopeExpansionAnswer, error) {
	row, err := c.interaction(ctx, "scope", request.InteractionID, &request)
	if err != nil {
		return model.ScopeExpansionAnswer{}, err
	}
	c.queue.MarkScopeExpansion(request)
	if err := c.deliverAcceptedAnswer(ctx, row); err != nil {
		return model.ScopeExpansionAnswer{}, err
	}
	select {
	case answer := <-c.queue.ScopeExpansionSignal():
		return answer, nil
	case <-ctx.Done():
		return model.ScopeExpansionAnswer{}, ctx.Err()
	}
}

func (c *checkpoint) AskResourceEditApproval(ctx context.Context, request model.ResourceEditApprovalRequestedPayload) (model.ResourceEditApprovalAnswer, error) {
	row, err := c.interaction(ctx, "resource", request.InteractionID, &request)
	if err != nil {
		return model.ResourceEditApprovalAnswer{}, err
	}
	c.queue.MarkResourceApproval(request)
	if err := c.deliverAcceptedAnswer(ctx, row); err != nil {
		return model.ResourceEditApprovalAnswer{}, err
	}
	select {
	case answer := <-c.queue.ResourceApprovalSignal():
		return answer, nil
	case <-ctx.Done():
		return model.ResourceEditApprovalAnswer{}, ctx.Err()
	}
}

func (c *checkpoint) AskCommandPermission(ctx context.Context, request model.CommandPermissionRequestedPayload) (model.CommandPermissionAnswer, error) {
	row, err := c.interaction(ctx, "command", request.InteractionID, &request)
	if err != nil {
		return model.CommandPermissionAnswer{}, err
	}
	c.queue.MarkCommandPermission(request)
	if err := c.deliverAcceptedAnswer(ctx, row); err != nil {
		return model.CommandPermissionAnswer{}, err
	}
	select {
	case answer := <-c.queue.CommandPermissionSignal():
		return answer, nil
	case <-ctx.Done():
		return model.CommandPermissionAnswer{}, ctx.Err()
	}
}
