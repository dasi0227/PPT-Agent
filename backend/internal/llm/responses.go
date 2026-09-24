package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// ResponsesAdapter maps normalized Runtime messages to the Responses API.
type ResponsesAdapter struct {
	provider     string
	model        string
	capabilities Capabilities
	http         adapterHTTP
}

var _ Provider = (*ResponsesAdapter)(nil)

func NewResponsesAdapter(cfg AdapterConfig) *ResponsesAdapter {
	return &ResponsesAdapter{
		provider: cfg.Provider, model: cfg.Model, capabilities: productCapabilities(),
		http: newAdapterHTTP(cfg.APIKey, cfg.BaseURL, cfg.Timeout),
	}
}

func (o *ResponsesAdapter) Name() string  { return o.provider }
func (o *ResponsesAdapter) Model() string { return o.model }
func (o *ResponsesAdapter) String() string {
	return fmt.Sprintf("ResponsesAdapter{Model:%q}", o.model)
}
func (o *ResponsesAdapter) GoString() string { return o.String() }
func (o *ResponsesAdapter) Capabilities() Capabilities {
	return cloneCapabilities(o.capabilities)
}

type responsesRequest struct {
	Model           string          `json:"model"`
	Instructions    string          `json:"instructions,omitempty"`
	Input           []any           `json:"input"`
	Tools           []responsesTool `json:"tools,omitempty"`
	Store           bool            `json:"store"`
	Include         []string        `json:"include,omitempty"`
	MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
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
	ID     string            `json:"id"`
	Output []json.RawMessage `json:"output"`
	Status string            `json:"status"`
	Usage  responsesUsage    `json:"usage"`
}

func (o *ResponsesAdapter) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if err := validateContinuation(req.Continuation, o.Name(), o.Model()); err != nil {
		return GenerateResponse{}, err
	}
	replay, err := responsesReplay(req, o.http.baseURL)
	if err != nil {
		return GenerateResponse{}, err
	}
	instructions := collectSystemInstructions(req.Messages)
	input, err := o.responsesInput(ctx, req.Messages, req.ImageResolver, replay)
	if err != nil {
		return GenerateResponse{}, err
	}
	body := responsesRequest{
		Model: o.model, Instructions: instructions, Input: input,
		Tools: responsesTools(req.Tools), Store: false, Include: []string{"reasoning.encrypted_content"},
		MaxOutputTokens: req.MaxOutputTokens,
	}
	var wire responsesResponse
	if err := o.http.doJSON(ctx, "/responses", body, req.OnRetry, &wire); err != nil {
		return GenerateResponse{}, err
	}
	if wire.Status == "failed" || wire.Status == "incomplete" || len(wire.Output) == 0 {
		return GenerateResponse{}, fmt.Errorf("%w: response is incomplete or empty", ErrUnavailable)
	}
	content := make([]ContentPart, 0)
	toolCalls := make([]ToolCall, 0)
	for _, raw := range wire.Output {
		var item responsesOutputItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return GenerateResponse{}, fmt.Errorf("%w: invalid response item", ErrUnavailable)
		}
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
	if len(content) == 0 && len(toolCalls) == 0 {
		return GenerateResponse{}, fmt.Errorf("%w: response contains no text or tool calls", ErrUnavailable)
	}
	next := newResponsesState(req, replay, wire.Output, content, toolCalls, o.provider, o.model, o.http.baseURL)
	return GenerateResponse{
		Content: content, ToolCalls: toolCalls, Continuation: next,
		Usage: Usage{
			InputTokens: wire.Usage.InputTokens, OutputTokens: wire.Usage.OutputTokens,
			TotalTokens: wire.Usage.TotalTokens,
		},
	}, nil
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

func (o *ResponsesAdapter) responsesInput(
	ctx context.Context,
	messages []Message,
	resolver ImageRefResolver,
	replay []responsesTurn,
) ([]any, error) {
	out := make([]any, 0, len(messages))
	for index, message := range messages {
		if message.Role == RoleSystem {
			continue
		}
		if items := replayItems(replay, index); len(items) > 0 {
			for _, item := range items {
				out = append(out, item)
			}
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

func (o *ResponsesAdapter) responsesContent(
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
			raw, mimeType, err := resolveProviderImage(ctx, part.ImageRef, resolver, o.capabilities)
			if err != nil {
				return nil, err
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
			Description: tool.Description, Parameters: toolParameters(tool.Parameters),
		}
	}
	return out
}
