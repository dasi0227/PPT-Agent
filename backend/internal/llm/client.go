// Package llm 定义 LLM 客户端契约（interface），DeepSeek 为其一实现（ARCH-LLM-001）。
package llm

import "context"

// Role 是消息角色；system 承载系统级约束，user 承载用户内容，二者不混淆（ARCH-LLM-005）。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 是一条对话消息。
// ToolCalls 在 role=assistant 时记录 LLM 发起的工具调用；ToolCallID 在 role=tool 时关联对应的工具调用。
type Message struct {
	Role             Role
	Content          []ContentPart
	ReasoningContent string
	ToolCallID       string
	ToolCalls        []ToolCall
}

type ContentPart struct {
	Type     string
	Text     string
	ImageRef string
	MIMEType string
	Detail   string
}

func TextContent(text string) []ContentPart {
	if text == "" {
		return []ContentPart{}
	}
	return []ContentPart{{Type: "text", Text: text}}
}

func (m Message) Text() string {
	for _, part := range m.Content {
		if part.Type == "text" {
			return part.Text
		}
	}
	return ""
}

type ProviderCapabilities struct {
	Vision            bool
	MultipleToolCalls bool
	ImageInputMIMEs   []string
	MaxImageBytes     int
}

type ImageData struct {
	Bytes    []byte
	MIMEType string
}

type ImageRefResolver interface {
	ResolveImage(context.Context, string) (ImageData, error)
}

type CapabilityProvider interface {
	Capabilities() ProviderCapabilities
}

// ToolSchema is one Runtime-disclosed function schema.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema
}

// ChatRequest 是一次补全请求。
type ChatRequest struct {
	Messages []Message
}

// ChatResponse 是一次性补全结果。
type ChatResponse struct {
	Content string
}

// StreamChunk 是流式增量。Done 为 true 时表示流结束（Err 携带非正常终止原因）。
type StreamChunk struct {
	Text string
	Done bool
	Err  error
}

// ToolCallRequest carries the strategy/stage/step-scoped tool subset.
type ToolCallRequest struct {
	Messages      []Message
	Tools         []ToolSchema
	ImageResolver ImageRefResolver
	OnRetry       func(attempt int)
}

// ToolCall 是 LLM 选择的一次工具调用。
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// ToolCallResponse keeps provider reasoning separate from ordinary assistant
// content. ReasoningContent is provider protocol state only and must never be
// projected to a public event or thread history.
type ToolCallResponse struct {
	ToolCalls        []ToolCall
	Text             string
	ReasoningContent string
}

// Client 抽象 LLM 调用；所有方法接受 context.Context 以支持取消（ARCH-LLM-002）。
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	Stream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
	CallTool(ctx context.Context, req ToolCallRequest) (ToolCallResponse, error)
}
