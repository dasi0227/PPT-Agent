package llm

import (
	"context"
	"encoding/base64"
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
		model: cfg.Model, capabilities: productCapabilities(),
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
	Role       string         `json:"role"`
	Content    any            `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
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
	Model     string            `json:"model"`
	Messages  []chatWireMessage `json:"messages"`
	Tools     []chatTool        `json:"tools,omitempty"`
	Thinking  map[string]string `json:"thinking,omitempty"`
	MaxTokens int               `json:"max_tokens,omitempty"`
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

func (d *DeepSeekAdapter) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if err := validateContinuation(req.Continuation, d.Name(), d.Model()); err != nil {
		return GenerateResponse{}, err
	}
	messages, err := chatMessages(ctx, req.Messages, req.ImageResolver, d.capabilities)
	if err != nil {
		return GenerateResponse{}, err
	}
	body := deepSeekRequest{
		Model: d.model, Messages: messages, Tools: chatTools(req.Tools), MaxTokens: req.MaxOutputTokens,
		Thinking: map[string]string{"type": "disabled"},
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
	return GenerateResponse{
		Content: TextContent(chatWireText(message.Content)), ToolCalls: toolCalls,
		Usage: Usage{
			InputTokens: wire.Usage.PromptTokens, OutputTokens: wire.Usage.CompletionTokens,
			TotalTokens: wire.Usage.TotalTokens,
		},
	}, nil
}

func chatMessages(
	ctx context.Context,
	messages []Message,
	resolver ImageRefResolver,
	capabilities Capabilities,
) ([]chatWireMessage, error) {
	out := make([]chatWireMessage, len(messages))
	for i, message := range messages {
		out[i] = chatWireMessage{Role: string(message.Role), ToolCallID: message.ToolCallID}
		if len(message.ToolCalls) > 0 {
			out[i].ToolCalls = make([]chatToolCall, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				out[i].ToolCalls = append(out[i].ToolCalls, toChatToolCall(call))
			}
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
					return nil, fmt.Errorf("%w: image resolver is required", ErrImageReference)
				}
				data, err := resolver.ResolveImage(ctx, part.ImageRef)
				if err != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return nil, err
					}
					return nil, fmt.Errorf("%w: %v", ErrImageReference, err)
				}
				raw, mimeType, err := prepareProviderImage(data, capabilities)
				if err != nil {
					return nil, fmt.Errorf("%w: invalid image input", ErrImageReference)
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
