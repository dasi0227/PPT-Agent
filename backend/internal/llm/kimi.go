package llm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type KimiConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// KimiAdapter owns Kimi's OpenAI-compatible Chat Completions mapping. It is
// intentionally separate from DeepSeek so model parameters and error behavior
// can evolve without coupling the providers.
type KimiAdapter struct {
	model        string
	capabilities Capabilities
	http         adapterHTTP
}

var _ Provider = (*KimiAdapter)(nil)

func NewKimiAdapter(cfg KimiConfig) *KimiAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.moonshot.cn/v1"
	}
	return &KimiAdapter{
		model: cfg.Model, capabilities: capabilitiesFor("kimi", cfg.Model),
		http: newAdapterHTTP(cfg.APIKey, cfg.BaseURL, cfg.Timeout),
	}
}

func (k *KimiAdapter) Name() string  { return "kimi" }
func (k *KimiAdapter) Model() string { return k.model }
func (k *KimiAdapter) String() string {
	return fmt.Sprintf("KimiAdapter{Model:%q}", k.model)
}
func (k *KimiAdapter) GoString() string { return k.String() }
func (k *KimiAdapter) Capabilities() Capabilities {
	return cloneCapabilities(k.capabilities)
}

type kimiRequest struct {
	Model    string            `json:"model"`
	Messages []chatWireMessage `json:"messages"`
	Tools    []chatTool        `json:"tools,omitempty"`
	Thinking map[string]string `json:"thinking,omitempty"`
}

type kimiResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

func (k *KimiAdapter) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if err := validateContinuation(req.Continuation, k.Name(), k.Model()); err != nil {
		return GenerateResponse{}, err
	}
	if req.Continuation != nil && len(req.Continuation.Opaque) > 0 {
		return GenerateResponse{}, fmt.Errorf("%w: Kimi continuation is unsupported", ErrBadRequest)
	}
	if !k.capabilities.ToolCalls && len(req.Tools) > 0 {
		return GenerateResponse{}, fmt.Errorf("%w: model tool capability is unknown", ErrBadRequest)
	}
	messages, err := kimiMessages(ctx, req.Messages, req.ImageResolver, k.capabilities)
	if err != nil {
		return GenerateResponse{}, err
	}
	body := kimiRequest{
		Model: k.model, Messages: messages, Tools: chatTools(req.Tools),
	}
	// Kimi reasoning parameters are provider-specific. The Runtime does not
	// consume hidden reasoning, so the initial adapter mode keeps it disabled.
	if k.capabilities.Reasoning {
		body.Thinking = map[string]string{"type": "disabled"}
	}
	var wire kimiResponse
	if err := k.http.doJSON(ctx, "/v1/chat/completions", body, req.OnRetry, &wire); err != nil {
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

// Kimi accepts tool results as role=tool text messages and multimodal input as
// role=user content. Keep the tool result pairing intact, then add each
// authorized screenshot as a following visual observation.
func kimiMessages(
	ctx context.Context,
	messages []Message,
	resolver ImageRefResolver,
	capabilities Capabilities,
) ([]chatWireMessage, error) {
	wire, err := chatMessages(ctx, messages, resolver, capabilities, nil)
	if err != nil {
		return nil, err
	}
	toolNames := make(map[string]string)
	for _, message := range messages {
		for _, call := range message.ToolCalls {
			toolNames[call.ID] = call.Name
		}
	}
	out := make([]chatWireMessage, 0, len(wire))
	for index := 0; index < len(wire); {
		if wire[index].Role != string(RoleTool) {
			out = append(out, wire[index])
			index++
			continue
		}
		end := index
		for end < len(wire) && wire[end].Role == string(RoleTool) {
			end++
		}
		visuals := make([]chatWireMessage, 0, end-index)
		for current := index; current < end; current++ {
			message := wire[current]
			message.Name = toolNames[message.ToolCallID]
			parts, multimodal := message.Content.([]chatContentPart)
			if !multimodal {
				out = append(out, message)
				continue
			}
			texts := make([]string, 0, len(parts))
			images := make([]chatContentPart, 0, len(parts))
			for _, part := range parts {
				if part.Type == "text" {
					if strings.TrimSpace(part.Text) != "" {
						texts = append(texts, part.Text)
					}
				} else if part.Type == "image_url" {
					images = append(images, part)
				}
			}
			message.Content = strings.Join(texts, "\n")
			out = append(out, message)
			if len(images) > 0 {
				content := []chatContentPart{{
					Type: "text",
					Text: "Visual observation for tool call " + message.ToolCallID + ":",
				}}
				content = append(content, images...)
				visuals = append(visuals, chatWireMessage{
					Role: string(RoleUser), Content: content,
				})
			}
		}
		out = append(out, visuals...)
		index = end
	}
	return out, nil
}
