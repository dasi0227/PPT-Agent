package run

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Prompter 是 harness/agent 请求用户输入的挂钩（被动应答 HITL）。
// 发 needs_input → Run 转 waiting → 阻塞等待匹配应答 → 转回 running（API-RUN-004）。
type Prompter interface {
	// NeedsInput 发出 needs_input 事件并阻塞，直到收到匹配 reply 或 ctx 取消。
	// 返回用户应答内容；ctx 取消返回 ctx.Err()。
	NeedsInput(ctx context.Context, id, prompt string, choices []string) (string, error)
}

// checkpoint 实现 Prompter：桥接 Bus（发事件）+ InputQueue（登记/等待应答）+ engine（改状态）。
type checkpoint struct {
	engine *Engine
	runID  string
	bus    *Bus
	queue  *InputQueue
}

// NeedsInputPayload 是 needs_input 事件负载（API-SSE / API-RUN-003）。
type NeedsInputPayload struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Choices []string `json:"choices,omitempty"`
}

func (c *checkpoint) NeedsInput(ctx context.Context, id, prompt string, choices []string) (string, error) {
	c.queue.MarkNeedsInput(id)
	c.engine.setStatus(ctx, c.runID, model.RunWaiting)
	if err := c.bus.Emit(ctx, model.EventNeedsInput, NeedsInputPayload{ID: id, Prompt: prompt, Choices: choices}); err != nil {
		return "", err
	}
	select {
	case reply := <-c.queue.ReplySignal():
		c.engine.setStatus(ctx, c.runID, model.RunRunning)
		return reply, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

var _ harness.Emitter = (*harnessEmitter)(nil)
