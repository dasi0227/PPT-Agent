package sessionstate

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

// PendingModelCall is a control-flow reference. Request/response bodies live in
// one private snapshot row and never enter public DTOs or checkpoint JSON.
type PendingModelCall struct {
	ID              string `json:"id"`
	ContextRevision int64  `json:"context_revision"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Stage           string `json:"stage"`
}

type ModelRequestSnapshot struct {
	ID              string
	SceneRevision   int64
	Provider        string
	Model           string
	ContextRevision int64
	RequestJSON     json.RawMessage
	ResponseJSON    json.RawMessage
}

// Only normalized request data is persisted. Transport, callbacks, resolvers,
// credentials and learned adapter constraint policy are process-local.
type ModelRequestBody struct {
	FormatVersion     int                       `json:"format_version"`
	Messages          []NativeMessage           `json:"messages"`
	Tools             []llm.ToolSchema          `json:"tools"`
	RequiredTool      string                    `json:"required_tool,omitempty"`
	ParallelToolCalls *bool                     `json:"parallel_tool_calls,omitempty"`
	Continuation      *llm.ProviderContinuation `json:"continuation,omitempty"`
	MaxOutputTokens   int                       `json:"max_output_tokens"`
	PauseOnFallback   bool                      `json:"pause_on_fallback"`
}

func EncodeModelRequest(request llm.GenerateRequest) (json.RawMessage, error) {
	body := ModelRequestBody{FormatVersion: 1, Messages: []NativeMessage{}, Tools: request.Tools, RequiredTool: request.RequiredTool, ParallelToolCalls: request.ParallelToolCalls, Continuation: request.Continuation, MaxOutputTokens: request.MaxOutputTokens, PauseOnFallback: request.PauseOnFallback}
	for _, message := range request.Messages {
		body.Messages = append(body.Messages, NativeMessage{FormatVersion: 1, Role: message.Role, Content: message.Content, ToolCallID: message.ToolCallID, ToolCalls: message.ToolCalls, Metadata: message.Metadata, Replay: message.Replay})
	}
	return json.Marshal(body)
}

func DecodeModelRequest(raw json.RawMessage, live llm.GenerateRequest) (llm.GenerateRequest, error) {
	var body ModelRequestBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return live, err
	}
	if body.FormatVersion != 1 || len(body.Messages) == 0 {
		return live, ErrInvalidState
	}
	live.Messages = make([]llm.Message, 0, len(body.Messages))
	for _, message := range body.Messages {
		if message.FormatVersion != 1 {
			return live, ErrInvalidState
		}
		switch message.Role {
		case llm.RoleSystem, llm.RoleUser, llm.RoleAssistant, llm.RoleTool:
		default:
			return live, ErrInvalidState
		}
		live.Messages = append(live.Messages, llm.Message{Role: message.Role, Content: message.Content, ToolCallID: message.ToolCallID, ToolCalls: message.ToolCalls, Metadata: message.Metadata, Replay: message.Replay})
	}
	live.Tools, live.RequiredTool, live.ParallelToolCalls = body.Tools, body.RequiredTool, body.ParallelToolCalls
	live.Continuation, live.MaxOutputTokens, live.PauseOnFallback = body.Continuation, body.MaxOutputTokens, body.PauseOnFallback
	return live, nil
}

type modelResponseBody struct {
	FormatVersion int                       `json:"format_version"`
	Thinking      []string                  `json:"thinking"`
	Replay        *llm.AssistantReplay      `json:"assistant_replay,omitempty"`
	Content       []llm.ContentPart         `json:"content"`
	ToolCalls     []llm.ToolCall            `json:"tool_calls"`
	Continuation  *llm.ProviderContinuation `json:"continuation,omitempty"`
	Usage         llm.Usage                 `json:"usage"`
}

func EncodeModelResponse(response llm.GenerateResponse) (json.RawMessage, error) {
	return json.Marshal(modelResponseBody{FormatVersion: 1, Thinking: response.Thinking, Replay: response.Replay, Content: response.Content, ToolCalls: response.ToolCalls, Continuation: response.Continuation, Usage: response.Usage})
}
func DecodeModelResponse(raw json.RawMessage) (llm.GenerateResponse, error) {
	var body modelResponseBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return llm.GenerateResponse{}, err
	}
	if body.FormatVersion != 1 {
		return llm.GenerateResponse{}, ErrInvalidState
	}
	return llm.GenerateResponse{Thinking: body.Thinking, Replay: body.Replay, Content: body.Content, ToolCalls: body.ToolCalls, Continuation: body.Continuation, Usage: body.Usage}, nil
}
