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

type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// OpenAIAdapter maps normalized Runtime messages to the Responses API.
type OpenAIAdapter struct {
	model        string
	capabilities Capabilities
	http         adapterHTTP
}

var _ Provider = (*OpenAIAdapter)(nil)

func NewOpenAIAdapter(cfg OpenAIConfig) *OpenAIAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	return &OpenAIAdapter{
		model: cfg.Model, capabilities: capabilitiesFor("openai", cfg.Model),
		http: newAdapterHTTP(cfg.APIKey, cfg.BaseURL, cfg.Timeout),
	}
}

func (o *OpenAIAdapter) Name() string  { return "openai" }
func (o *OpenAIAdapter) Model() string { return o.model }
func (o *OpenAIAdapter) String() string {
	return fmt.Sprintf("OpenAIAdapter{Model:%q}", o.model)
}
func (o *OpenAIAdapter) GoString() string { return o.String() }
func (o *OpenAIAdapter) Capabilities() Capabilities {
	return cloneCapabilities(o.capabilities)
}

type responsesRequest struct {
	Model              string          `json:"model"`
	Instructions       string          `json:"instructions,omitempty"`
	Input              []any           `json:"input"`
	Tools              []responsesTool `json:"tools,omitempty"`
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
}

type responsesTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type responsesOutputItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type responsesResponse struct {
	ID     string                `json:"id"`
	Output []responsesOutputItem `json:"output"`
	Usage  responsesUsage        `json:"usage"`
}

type openAIContinuation struct {
	PreviousResponseID string `json:"previous_response_id"`
	MessageCount       int    `json:"message_count"`
}

func (o *OpenAIAdapter) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if err := validateContinuation(req.Continuation, o.Name(), o.Model()); err != nil {
		return GenerateResponse{}, err
	}
	if !o.capabilities.ToolCalls && len(req.Tools) > 0 {
		return GenerateResponse{}, fmt.Errorf("%w: model tool capability is unknown", ErrBadRequest)
	}
	continuation, err := decodeOpenAIContinuation(req.Continuation)
	if err != nil {
		return GenerateResponse{}, err
	}
	start := 0
	if continuation.PreviousResponseID != "" {
		if continuation.MessageCount < 0 || continuation.MessageCount > len(req.Messages) {
			// Compaction changed the normalized history. Fall back to a full
			// stateless request instead of attaching mismatched response state.
			continuation = openAIContinuation{}
		} else {
			start = continuation.MessageCount
		}
	}
	instructions := collectSystemInstructions(req.Messages)
	input, err := o.responsesInput(ctx, req.Messages[start:], req.ImageResolver, continuation.PreviousResponseID != "")
	if err != nil {
		return GenerateResponse{}, err
	}
	body := responsesRequest{
		Model: o.model, Instructions: instructions, Input: input,
		Tools: responsesTools(req.Tools), PreviousResponseID: continuation.PreviousResponseID,
	}
	var wire responsesResponse
	if err := o.http.doJSON(ctx, "/v1/responses", body, req.OnRetry, &wire); err != nil {
		return GenerateResponse{}, err
	}
	content := make([]ContentPart, 0)
	toolCalls := make([]ToolCall, 0)
	for _, item := range wire.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" && part.Text != "" {
					content = append(content, ContentPart{Type: "text", Text: part.Text})
				}
			}
		case "function_call":
			if strings.TrimSpace(item.CallID) == "" || strings.TrimSpace(item.Name) == "" {
				return GenerateResponse{}, fmt.Errorf("%w: missing tool call identity", ErrBadToolCall)
			}
			args, err := decodeToolArguments(item.Arguments)
			if err != nil {
				return GenerateResponse{}, fmt.Errorf("%w: invalid tool arguments", ErrBadToolCall)
			}
			toolCalls = append(toolCalls, ToolCall{ID: item.CallID, Name: item.Name, Args: args})
		}
	}
	var next *ProviderContinuation
	if wire.ID != "" {
		raw, err := json.Marshal(openAIContinuation{
			PreviousResponseID: wire.ID, MessageCount: len(req.Messages),
		})
		if err != nil {
			return GenerateResponse{}, fmt.Errorf("%w: encode continuation", ErrBadRequest)
		}
		next = &ProviderContinuation{Provider: o.Name(), Model: o.Model(), Opaque: raw}
	}
	return GenerateResponse{
		Content: content, ToolCalls: toolCalls, Continuation: next,
		Usage: Usage{
			InputTokens: wire.Usage.InputTokens, OutputTokens: wire.Usage.OutputTokens,
			TotalTokens: wire.Usage.TotalTokens,
		},
	}, nil
}

func decodeOpenAIContinuation(value *ProviderContinuation) (openAIContinuation, error) {
	if value == nil || len(value.Opaque) == 0 {
		return openAIContinuation{}, nil
	}
	var out openAIContinuation
	if err := json.Unmarshal(value.Opaque, &out); err != nil {
		return openAIContinuation{}, fmt.Errorf("%w: invalid continuation", ErrBadRequest)
	}
	return out, nil
}

func collectSystemInstructions(messages []Message) string {
	var instructions []string
	for _, message := range messages {
		if message.Role == RoleSystem && strings.TrimSpace(message.Text()) != "" {
			instructions = append(instructions, message.Text())
		}
	}
	return strings.Join(instructions, "\n\n")
}

func (o *OpenAIAdapter) responsesInput(
	ctx context.Context,
	messages []Message,
	resolver ImageRefResolver,
	nativeContinuation bool,
) ([]any, error) {
	out := make([]any, 0, len(messages))
	for _, message := range messages {
		if message.Role == RoleSystem {
			continue
		}
		if nativeContinuation && message.Role == RoleAssistant {
			// The prior Responses output is already referenced by
			// previous_response_id.
			continue
		}
		switch message.Role {
		case RoleTool:
			output, err := o.responsesContent(ctx, message.Content, resolver)
			if err != nil {
				return nil, err
			}
			var value any = ""
			if len(output) == 1 {
				if text, ok := output[0].(map[string]any); ok && text["type"] == "input_text" {
					value = text["text"]
				} else {
					value = output
				}
			} else if len(output) > 1 {
				value = output
			}
			out = append(out, map[string]any{
				"type": "function_call_output", "call_id": message.ToolCallID, "output": value,
			})
		case RoleAssistant:
			if message.Text() != "" {
				out = append(out, map[string]any{
					"role":    "assistant",
					"content": []any{map[string]any{"type": "output_text", "text": message.Text()}},
				})
			}
			for _, call := range message.ToolCalls {
				out = append(out, map[string]any{
					"type": "function_call", "call_id": call.ID,
					"name": call.Name, "arguments": encodeToolArguments(call.Args),
				})
			}
		default:
			content, err := o.responsesContent(ctx, message.Content, resolver)
			if err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"role": string(message.Role), "content": content})
		}
	}
	return out, nil
}

func (o *OpenAIAdapter) responsesContent(
	ctx context.Context,
	parts []ContentPart,
	resolver ImageRefResolver,
) ([]any, error) {
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			out = append(out, map[string]any{"type": "input_text", "text": part.Text})
		case "image":
			if !o.capabilities.Vision {
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
			raw, mimeType, err := prepareProviderImage(data, o.capabilities)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid image input", ErrBadRequest)
			}
			detail := part.Detail
			if detail != "low" && detail != "high" {
				detail = "auto"
			}
			out = append(out, map[string]any{
				"type":      "input_image",
				"image_url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(raw),
				"detail":    detail,
			})
		}
	}
	return out, nil
}

func responsesTools(tools []ToolSchema) []responsesTool {
	out := make([]responsesTool, len(tools))
	for i, tool := range tools {
		out[i] = responsesTool{
			Type: "function", Name: tool.Name,
			Description: tool.Description, Parameters: tool.Parameters,
		}
	}
	return out
}
