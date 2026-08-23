package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// DeepSeekAdapter maps the provider-neutral protocol to DeepSeek Chat
// Completions. DeepSeek models are intentionally treated as text-only here.
type DeepSeekAdapter struct {
	model        string
	capabilities Capabilities
	http         adapterHTTP
}

var _ Provider = (*DeepSeekAdapter)(nil)

func NewDeepSeekAdapter(cfg DeepSeekConfig) *DeepSeekAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	return &DeepSeekAdapter{
		model: cfg.Model, capabilities: capabilitiesFor("deepseek", cfg.Model),
		http: newAdapterHTTP(cfg.APIKey, cfg.BaseURL, cfg.Timeout),
	}
}

func (d *DeepSeekAdapter) Name() string  { return "deepseek" }
func (d *DeepSeekAdapter) Model() string { return d.model }
func (d *DeepSeekAdapter) String() string {
	return fmt.Sprintf("DeepSeekAdapter{Model:%q}", d.model)
}
func (d *DeepSeekAdapter) GoString() string { return d.String() }
func (d *DeepSeekAdapter) Capabilities() Capabilities {
	return cloneCapabilities(d.capabilities)
}

type chatWireMessage struct {
	Role             string         `json:"role"`
	Content          any            `json:"content"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	Name             string         `json:"name,omitempty"`
	ToolCalls        []chatToolCall `json:"tool_calls,omitempty"`
}

type chatContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

type chatImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type deepSeekRequest struct {
	Model           string            `json:"model"`
	Messages        []chatWireMessage `json:"messages"`
	Tools           []chatTool        `json:"tools,omitempty"`
	ReasoningEffort string            `json:"reasoning_effort,omitempty"`
	Thinking        map[string]string `json:"thinking,omitempty"`
	MaxTokens       int               `json:"max_tokens,omitempty"`
}

type chatChoice struct {
	Message chatWireMessage `json:"message"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type deepSeekResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type deepSeekContinuation struct {
	ReasoningByCall map[string]string `json:"reasoning_by_call,omitempty"`
}

func (d *DeepSeekAdapter) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if err := validateContinuation(req.Continuation, d.Name(), d.Model()); err != nil {
		return GenerateResponse{}, err
	}
	if !d.capabilities.ToolCalls && len(req.Tools) > 0 {
		return GenerateResponse{}, fmt.Errorf("%w: model tool capability is unknown", ErrBadRequest)
	}
	continuation, err := decodeDeepSeekContinuation(req.Continuation)
	if err != nil {
		return GenerateResponse{}, err
	}
	messages, err := chatMessages(ctx, req.Messages, req.ImageResolver, d.capabilities, continuation.ReasoningByCall)
	if err != nil {
		return GenerateResponse{}, err
	}
	body := deepSeekRequest{
		Model: d.model, Messages: messages, Tools: chatTools(req.Tools), MaxTokens: req.MaxOutputTokens,
	}
	if d.capabilities.Reasoning {
		if req.Reasoning == ReasoningDisabled {
			body.Thinking = map[string]string{"type": "disabled"}
		} else {
			body.ReasoningEffort = "high"
			body.Thinking = map[string]string{"type": "enabled"}
		}
	}
	var wire deepSeekResponse
	if err := d.http.doJSON(ctx, "/v1/chat/completions", body, req.OnRetry, &wire); err != nil {
		return GenerateResponse{}, err
	}
	if len(wire.Choices) == 0 {
		return GenerateResponse{}, fmt.Errorf("%w: empty provider response", ErrUnavailable)
	}
	message := wire.Choices[0].Message
	toolCalls, err := normalizedChatToolCalls(message.ToolCalls)
	if err != nil {
		return GenerateResponse{}, err
	}
	if len(toolCalls) > 0 && message.ReasoningContent != "" {
		if continuation.ReasoningByCall == nil {
			continuation.ReasoningByCall = map[string]string{}
		}
		continuation.ReasoningByCall[toolCalls[0].ID] = message.ReasoningContent
	}
	next, err := encodeDeepSeekContinuation(d.Name(), d.Model(), continuation)
	if err != nil {
		return GenerateResponse{}, err
	}
	return GenerateResponse{
		Content: TextContent(chatWireText(message.Content)), ToolCalls: toolCalls,
		Continuation: next,
		Usage: Usage{
			InputTokens: wire.Usage.PromptTokens, OutputTokens: wire.Usage.CompletionTokens,
			TotalTokens: wire.Usage.TotalTokens,
		},
	}, nil
}

func decodeDeepSeekContinuation(value *ProviderContinuation) (deepSeekContinuation, error) {
	out := deepSeekContinuation{ReasoningByCall: map[string]string{}}
	if value == nil || len(value.Opaque) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(value.Opaque, &out); err != nil {
		return deepSeekContinuation{}, fmt.Errorf("%w: invalid continuation", ErrBadRequest)
	}
	if out.ReasoningByCall == nil {
		out.ReasoningByCall = map[string]string{}
	}
	return out, nil
}

func encodeDeepSeekContinuation(provider, model string, value deepSeekContinuation) (*ProviderContinuation, error) {
	if len(value.ReasoningByCall) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: encode continuation", ErrBadRequest)
	}
	return &ProviderContinuation{Provider: provider, Model: model, Opaque: raw}, nil
}

func chatMessages(
	ctx context.Context,
	messages []Message,
	resolver ImageRefResolver,
	capabilities Capabilities,
	reasoningByCall map[string]string,
) ([]chatWireMessage, error) {
	out := make([]chatWireMessage, len(messages))
	for i, message := range messages {
		out[i] = chatWireMessage{Role: string(message.Role), ToolCallID: message.ToolCallID}
		if len(message.ToolCalls) > 0 {
			out[i].ToolCalls = make([]chatToolCall, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				out[i].ToolCalls = append(out[i].ToolCalls, toChatToolCall(call))
			}
			out[i].ReasoningContent = reasoningByCall[message.ToolCalls[0].ID]
		}
		parts := make([]chatContentPart, 0, len(message.Content))
		hasImage := false
		for _, part := range message.Content {
			switch part.Type {
			case "text":
				parts = append(parts, chatContentPart{Type: "text", Text: part.Text})
			case "image":
				hasImage = true
				if !capabilities.Vision {
					return nil, fmt.Errorf("%w: model does not support image input", ErrBadRequest)
				}
				if resolver == nil {
					return nil, fmt.Errorf("%w: image resolver is required", ErrBadRequest)
				}
				data, err := resolver.ResolveImage(ctx, part.ImageRef)
				if err != nil {
					if errors.Is(err, context.Canceled) || ctx.Err() != nil {
						return nil, ctx.Err()
					}
					return nil, fmt.Errorf("%w: image reference is not authorized", ErrBadRequest)
				}
				raw, mimeType, err := prepareProviderImage(data, capabilities)
				if err != nil {
					return nil, fmt.Errorf("%w: invalid image input", ErrBadRequest)
				}
				detail := part.Detail
				if detail != "low" && detail != "high" {
					detail = "high"
				}
				parts = append(parts, chatContentPart{
					Type: "image_url",
					ImageURL: &chatImageURL{
						URL:    "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(raw),
						Detail: detail,
					},
				})
			}
		}
		if hasImage {
			out[i].Content = parts
		} else {
			out[i].Content = message.Text()
		}
	}
	return out, nil
}

func toChatToolCall(call ToolCall) chatToolCall {
	var out chatToolCall
	out.ID, out.Type = call.ID, "function"
	out.Function.Name = call.Name
	out.Function.Arguments = encodeToolArguments(call.Args)
	return out
}

func chatTools(tools []ToolSchema) []chatTool {
	out := make([]chatTool, len(tools))
	for i, tool := range tools {
		out[i].Type = "function"
		out[i].Function.Name = tool.Name
		out[i].Function.Description = tool.Description
		out[i].Function.Parameters = tool.Parameters
	}
	return out
}

func normalizedChatToolCalls(calls []chatToolCall) ([]ToolCall, error) {
	out := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		if strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Function.Name) == "" {
			return nil, fmt.Errorf("%w: missing tool call identity", ErrBadToolCall)
		}
		args, err := decodeToolArguments(call.Function.Arguments)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid tool arguments", ErrBadToolCall)
		}
		out = append(out, ToolCall{ID: call.ID, Name: call.Function.Name, Args: args})
	}
	return out, nil
}

func chatWireText(value any) string {
	text, _ := value.(string)
	return text
}
