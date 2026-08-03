package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

type staticImageResolver struct {
	data ImageData
	err  error
	refs []string
}

func (r *staticImageResolver) ResolveImage(_ context.Context, ref string) (ImageData, error) {
	r.refs = append(r.refs, ref)
	return r.data, r.err
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, img); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes()
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func newServer(t *testing.T, h http.HandlerFunc) *DeepSeek {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewDeepSeek(DeepSeekConfig{APIKey: "sk-secret-key", BaseURL: srv.URL, Model: "deepseek-chat"})
}

func TestChatOK(t *testing.T) {
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`))
	})
	resp, err := d.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: RoleUser, Content: TextContent("hi")}}})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("want hello, got %q", resp.Content)
	}
}

func TestCallToolParsesToolCall(t *testing.T) {
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"thinking",
		"tool_calls":[{"id":"c1","type":"function","function":{"name":"finish","arguments":"{\"summary\":\"done\"}"}}]}}]}`))
	})
	resp, err := d.CallTool(context.Background(), ToolCallRequest{})
	if err != nil {
		t.Fatalf("calltool: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "finish" {
		t.Fatalf("expected finish tool call, got %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].Args["summary"] != "done" {
		t.Errorf("bad args: %+v", resp.ToolCalls[0].Args)
	}
}

func TestCallToolPreservesEveryProviderToolCallInOrder(t *testing.T) {
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[
			{"id":"c1","type":"function","function":{"name":"read_ppt","arguments":"{\"resource\":{\"type\":\"deck\",\"part\":\"outline\"}}"}},
			{"id":"c2","type":"function","function":{"name":"search_refs","arguments":"{\"query\":\"market\"}"}}
		]}}]}`))
	})
	resp, err := d.CallTool(context.Background(), ToolCallRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 2 || resp.ToolCalls[0].ID != "c1" || resp.ToolCalls[1].ID != "c2" {
		t.Fatalf("provider tool calls were lost or reordered: %+v", resp.ToolCalls)
	}
}

func TestCallToolSendsAssistantToolCallsAndObservation(t *testing.T) {
	var body map[string]any
	d := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	})

	_, err := d.CallTool(context.Background(), ToolCallRequest{Messages: []Message{
		{Role: RoleUser, Content: TextContent("do it")},
		{Role: RoleAssistant, Content: TextContent("thinking"), ToolCalls: []ToolCall{{
			ID: "deepseek-call-1", Name: "demo", Args: map[string]any{"x": "1"},
		}}},
		{Role: RoleTool, ToolCallID: "deepseek-call-1", Content: TextContent("ok")},
	}})
	if err != nil {
		t.Fatalf("calltool: %v", err)
	}
	messages, ok := body["messages"].([]any)
	if !ok || len(messages) != 3 {
		t.Fatalf("expected 3 wire messages, got %#v", body["messages"])
	}
	assistant, _ := messages[1].(map[string]any)
	toolCalls, ok := assistant["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("assistant message must include tool_calls, got %#v", assistant)
	}
	toolCall, _ := toolCalls[0].(map[string]any)
	if toolCall["id"] != "deepseek-call-1" || toolCall["type"] != "function" {
		t.Fatalf("bad wire tool call envelope: %#v", toolCall)
	}
	fn, _ := toolCall["function"].(map[string]any)
	if fn["name"] != "demo" || fn["arguments"] != `{"x":"1"}` {
		t.Fatalf("bad wire function call: %#v", fn)
	}
	toolMsg, _ := messages[2].(map[string]any)
	if toolMsg["role"] != "tool" || toolMsg["tool_call_id"] != "deepseek-call-1" || toolMsg["content"] != "ok" {
		t.Fatalf("tool observation must reference matching call id, got %#v", toolMsg)
	}
}

func TestCallToolWireRequestContainsResolvedScreenshot(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"seen"}}]}`))
	}))
	defer srv.Close()
	d := NewDeepSeek(DeepSeekConfig{
		APIKey: "secret", BaseURL: srv.URL, Model: "vision-model", Vision: true,
		ImageInputMIMEs: []string{"image/png", "image/jpeg"}, MaxImageBytes: 1024 * 1024,
	})
	resolver := &staticImageResolver{data: ImageData{Bytes: testPNG(t, 32, 18), MIMEType: "image/png"}}
	_, err := d.CallTool(context.Background(), ToolCallRequest{
		ImageResolver: resolver,
		Messages: []Message{{
			Role: RoleTool, ToolCallID: "call-render-1",
			Content: []ContentPart{
				{Type: "text", Text: `{"tool_call_id":"call-render-1","slide_id":"slide-2","source_hash":"abc"}`},
				{Type: "image", ImageRef: "run:run-1/screenshot:shot-1", MIMEType: "image/png", Detail: "high"},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolver.refs) != 1 || resolver.refs[0] != "run:run-1/screenshot:shot-1" {
		t.Fatalf("unexpected image resolution: %+v", resolver.refs)
	}
	messages := body["messages"].([]any)
	wire := messages[0].(map[string]any)
	if wire["tool_call_id"] != "call-render-1" {
		t.Fatalf("tool call pairing lost: %#v", wire)
	}
	parts := wire["content"].([]any)
	if len(parts) != 2 {
		t.Fatalf("multimodal content missing: %#v", parts)
	}
	imagePart := parts[1].(map[string]any)
	imageURL := imagePart["image_url"].(map[string]any)
	url := imageURL["url"].(string)
	if imagePart["type"] != "image_url" || !strings.HasPrefix(url, "data:image/png;base64,") ||
		strings.Contains(url, "/Users/") || strings.Contains(url, "run:run-1") {
		t.Fatalf("provider image wire payload is unsafe: %#v", imagePart)
	}
}

func TestCallToolRejectsImageWhenProviderHasNoVision(t *testing.T) {
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("provider must not be called without vision capability")
	})
	_, err := d.CallTool(context.Background(), ToolCallRequest{
		ImageResolver: &staticImageResolver{data: ImageData{Bytes: testPNG(t, 2, 2), MIMEType: "image/png"}},
		Messages: []Message{{Role: RoleTool, Content: []ContentPart{{
			Type: "image", ImageRef: "run:r/screenshot:shot-1",
		}}}},
	})
	if err == nil || !strings.Contains(err.Error(), "VISION_CAPABILITY_REQUIRED") {
		t.Fatalf("expected explicit vision capability error, got %v", err)
	}
}

func TestPrepareProviderImageCompressesToProviderLimit(t *testing.T) {
	source := testPNG(t, 512, 320)
	raw, mimeType, err := prepareProviderImage(ImageData{Bytes: source, MIMEType: "image/png"}, ProviderCapabilities{
		Vision: true, ImageInputMIMEs: []string{"image/jpeg"}, MaxImageBytes: 24 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/jpeg" || len(raw) > 24*1024 || len(raw) == 0 {
		t.Fatalf("compression did not honor provider constraints: mime=%s bytes=%d", mimeType, len(raw))
	}
	if _, format, err := image.Decode(bytes.NewReader(raw)); err != nil || format != "jpeg" {
		t.Fatalf("compressed provider image is invalid: format=%s err=%v", format, err)
	}
}

// ARCH-LLM-FC-001：function call 参数非法 JSON → ErrBadToolCall（最小失败退出）。
func TestCallToolBadArgs(t *testing.T) {
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant",
		"tool_calls":[{"id":"c1","type":"function","function":{"name":"finish","arguments":"{not json"}}]}}]}`))
	})
	_, err := d.CallTool(context.Background(), ToolCallRequest{})
	if !errors.Is(err, ErrBadToolCall) {
		t.Fatalf("want ErrBadToolCall, got %v", err)
	}
}

func TestCallToolRepairsTrailingCloseBraceArgs(t *testing.T) {
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant",
		"tool_calls":[{"id":"c1","type":"function","function":{"name":"finish","arguments":"{\"summary\":\"done\"}}"}}]}}]}`))
	})
	resp, err := d.CallTool(context.Background(), ToolCallRequest{})
	if err != nil {
		t.Fatalf("calltool: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Args["summary"] != "done" {
		t.Fatalf("bad repaired args: %+v", resp.ToolCalls)
	}
}

// ARCH-LLM 重试表：4xx 不重试，映射 ErrBadRequest。
func TestRetryPolicy4xxNoRetry(t *testing.T) {
	var hits int32
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadRequest)
	})
	_, err := d.Chat(context.Background(), ChatRequest{})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("want ErrBadRequest, got %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("4xx must not retry, hits=%d", got)
	}
}

// ARCH-LLM 重试表：5xx 退避重试至上限。
func TestRetryPolicy5xxRetries(t *testing.T) {
	var hits int32
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := d.Chat(context.Background(), ChatRequest{})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	// 首次 + 3 次重试 = 4 次。
	if got := atomic.LoadInt32(&hits); got != 4 {
		t.Errorf("want 4 attempts, got %d", got)
	}
}

func TestToolCallReportsOnlyTransientAutomaticRetries(t *testing.T) {
	var hits int32
	var retries int32
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	_, err := d.CallTool(context.Background(), ToolCallRequest{
		OnRetry: func(int) { atomic.AddInt32(&retries, 1) },
	})
	if !errors.Is(err, ErrUnavailable) || hits != 4 || retries != 3 {
		t.Fatalf("transient retry reporting mismatch: err=%v hits=%d retries=%d", err, hits, retries)
	}

	hits, retries = 0, 0
	d = newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadRequest)
	})
	_, err = d.CallTool(context.Background(), ToolCallRequest{
		OnRetry: func(int) { atomic.AddInt32(&retries, 1) },
	})
	if !errors.Is(err, ErrBadRequest) || hits != 1 || retries != 0 {
		t.Fatalf("non-transient request was retried: err=%v hits=%d retries=%d", err, hits, retries)
	}
}

// AC-LLM-002：取消 ctx → 流式 goroutine 及时退出，无泄漏（TestMain goleak 兜底）。
func TestStreamCancel(t *testing.T) {
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		fl, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 1000; i++ {
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n"))
			if fl != nil {
				fl.Flush()
			}
			time.Sleep(5 * time.Millisecond)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := d.Stream(ctx, ChatRequest{})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	// 读一个分片后取消。
	<-ch
	cancel()
	// 排空 channel 直到关闭；不应泄漏。
	for range ch {
	}
}

// AC-LLM-003：错误信息不泄漏明文 API Key。
func TestErrorDoesNotLeakKey(t *testing.T) {
	d := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := d.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "sk-secret-key") {
		t.Fatalf("error leaked API key: %v", err)
	}
}
