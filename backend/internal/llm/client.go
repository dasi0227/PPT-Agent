// Package llm defines the provider-neutral protocol used by the PPT runtime.
package llm

import (
	"context"
	"encoding/json"
	"strings"
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
	Metadata   *MessageMetadata `json:"-"`
}

// MessageMetadata is local provenance, persisted by the transcript store but
// never sent to providers. User text cannot manufacture this authority.
type MessageMetadata struct {
	Origin    string          `json:"origin"`
	Kind      string          `json:"kind"`
	Key       string          `json:"key,omitempty"`
	Hash      string          `json:"hash,omitempty"`
	RunID     string          `json:"run_id,omitempty"`
	Resources []ResourceStamp `json:"resources,omitempty"`
}

type ResourceStamp struct {
	Key  string `json:"key"`
	Hash string `json:"hash"`
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

// Text returns every text part in order, separated by paragraph boundaries.
// It is a text-only projection; multimodal adapters must preserve Content to
// retain images and their position relative to the text.
func (m Message) Text() string {
	return contentText(m.Content)
}

func contentText(content []ContentPart) string {
	texts := make([]string, 0, len(content))
	for _, part := range content {
		if part.Type == "text" && part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n\n")
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
	PauseOnFallback     bool // Runtime rebuilds context before invoking the activated fallback.
	Messages            []Message
	Tools               []ToolSchema
	ImageResolver       ImageRefResolver
	Continuation        *ProviderContinuation
	OnRetry             func(attempt int)
	MaxOutputTokens     int
	OnContinuationReset func(string)
}

type GenerateResponse struct {
	Content      []ContentPart
	ToolCalls    []ToolCall
	Continuation *ProviderContinuation
	Usage        Usage
}

func (r GenerateResponse) Text() string {
	return contentText(r.Content)
}

// Provider is the only model protocol used by the core ReAct Runtime.
type Provider interface {
	Name() string
	Model() string
	Capabilities() Capabilities
	Generate(context.Context, GenerateRequest) (GenerateResponse, error)
}
