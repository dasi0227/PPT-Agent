package harness

import "github.com/dasi0227/PPT-Agent/backend/internal/llm"

// 上下文预算：保留的最近工具轮次消息数上限（system + user 恒保留）。
// 超限时对早期 tool observation 做滚动摘要化压缩（ARCH-HARNESS-CTX-001/003）。
const maxHistoryMessages = 40

// keepRecentTurns 是压缩时保留的最近消息数（保留结论、丢弃早期冗余原文）。
const keepRecentTurns = 24

// compress 在历史超预算时裁剪：保留 system(0) 与首条 user(1)，将中间早期消息
// 折叠为一条摘要，尾部保留最近若干轮。优先级：目标产物/最新上下文 > 早期历史。
func compress(msgs []llm.Message) []llm.Message {
	if len(msgs) <= maxHistoryMessages {
		return msgs
	}
	head := msgs[:2] // system + 首条 user 指令，恒保留
	tail := msgs[len(msgs)-keepRecentTurns:]

	summary := llm.Message{
		Role:    llm.RoleAssistant,
		Content: "[早期轮次已摘要：先前工具调用与观察结果的结论已并入当前上下文]",
	}

	out := make([]llm.Message, 0, len(head)+1+len(tail))
	out = append(out, head...)
	out = append(out, summary)
	out = append(out, tail...)
	return out
}
