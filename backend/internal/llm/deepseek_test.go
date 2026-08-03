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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

type staticImageResolver struct {
	mu   sync.Mutex
	data ImageData
	err  error
	refs []string
}

func (r *staticImageResolver) ResolveImage(ctx context.Context, ref string) (ImageData, error) {
	if err := ctx.Err(); err != nil {
		return ImageData{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
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

func TestDeepSeekGeneratePreservesMultipleToolCallsAndReasoningContinuation(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, body)
		if len(requests) == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"checking","reasoning_content":"private-state","tool_calls":[
				{"id":"call-1","type":"function","function":{"name":"read_ppt","arguments":"{\"resource\":{\"type\":\"deck\",\"part\":\"outline\"}}"}},
				{"id":"call-2","type":"function","function":{"name":"search_refs","arguments":"{\"query\":\"market\"}"}}
			]}}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"done","tool_calls":[]}}]}`))
	}))
	defer server.Close()
	adapter := NewDeepSeekAdapter(DeepSeekConfig{
		APIKey: "sk-test-secret", BaseURL: server.URL, Model: "deepseek-v4-pro",
	})
	first, err := adapter.Generate(context.Background(), GenerateRequest{
		Messages: []Message{{Role: RoleUser, Content: TextContent("inspect")}},
		Tools:    []ToolSchema{{Name: "read_ppt"}, {Name: "search_refs"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 2 || first.ToolCalls[0].ID != "call-1" ||
		first.ToolCalls[1].ID != "call-2" || first.Usage.TotalTokens != 14 ||
		first.Continuation == nil {
		t.Fatalf("normalized response is incomplete: %+v", first)
	}
	_, err = adapter.Generate(context.Background(), GenerateRequest{
		Messages: []Message{
			{Role: RoleUser, Content: TextContent("inspect")},
			{Role: RoleAssistant, Content: TextContent("checking"), ToolCalls: first.ToolCalls},
			{Role: RoleTool, ToolCallID: "call-1", Content: TextContent("outline")},
			{Role: RoleTool, ToolCallID: "call-2", Content: TextContent("refs")},
		},
		Tools:        []ToolSchema{{Name: "read_ppt"}, {Name: "search_refs"}},
		Continuation: first.Continuation,
	})
	if err != nil {
		t.Fatal(err)
	}
	messages := requests[1]["messages"].([]any)
	assistant := messages[1].(map[string]any)
	if assistant["reasoning_content"] != "private-state" {
		t.Fatalf("DeepSeek reasoning continuation was not replayed: %#v", assistant)
	}
}

func TestDeepSeekFailsClosedForImageInput(t *testing.T) {
	adapter := NewDeepSeekAdapter(DeepSeekConfig{
		APIKey: "secret", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro",
	})
	if adapter.Capabilities().Vision {
		t.Fatal("DeepSeek V4 must remain text-only")
	}
	_, err := adapter.Generate(context.Background(), GenerateRequest{
		Messages: []Message{{Role: RoleUser, Content: []ContentPart{{
			Type: "image", ImageRef: "run:r/screenshot:shot-1",
		}}}},
		ImageResolver: &staticImageResolver{data: ImageData{
			Bytes: testPNG(t, 2, 2), MIMEType: "image/png",
		}},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected a closed image capability failure, got %v", err)
	}
}

func TestDeepSeekProviderErrorMappingAndCancellation(t *testing.T) {
	t.Run("transient", func(t *testing.T) {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		adapter := NewDeepSeekAdapter(DeepSeekConfig{APIKey: "secret", BaseURL: server.URL, Model: "deepseek-chat"})
		adapter.http.maxRetries = 1
		retries := 0
		_, err := adapter.Generate(context.Background(), GenerateRequest{OnRetry: func(int) { retries++ }})
		if !errors.Is(err, ErrUnavailable) || hits.Load() != 2 || retries != 1 {
			t.Fatalf("transient mapping mismatch: err=%v hits=%d retries=%d", err, hits.Load(), retries)
		}
	})

	t.Run("non-transient", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()
		adapter := NewDeepSeekAdapter(DeepSeekConfig{APIKey: "secret", BaseURL: server.URL, Model: "deepseek-chat"})
		_, err := adapter.Generate(context.Background(), GenerateRequest{})
		if !errors.Is(err, ErrBadRequest) {
			t.Fatalf("expected non-transient error, got %v", err)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		started := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			close(started)
			<-request.Context().Done()
		}))
		defer server.Close()
		adapter := NewDeepSeekAdapter(DeepSeekConfig{APIKey: "secret", BaseURL: server.URL, Model: "deepseek-chat"})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := adapter.Generate(ctx, GenerateRequest{})
			done <- err
		}()
		<-started
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) || errors.Is(err, ErrUnavailable) {
				t.Fatalf("cancel was wrapped incorrectly: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("provider request did not observe context cancellation")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(80 * time.Millisecond)
			_, _ = w.Write([]byte(`{"choices":[]}`))
		}))
		defer server.Close()
		adapter := NewDeepSeekAdapter(DeepSeekConfig{
			APIKey: "secret", BaseURL: server.URL, Model: "deepseek-chat",
			Timeout: 10 * time.Millisecond,
		})
		adapter.http.maxRetries = 0
		_, err := adapter.Generate(context.Background(), GenerateRequest{})
		if !errors.Is(err, ErrUnavailable) || errors.Is(err, context.Canceled) {
			t.Fatalf("timeout mapping mismatch: %v", err)
		}
	})
}

func TestAdapterErrorsNeverLeakKey(t *testing.T) {
	const secret = "sk-never-print-this"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"provider body must stay private"}}`))
	}))
	defer server.Close()
	adapter := NewDeepSeekAdapter(DeepSeekConfig{APIKey: secret, BaseURL: server.URL, Model: "deepseek-chat"})
	_, err := adapter.Generate(context.Background(), GenerateRequest{})
	if err == nil || strings.Contains(err.Error(), secret) ||
		strings.Contains(err.Error(), "provider body") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("unsafe provider error projection: %v", err)
	}
}

func TestPrepareProviderImageCompressesToProviderLimit(t *testing.T) {
	raw, mimeType, err := prepareProviderImage(
		ImageData{Bytes: testPNG(t, 512, 320), MIMEType: "image/png"},
		Capabilities{
			Vision: true, ImageInputMIMEs: []string{"image/jpeg"},
			MaxImageBytes: 24 * 1024,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/jpeg" || len(raw) == 0 || len(raw) > 24*1024 {
		t.Fatalf("compression did not honor the provider limit: mime=%s bytes=%d", mimeType, len(raw))
	}
}
