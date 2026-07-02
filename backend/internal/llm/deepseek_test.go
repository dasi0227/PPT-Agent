package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

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
	resp, err := d.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "hi"}}})
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
	if resp.ToolCall == nil || resp.ToolCall.Name != "finish" {
		t.Fatalf("expected finish tool call, got %+v", resp.ToolCall)
	}
	if resp.ToolCall.Args["summary"] != "done" {
		t.Errorf("bad args: %+v", resp.ToolCall.Args)
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
