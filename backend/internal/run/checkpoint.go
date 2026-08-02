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
	if err := c.bus.Emit(ctx, model.EventQuestionAsked, question); err != nil {
		return model.QuestionAnswer{}, "", err
	}
	select {
	case reply := <-c.queue.ReplySignal():
		if err := c.bus.Emit(ctx, model.EventQuestionAnswered, model.QuestionAnsweredPayload{
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
