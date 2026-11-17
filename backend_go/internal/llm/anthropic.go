package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/modelusage"
)

// AnthropicAdapter implements Messages for any configured provider or gateway.
type AnthropicAdapter struct {
	provider     string
	model        string
	capabilities Capabilities
	replaySource string
	http         adapterHTTP
}

var _ Provider = (*AnthropicAdapter)(nil)

func NewAnthropicAdapter(cfg AdapterConfig) *AnthropicAdapter {
	http := newAdapterHTTP(cfg.APIKey, cfg.BaseURL, cfg.Timeout)
	http.client.Transport = modelusage.NewTransport(http.client.Transport, cfg.Provider, cfg.Model)
	http.protocol = ProtocolAnthropic
	return &AnthropicAdapter{
		provider: cfg.Provider, model: cfg.Model, capabilities: productCapabilities(), http: http,
		replaySource: continuationFingerprint([]string{ProtocolAnthropic, cfg.Provider, cfg.Model, http.baseURL}),
	}
}
func (a *AnthropicAdapter) Name() string               { return a.provider }
func (a *AnthropicAdapter) Model() string              { return a.model }
func (a *AnthropicAdapter) Capabilities() Capabilities { return cloneCapabilities(a.capabilities) }
func (a *AnthropicAdapter) String() string {
	return fmt.Sprintf("AnthropicAdapter{Provider:%q,Model:%q}", a.provider, a.model)
}
func (a *AnthropicAdapter) GoString() string { return a.String() }

func anthropicThinkingType(model string) string {
	switch path.Base(model) {
	case "glm-5.3-flash", "mimo-v2.6-flash", "deepseek-flash":
		return "enabled"
	default:
		return "disabled"
	}
}

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
	Model      string             `json:"model"`
	System     string             `json:"system,omitempty"`
	Messages   []anthropicMessage `json:"messages"`
	Tools      []anthropicTool    `json:"tools,omitempty"`
	MaxTokens  int                `json:"max_tokens"`
	Thinking   map[string]string  `json:"thinking"`
	ToolChoice map[string]any     `json:"tool_choice,omitempty"`
}

// Decode typed fields inside the HTTP decoder so malformed blocks follow the
// normal retry policy. Retain every field, including opaque ones, for replay.
type anthropicBlock struct {
	Type     string         `json:"type"`
	Thinking string         `json:"thinking"`
	Text     string         `json:"text"`
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Input    map[string]any `json:"input"`
	raw      json.RawMessage
}

func (b *anthropicBlock) UnmarshalJSON(raw []byte) error {
	type fields anthropicBlock
	if err := json.Unmarshal(raw, (*fields)(b)); err != nil {
		return err
	}
	b.raw = append(json.RawMessage(nil), raw...)
	return nil
}

type anthropicResponse struct {
	Type       string           `json:"type"`
	StopReason string           `json:"stop_reason"`
	Content    []anthropicBlock `json:"content"`
	Usage      struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func (a *AnthropicAdapter) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	strategy, err := toolStrategy(req, a.provider, ProtocolAnthropic, a.model, a.http.baseURL)
	if err != nil {
		return GenerateResponse{}, err
	}
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
	body := anthropicRequest{Model: a.model, System: collectSystemInstructions(req.Messages), Messages: messages, Tools: tools, MaxTokens: maxTokens, Thinking: map[string]string{"type": anthropicThinkingType(a.model)}}
	var wire anthropicResponse
	if strategy.RequiredTool != "" || strategy.ParallelToolCalls != nil {
		body.ToolChoice = map[string]any{"type": "auto"}
		if strategy.RequiredTool != "" {
			body.ToolChoice["type"] = "tool"
			body.ToolChoice["name"] = strategy.RequiredTool
		}
		if strategy.ParallelToolCalls != nil {
			body.ToolChoice["disable_parallel_tool_use"] = !*strategy.ParallelToolCalls
		}
	}
	if err := a.http.doJSONObserved(ctx, "/messages", body, req.OnRetry, req.OnRequest, &wire); err != nil {
		return GenerateResponse{}, classifyToolConstraintError(err, strategy)
	}
	if wire.StopReason == "refusal" {
		return GenerateResponse{}, fmt.Errorf("%w: model refused the request", ErrUnavailable)
	}
	if wire.Type == "error" || wire.StopReason == "max_tokens" || wire.StopReason == "pause_turn" || len(wire.Content) == 0 {
		return GenerateResponse{}, fmt.Errorf("%w: response is incomplete or empty", ErrUnavailable)
	}
	out := GenerateResponse{}
	if body.Thinking["type"] == "enabled" {
		out.Replay = &AssistantReplay{Source: a.replaySource, Blocks: make([]json.RawMessage, 0, len(wire.Content))}
	}
	for _, block := range wire.Content {
		if out.Replay != nil {
			out.Replay.Blocks = append(out.Replay.Blocks, block.raw)
		}
		switch block.Type {
		case "thinking":
			if thought := strings.TrimSpace(block.Thinking); thought != "" {
				out.Thinking = append(out.Thinking, thought)
			}
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
	var foreignCalls bool
	for _, message := range messages {
		if message.Role == RoleSystem {
			continue
		}
		parts, err := a.content(ctx, message.Content, resolver)
		if err != nil {
			return nil, err
		}
		role := string(message.Role)
		if message.Role == RoleAssistant {
			foreignCalls = len(message.ToolCalls) > 0 && anthropicThinkingType(a.model) == "enabled" &&
				(message.Replay == nil || message.Replay.Source != a.replaySource)
		}
		if message.Role == RoleTool && foreignCalls {
			// A different endpoint cannot authenticate this round. Carry its results
			// as attributed history, keeping images in user content.
			role = "user"
			parts = append([]any{map[string]any{"type": "text", "text": "Historical tool result for " + message.ToolCallID + ":"}}, parts...)
		} else if message.Role == RoleTool {
			role = "user"
			block := map[string]any{"type": "tool_result", "tool_use_id": message.ToolCallID, "content": parts}
			if message.Metadata != nil && message.Metadata.Origin == "runtime" && message.Metadata.Kind == "submission_error" {
				block["is_error"] = true
			}
			parts = []any{block}
		} else if message.Role == RoleAssistant && message.Replay != nil && message.Replay.Source == a.replaySource {
			parts = make([]any, 0, len(message.Replay.Blocks))
			for _, raw := range message.Replay.Blocks {
				parts = append(parts, raw)
			}
		} else if message.Role == RoleAssistant && foreignCalls {
			raw, err := json.Marshal(message.ToolCalls)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid historical tool calls", ErrBadToolCall)
			}
			parts = append(parts, map[string]any{"type": "text", "text": "Historical tool calls: " + string(raw)})
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
