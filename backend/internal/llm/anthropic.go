package llm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

// AnthropicAdapter implements Messages for any configured provider or gateway.
type AnthropicAdapter struct {
	provider     string
	model        string
	capabilities Capabilities
	http         adapterHTTP
}

var _ Provider = (*AnthropicAdapter)(nil)

func NewAnthropicAdapter(cfg AdapterConfig) *AnthropicAdapter {
	http := newAdapterHTTP(cfg.APIKey, cfg.BaseURL, cfg.Timeout)
	http.protocol = ProtocolAnthropic
	return &AnthropicAdapter{provider: cfg.Provider, model: cfg.Model, capabilities: productCapabilities(), http: http}
}
func (a *AnthropicAdapter) Name() string               { return a.provider }
func (a *AnthropicAdapter) Model() string              { return a.model }
func (a *AnthropicAdapter) Capabilities() Capabilities { return cloneCapabilities(a.capabilities) }
func (a *AnthropicAdapter) String() string {
	return fmt.Sprintf("AnthropicAdapter{Provider:%q,Model:%q}", a.provider, a.model)
}
func (a *AnthropicAdapter) GoString() string { return a.String() }

type anthropicMessage struct {
	Role    string `json:"role"`
	Content []any  `json:"content"`
}
type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}
type anthropicRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Thinking  map[string]string  `json:"thinking"`
}
type anthropicResponse struct {
	Type       string `json:"type"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type  string         `json:"type"`
		Text  string         `json:"text"`
		ID    string         `json:"id"`
		Name  string         `json:"name"`
		Input map[string]any `json:"input"`
	} `json:"content"`
	Usage struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func (a *AnthropicAdapter) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if err := validateContinuation(req.Continuation, a.Name(), a.Model()); err != nil {
		return GenerateResponse{}, err
	}
	if req.Continuation != nil && len(req.Continuation.Opaque) > 0 {
		return GenerateResponse{}, fmt.Errorf("%w: anthropic does not accept protocol continuation", ErrBadRequest)
	}
	messages, err := a.messages(ctx, req.Messages, req.ImageResolver)
	if err != nil {
		return GenerateResponse{}, err
	}
	tools := make([]anthropicTool, 0, len(req.Tools))
	for _, tool := range req.Tools {
		tools = append(tools, anthropicTool{Name: tool.Name, Description: tool.Description, InputSchema: toolParameters(tool.Parameters)})
	}
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	body := anthropicRequest{Model: a.model, System: collectSystemInstructions(req.Messages), Messages: messages, Tools: tools, MaxTokens: maxTokens, Thinking: map[string]string{"type": "disabled"}}
	var wire anthropicResponse
	if err := a.http.doJSON(ctx, "/messages", body, req.OnRetry, &wire); err != nil {
		return GenerateResponse{}, err
	}
	if wire.Type == "error" || wire.StopReason == "max_tokens" || len(wire.Content) == 0 {
		return GenerateResponse{}, fmt.Errorf("%w: response is incomplete or empty", ErrUnavailable)
	}
	out := GenerateResponse{}
	for _, block := range wire.Content {
		switch block.Type {
		case "text":
			out.Content = append(out.Content, TextContent(block.Text)...)
		case "tool_use":
			if strings.TrimSpace(block.ID) == "" || strings.TrimSpace(block.Name) == "" || block.Input == nil {
				return GenerateResponse{}, fmt.Errorf("%w: invalid tool use", ErrBadToolCall)
			}
			out.ToolCalls = append(out.ToolCalls, ToolCall{ID: block.ID, Name: block.Name, Args: block.Input})
		}
	}
	if len(out.Content) == 0 && len(out.ToolCalls) == 0 {
		return GenerateResponse{}, fmt.Errorf("%w: response contains no text or tool calls", ErrUnavailable)
	}
	inputTokens := wire.Usage.InputTokens + wire.Usage.CacheCreationInputTokens + wire.Usage.CacheReadInputTokens
	out.Usage = Usage{InputTokens: inputTokens, OutputTokens: wire.Usage.OutputTokens, TotalTokens: inputTokens + wire.Usage.OutputTokens}
	return out, nil
}

func (a *AnthropicAdapter) messages(ctx context.Context, messages []Message, resolver ImageRefResolver) ([]anthropicMessage, error) {
	out := make([]anthropicMessage, 0, len(messages))
	for _, message := range messages {
		if message.Role == RoleSystem {
			continue
		}
		parts, err := a.content(ctx, message.Content, resolver)
		if err != nil {
			return nil, err
		}
		role := string(message.Role)
		if message.Role == RoleTool {
			role = "user"
			parts = []any{map[string]any{"type": "tool_result", "tool_use_id": message.ToolCallID, "content": parts}}
		} else if message.Role == RoleAssistant {
			for _, call := range message.ToolCalls {
				args := call.Args
				if args == nil {
					args = map[string]any{}
				}
				parts = append(parts, map[string]any{"type": "tool_use", "id": call.ID, "name": call.Name, "input": args})
			}
		}
		if len(parts) == 0 {
			continue
		}
		// Consecutive tool results must share a single user turn directly after tool_use.
		if len(out) > 0 && out[len(out)-1].Role == role {
			out[len(out)-1].Content = append(out[len(out)-1].Content, parts...)
		} else {
			out = append(out, anthropicMessage{Role: role, Content: parts})
		}
	}
	return out, nil
}

func (a *AnthropicAdapter) content(ctx context.Context, parts []ContentPart, resolver ImageRefResolver) ([]any, error) {
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text != "" {
				out = append(out, map[string]any{"type": "text", "text": part.Text})
			}
		case "image":
			raw, mimeType, err := resolveProviderImage(ctx, part.ImageRef, resolver, a.capabilities)
			if err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": mimeType, "data": base64.StdEncoding.EncodeToString(raw)}})
		}
	}
	return out, nil
}
