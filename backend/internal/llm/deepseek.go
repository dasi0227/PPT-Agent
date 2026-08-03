package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeepSeekConfig 来自环境变量装配（config 包），Key 不落日志（ARCH-LLM-003）。
type DeepSeekConfig struct {
	APIKey          string
	BaseURL         string // 默认 https://api.deepseek.com
	Model           string // 默认 deepseek-chat
	Timeout         time.Duration
	Vision          bool
	ImageInputMIMEs []string
	MaxImageBytes   int
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
	if len(cfg.ImageInputMIMEs) == 0 {
		cfg.ImageInputMIMEs = []string{"image/png", "image/jpeg"}
	}
	if cfg.MaxImageBytes <= 0 {
		cfg.MaxImageBytes = 4 * 1024 * 1024
	}
	return &DeepSeek{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		maxRetries: 3,
	}
}

func (d *DeepSeek) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Vision: d.cfg.Vision, MultipleToolCalls: true,
		ImageInputMIMEs: append([]string(nil), d.cfg.ImageInputMIMEs...),
		MaxImageBytes:   d.cfg.MaxImageBytes,
	}
}

// ── 线路层类型（OpenAI 兼容） ─────────────────────────────

type wireMessage struct {
	Role             string         `json:"role"`
	Content          any            `json:"content"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	ToolCalls        []wireToolCall `json:"tool_calls,omitempty"`
}

type wireContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *wireImageURL `json:"image_url,omitempty"`
}

type wireImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
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

func (d *DeepSeek) toWireMessages(ctx context.Context, msgs []Message, resolver ImageRefResolver) ([]wireMessage, error) {
	out := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		out[i] = wireMessage{
			Role:             string(m.Role),
			ReasoningContent: m.ReasoningContent, ToolCallID: m.ToolCallID,
		}
		parts := make([]wireContentPart, 0, len(m.Content))
		hasImage := false
		for _, part := range m.Content {
			switch part.Type {
			case "text":
				parts = append(parts, wireContentPart{Type: "text", Text: part.Text})
			case "image":
				hasImage = true
				if !d.cfg.Vision {
					return nil, fmt.Errorf("%w: VISION_CAPABILITY_REQUIRED", ErrBadRequest)
				}
				if resolver == nil {
					return nil, fmt.Errorf("%w: image resolver is required", ErrBadRequest)
				}
				imageData, err := resolver.ResolveImage(ctx, part.ImageRef)
				if err != nil {
					return nil, fmt.Errorf("%w: resolve image: %v", ErrBadRequest, err)
				}
				raw, mimeType, err := prepareProviderImage(imageData, d.Capabilities())
				if err != nil {
					return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
				}
				detail := part.Detail
				if detail != "low" && detail != "high" {
					detail = "high"
				}
				parts = append(parts, wireContentPart{
					Type:     "image_url",
					ImageURL: &wireImageURL{URL: "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(raw), Detail: detail},
				})
			}
		}
		if hasImage {
			out[i].Content = parts
		} else {
			out[i].Content = m.Text()
		}
		if len(m.ToolCalls) > 0 {
			out[i].ToolCalls = make([]wireToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				out[i].ToolCalls = append(out[i].ToolCalls, toWireToolCall(tc))
			}
		}
	}
	return out, nil
}

func prepareProviderImage(data ImageData, capabilities ProviderCapabilities) ([]byte, string, error) {
	if len(data.Bytes) == 0 {
		return nil, "", fmt.Errorf("image is empty")
	}
	supports := func(mimeType string) bool {
		for _, candidate := range capabilities.ImageInputMIMEs {
			if candidate == mimeType {
				return true
			}
		}
		return false
	}
	supported := supports(data.MIMEType)
	if supported && len(data.Bytes) <= capabilities.MaxImageBytes {
		return data.Bytes, data.MIMEType, nil
	}
	source, _, err := image.Decode(bytes.NewReader(data.Bytes))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	for scale := 1.0; scale >= 0.2; scale -= 0.15 {
		w, h := int(float64(width)*scale), int(float64(height)*scale)
		if w < 1 || h < 1 {
			break
		}
		target := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := color.RGBAModel.Convert(source.At(bounds.Min.X+x*width/w, bounds.Min.Y+y*height/h))
				target.Set(x, y, c)
			}
		}
		if supports("image/jpeg") {
			for quality := 85; quality >= 45; quality -= 10 {
				var encoded bytes.Buffer
				if err := jpeg.Encode(&encoded, target, &jpeg.Options{Quality: quality}); err != nil {
					return nil, "", err
				}
				if encoded.Len() <= capabilities.MaxImageBytes {
					return encoded.Bytes(), "image/jpeg", nil
				}
			}
		}
		if supports("image/png") {
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, target); err != nil {
				return nil, "", err
			}
			if encoded.Len() <= capabilities.MaxImageBytes {
				return encoded.Bytes(), "image/png", nil
			}
		}
	}
	return nil, "", fmt.Errorf("image exceeds provider byte limit")
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
	messages, err := d.toWireMessages(ctx, req.Messages, nil)
	if err != nil {
		return ChatResponse{}, err
	}
	body := wireRequest{Model: d.cfg.Model, Messages: messages}
	resp, err := d.doJSON(ctx, body, nil)
	if err != nil {
		return ChatResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("%w: empty choices", ErrUnavailable)
	}
	return ChatResponse{Content: wireText(resp.Choices[0].Message.Content)}, nil
}

func (d *DeepSeek) CallTool(ctx context.Context, req ToolCallRequest) (ToolCallResponse, error) {
	messages, err := d.toWireMessages(ctx, req.Messages, req.ImageResolver)
	if err != nil {
		return ToolCallResponse{}, err
	}
	body := wireRequest{
		Model:    d.cfg.Model,
		Messages: messages,
		Tools:    toWireTools(req.Tools),
	}
	resp, err := d.doJSON(ctx, body, req.OnRetry)
	if err != nil {
		return ToolCallResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return ToolCallResponse{}, fmt.Errorf("%w: empty choices", ErrUnavailable)
	}
	msg := resp.Choices[0].Message
	out := ToolCallResponse{Text: wireText(msg.Content), ReasoningContent: msg.ReasoningContent}
	for _, tc := range msg.ToolCalls {
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
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Args: args})
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
	messages, err := d.toWireMessages(ctx, req.Messages, nil)
	if err != nil {
		return nil, err
	}
	body := wireRequest{Model: d.cfg.Model, Messages: messages, Stream: true}
	httpResp, err := d.doRaw(ctx, body, nil)
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
			if len(chunk.Choices) > 0 && wireText(chunk.Choices[0].Delta.Content) != "" {
				if !trySend(ctx, out, StreamChunk{Text: wireText(chunk.Choices[0].Delta.Content)}) {
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

func wireText(value any) string {
	text, _ := value.(string)
	return text
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

func (d *DeepSeek) doJSON(ctx context.Context, body wireRequest, onRetry func(int)) (wireResponse, error) {
	httpResp, err := d.doRaw(ctx, body, onRetry)
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
func (d *DeepSeek) doRaw(ctx context.Context, body wireRequest, onRetry func(int)) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(d.cfg.BaseURL, "/") + "/v1/chat/completions"

	var lastErr error
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			if onRetry != nil {
				onRetry(attempt)
			}
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
