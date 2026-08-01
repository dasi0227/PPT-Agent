package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeepSeekConfig 来自环境变量装配（config 包），Key 不落日志（ARCH-LLM-003）。
type DeepSeekConfig struct {
	APIKey  string
	BaseURL string // 默认 https://api.deepseek.com
	Model   string // 默认 deepseek-chat
	Timeout time.Duration
}

// DeepSeek 实现 Client，对接 OpenAI 兼容的 Chat Completions + tools 端点。
type DeepSeek struct {
	cfg        DeepSeekConfig
	httpClient *http.Client
	maxRetries int
}

var _ Client = (*DeepSeek)(nil)

func NewDeepSeek(cfg DeepSeekConfig) *DeepSeek {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	if cfg.Timeout == 0 {
		// 覆盖 chat completions 从建连、TLS 到 decode body 的整体耗时。长 prompt +
		// tools 场景下 60s 常常在 decode 阶段被 kill；180s 与 config 默认值保持一致。
		cfg.Timeout = 180 * time.Second
	}
	return &DeepSeek{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		maxRetries: 3,
	}
}

// ── 线路层类型（OpenAI 兼容） ─────────────────────────────

type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type wireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type wireRequest struct {
	Model    string        `json:"model"`
	Messages []wireMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Tools    []wireTool    `json:"tools,omitempty"`
}

type wireChoice struct {
	Delta   wireMessage `json:"delta"`
	Message wireMessage `json:"message"`
}

type wireResponse struct {
	Choices []wireChoice `json:"choices"`
}

func toWireMessages(msgs []Message) []wireMessage {
	out := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		out[i] = wireMessage{Role: string(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		if len(m.ToolCalls) > 0 {
			out[i].ToolCalls = make([]wireToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				out[i].ToolCalls = append(out[i].ToolCalls, toWireToolCall(tc))
			}
		}
	}
	return out
}

func toWireToolCall(tc ToolCall) wireToolCall {
	var out wireToolCall
	out.ID = tc.ID
	out.Type = "function"
	out.Function.Name = tc.Name
	out.Function.Arguments = encodeToolArguments(tc.Args)
	return out
}

func encodeToolArguments(args map[string]any) string {
	if args == nil {
		return "{}"
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func toWireTools(tools []ToolSchema) []wireTool {
	out := make([]wireTool, len(tools))
	for i, t := range tools {
		out[i].Type = "function"
		out[i].Function.Name = t.Name
		out[i].Function.Description = t.Description
		out[i].Function.Parameters = t.Parameters
	}
	return out
}

// ── Client 实现 ──────────────────────────────────────────

func (d *DeepSeek) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := wireRequest{Model: d.cfg.Model, Messages: toWireMessages(req.Messages)}
	resp, err := d.doJSON(ctx, body)
	if err != nil {
		return ChatResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("%w: empty choices", ErrUnavailable)
	}
	return ChatResponse{Content: resp.Choices[0].Message.Content}, nil
}

func (d *DeepSeek) CallTool(ctx context.Context, req ToolCallRequest) (ToolCallResponse, error) {
	body := wireRequest{
		Model:    d.cfg.Model,
		Messages: toWireMessages(req.Messages),
		Tools:    toWireTools(req.Tools),
	}
	resp, err := d.doJSON(ctx, body)
	if err != nil {
		return ToolCallResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return ToolCallResponse{}, fmt.Errorf("%w: empty choices", ErrUnavailable)
	}
	msg := resp.Choices[0].Message
	out := ToolCallResponse{Text: msg.Content}
	if len(msg.ToolCalls) > 0 {
		tc := msg.ToolCalls[0]
		args := map[string]any{}
		// function call 参数无法解析 → 最小失败退出（ARCH-LLM-FC-001）。
		if strings.TrimSpace(tc.Function.Arguments) != "" {
			if err := decodeToolArguments(tc.Function.Arguments, &args); err != nil {
				return ToolCallResponse{}, fmt.Errorf("%w: %v", ErrBadToolCall, err)
			}
		}
		if tc.Function.Name == "" {
			return ToolCallResponse{}, fmt.Errorf("%w: missing tool name", ErrBadToolCall)
		}
		out.ToolCall = &ToolCall{ID: tc.ID, Name: tc.Function.Name, Args: args}
	}
	return out, nil
}

func decodeToolArguments(s string, out *map[string]any) error {
	if err := json.Unmarshal([]byte(s), out); err == nil {
		return nil
	}

	decoder := json.NewDecoder(strings.NewReader(s))
	if err := decoder.Decode(out); err != nil {
		return err
	}
	rest := strings.TrimSpace(s[decoder.InputOffset():])
	for _, r := range rest {
		if r != '}' {
			return fmt.Errorf("unexpected trailing tool arguments: %q", rest)
		}
	}
	return nil
}

// Stream 流式补全；返回的 channel 在 ctx 取消或流结束时关闭，goroutine 及时退出（ARCH-LLM-002）。
func (d *DeepSeek) Stream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	body := wireRequest{Model: d.cfg.Model, Messages: toWireMessages(req.Messages), Stream: true}
	httpResp, err := d.doRaw(ctx, body)
	if err != nil {
		return nil, err
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		defer httpResp.Body.Close()

		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			// ctx 取消：立即停止读取，goroutine 退出，无泄漏。
			select {
			case <-ctx.Done():
				trySend(ctx, out, StreamChunk{Done: true, Err: ctx.Err()})
				return
			default:
			}
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				trySend(ctx, out, StreamChunk{Done: true})
				return
			}
			var chunk wireResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				if !trySend(ctx, out, StreamChunk{Text: chunk.Choices[0].Delta.Content}) {
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			trySend(ctx, out, StreamChunk{Done: true, Err: err})
			return
		}
		trySend(ctx, out, StreamChunk{Done: true})
	}()
	return out, nil
}

// trySend 在 ctx 未取消时投递；取消则返回 false，让发送方立即退出（防泄漏）。
func trySend(ctx context.Context, out chan<- StreamChunk, c StreamChunk) bool {
	select {
	case out <- c:
		return true
	case <-ctx.Done():
		return false
	}
}

// ── HTTP 与重试 ──────────────────────────────────────────

func (d *DeepSeek) doJSON(ctx context.Context, body wireRequest) (wireResponse, error) {
	httpResp, err := d.doRaw(ctx, body)
	if err != nil {
		return wireResponse{}, err
	}
	defer httpResp.Body.Close()
	var out wireResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return wireResponse{}, fmt.Errorf("%w: decode: %v", ErrUnavailable, err)
	}
	return out, nil
}

// doRaw 执行请求，含重试：429/5xx 退避重试(≤3)，4xx 不重试映射 ErrBadRequest（ARCH-LLM 重试表）。
func (d *DeepSeek) doRaw(ctx context.Context, body wireRequest) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(d.cfg.BaseURL, "/") + "/v1/chat/completions"

	var lastErr error
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+d.cfg.APIKey)

		resp, err := d.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = fmt.Errorf("%w: %v", ErrUnavailable, err)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		drainClose(resp.Body)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
			continue
		}
		// 4xx（非 429）：不重试。错误消息不含 Authorization 头（ARCH-LLM-003）。
		return nil, fmt.Errorf("%w: status %d", ErrBadRequest, resp.StatusCode)
	}
	return nil, lastErr
}

func backoff(attempt int) time.Duration {
	return time.Duration(attempt*attempt) * 200 * time.Millisecond
}

func drainClose(rc io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(rc, 1<<16))
	_ = rc.Close()
}
