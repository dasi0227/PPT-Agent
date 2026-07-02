package run

import "sync"

// InputQueue 是控制输入队列（HITL）：主动注入的消息入队，仅在 checkpoint 被排空消费（ARCH-RUN-002）。
// 同时跟踪未应答的 needs_input，供 reply_to 精确匹配（API-RUN-003 / API-SSE-005）。
type InputQueue struct {
	mu      sync.Mutex
	pending []string
	// awaiting 是已发出但未应答的 needs_input id 集合。
	awaiting map[string]bool
	// reply 通道：waiting 状态下收到匹配应答时通知 engine 恢复。
	replyCh chan string
}

func NewInputQueue() *InputQueue {
	return &InputQueue{awaiting: map[string]bool{}, replyCh: make(chan string, 8)}
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

// MarkNeedsInput 登记一个待应答的 needs_input id。
func (q *InputQueue) MarkNeedsInput(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.awaiting[id] = true
}

// Reply 处理带 reply_to 的应答：必须匹配某个未应答的 needs_input，否则返回 false（API-RUN-003）。
// 匹配成功则入队内容并通知 engine 恢复。
func (q *InputQueue) Reply(replyTo, content string) bool {
	q.mu.Lock()
	if !q.awaiting[replyTo] {
		q.mu.Unlock()
		return false
	}
	delete(q.awaiting, replyTo)
	q.pending = append(q.pending, content)
	q.mu.Unlock()

	select {
	case q.replyCh <- content:
	default:
	}
	return true
}

// HasAwaiting 报告是否存在未应答的 needs_input（用于判断 reply 语义）。
func (q *InputQueue) HasAwaiting() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.awaiting) > 0
}

// ReplySignal 暴露应答信号通道，engine 在 waiting 时等待它恢复。
func (q *InputQueue) ReplySignal() <-chan string {
	return q.replyCh
}
