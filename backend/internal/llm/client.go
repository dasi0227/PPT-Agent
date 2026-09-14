// Package llm defines the provider-neutral protocol used by the PPT runtime.
package llm

import (
	"context"
	"encoding/json"
)

// Role is the normalized message role used by every provider adapter.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is provider-neutral conversation state. Provider-private reasoning
// never belongs here; it is carried only by ProviderContinuation.
type Message struct {
	Role       Role
	Content    []ContentPart
	ToolCallID string
	ToolCalls  []ToolCall
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

// Capabilities are the fixed product contract for every configured model.
type Capabilities struct {
	Vision              bool
	ToolCalls           bool
	MultipleToolCalls   bool
	ContextWindowTokens int
	ImageInputMIMEs     []string
	MaxImageBytes       int
}

type ImageData struct {
	Bytes    []byte
	MIMEType string
}

type ImageRefResolver interface {
	ResolveImage(context.Context, string) (ImageData, error)
}

// ToolSchema is one Runtime-disclosed function schema.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// ToolCall is one normalized function call selected by a provider.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// ProviderContinuation is deliberately opaque to Runtime. Adapters use it for
// provider protocol state such as OpenAI response state.
type ProviderContinuation struct {
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	Opaque   json.RawMessage `json:"opaque"`
}

type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type GenerateRequest struct {
	Messages        []Message
	Tools           []ToolSchema
	ImageResolver   ImageRefResolver
	Continuation    *ProviderContinuation
	OnRetry         func(attempt int)
	MaxOutputTokens int
}

type GenerateResponse struct {
	Content      []ContentPart
	ToolCalls    []ToolCall
	Continuation *ProviderContinuation
	Usage        Usage
}

func (r GenerateResponse) Text() string {
	for _, part := range r.Content {
		if part.Type == "text" {
			return part.Text
		}
	}
	return ""
}

// Provider is the only model protocol used by the core ReAct Runtime.
type Provider interface {
	Name() string
	Model() string
	Capabilities() Capabilities
	Generate(context.Context, GenerateRequest) (GenerateResponse, error)
}
