// Package llm 定义 LLM 客户端契约（interface），具体实现（DeepSeek）可替换（ARCH-BACKEND-001）。
// 具体实现随 M1 落地；此处仅锁定契约。
package llm

import "context"

// Message 是一条对话消息（role: system/user/assistant/tool）。
type Message struct {
	Role    string
	Content string
}

// Client 抽象 LLM 调用；所有方法接受 context.Context 以支持取消（DEV-CODING）。
type Client interface {
	Chat(ctx context.Context, msgs []Message) (string, error)
}
