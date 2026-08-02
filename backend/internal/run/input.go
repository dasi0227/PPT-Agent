package run

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// InputQueue 是控制输入队列（HITL）：主动注入的消息入队，仅在 checkpoint 被排空消费（ARCH-RUN-002）。
// 同时跟踪未应答 question，供 reply_to 与结构化答案精确校验。
type InputQueue struct {
	mu      sync.Mutex
	pending []string
	// awaiting 保存未应答的权威问题，用于校验 reply_to 与结构化答案。
	awaiting map[string]model.QuestionAskedPayload
	// reply 通道：waiting 状态下收到匹配应答时通知 engine 恢复。
	replyCh chan AcceptedReply
}

func NewInputQueue() *InputQueue {
	return &InputQueue{
		awaiting: map[string]model.QuestionAskedPayload{},
		replyCh:  make(chan AcceptedReply, 8),
	}
}

type AcceptedReply struct {
	QuestionID  string
	Answer      model.QuestionAnswer
	DisplayText string
}

// Enqueue 追加一条控制输入（主动注入，checkpoint 消费）。
func (q *InputQueue) Enqueue(content string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending = append(q.pending, content)
}

// Drain 排空并返回队列中全部待消费输入（checkpoint 调用）。
func (q *InputQueue) Drain() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return nil
	}
	out := q.pending
	q.pending = nil
	return out
}

// MarkQuestion 登记一个待应答的权威问题。
func (q *InputQueue) MarkQuestion(question model.QuestionAskedPayload) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.awaiting[question.QuestionID] = question
}

// Reply 处理带 reply_to 的应答：必须匹配未应答 question 且答案合法。
// 匹配成功则入队内容并通知 engine 恢复。
func (q *InputQueue) Reply(replyTo, content string) bool {
	q.mu.Lock()
	question, ok := q.awaiting[replyTo]
	if !ok {
		q.mu.Unlock()
		return false
	}
	answer, displayText, ok := validateQuestionAnswer(question, content)
	if !ok {
		q.mu.Unlock()
		return false
	}
	delete(q.awaiting, replyTo)
	q.mu.Unlock()

	select {
	case q.replyCh <- AcceptedReply{
		QuestionID: replyTo, Answer: answer, DisplayText: displayText,
	}:
	default:
	}
	return true
}

// HasAwaiting 报告是否存在未应答 question。
func (q *InputQueue) HasAwaiting() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.awaiting) > 0
}

// ReplySignal 暴露应答信号通道，engine 在 waiting 时等待它恢复。
func (q *InputQueue) ReplySignal() <-chan AcceptedReply {
	return q.replyCh
}

func validateQuestionAnswer(question model.QuestionAskedPayload, content string) (model.QuestionAnswer, string, bool) {
	var answer model.QuestionAnswer
	if err := json.Unmarshal([]byte(content), &answer); err != nil {
		return model.QuestionAnswer{}, "", false
	}
	labels := map[string]string{}
	for _, option := range question.Options {
		labels[option.ID] = option.Label
	}
	seen := map[string]bool{}
	display := []string{}
	for _, id := range answer.SelectedOptionIDs {
		if labels[id] == "" || seen[id] {
			return model.QuestionAnswer{}, "", false
		}
		seen[id] = true
		display = append(display, labels[id])
	}
	if question.Selection == "single" && len(answer.SelectedOptionIDs) > 1 {
		return model.QuestionAnswer{}, "", false
	}
	answer.CustomText = strings.TrimSpace(answer.CustomText)
	if answer.CustomText != "" {
		if !question.AllowCustom {
			return model.QuestionAnswer{}, "", false
		}
		display = append(display, answer.CustomText)
	}
	if len(display) == 0 {
		return model.QuestionAnswer{}, "", false
	}
	return answer, strings.Join(display, "；"), true
}
