package llm

import (
	"bytes"
	"context"
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

func TestAdapterProviderErrorMappingAndCancellation(t *testing.T) {
	t.Run("transient", func(t *testing.T) {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		adapter := NewResponsesAdapter(AdapterConfig{Provider: "custom", APIKey: "secret", BaseURL: server.URL, Model: "test-model"})
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
		adapter := NewResponsesAdapter(AdapterConfig{Provider: "custom", APIKey: "secret", BaseURL: server.URL, Model: "test-model"})
		_, err := adapter.Generate(context.Background(), GenerateRequest{})
		if !errors.Is(err, ErrBadRequest) {
			t.Fatalf("expected non-transient error, got %v", err)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			close(started)
			select {
			case <-request.Context().Done():
			case <-release:
			}
		}))
		defer server.Close()
		defer close(release)
		adapter := NewResponsesAdapter(AdapterConfig{Provider: "custom", APIKey: "secret", BaseURL: server.URL, Model: "test-model"})
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
			_, _ = w.Write([]byte(`{"output":[]}`))
		}))
		defer server.Close()
		adapter := NewResponsesAdapter(AdapterConfig{Provider: "custom",
			APIKey: "secret", BaseURL: server.URL, Model: "test-model",
			Timeout: 10 * time.Millisecond,
		})
		adapter.http.maxRetries = 0
		_, err := adapter.Generate(context.Background(), GenerateRequest{})
		if !errors.Is(err, ErrUnavailable) || errors.Is(err, context.Canceled) {
			t.Fatalf("timeout mapping mismatch: %v", err)
		}
	})
}

func TestAdapterProviderErrorCarriesSanitizedDiagnostic(t *testing.T) {
	const secret = "sk-never-print-this"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("x-request-id", "req-123")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad schema api_key=sk-body-secret Authorization=Bearer hidden","type":"invalid_request_error","code":"invalid_request"}}`))
	}))
	defer server.Close()
	adapter := NewResponsesAdapter(AdapterConfig{Provider: "custom", APIKey: secret, BaseURL: server.URL, Model: "test-model"})
	_, err := adapter.Generate(context.Background(), GenerateRequest{})
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.StatusCode != http.StatusUnauthorized ||
		providerErr.Code != "invalid_request" || providerErr.Type != "invalid_request_error" ||
		!strings.Contains(providerErr.Message, "bad schema") || providerErr.RequestID != "req-123" {
		t.Fatalf("provider diagnostic missing: %#v err=%v", providerErr, err)
	}
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "sk-body-secret") ||
		strings.Contains(err.Error(), "hidden") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("unsafe provider error projection: %v", err)
	}
}

func TestAdapterRetriesMalformedSuccessfulResponseOnceWithDiagnostics(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempt := hits.Add(1)
		w.Header().Set("x-request-id", "req-decode")
		if attempt == 1 {
			_, _ = w.Write([]byte("upstream temporarily returned html"))
			return
		}
		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"recovered"}]}]}`))
	}))
	defer server.Close()
	adapter := NewResponsesAdapter(AdapterConfig{Provider: "custom", APIKey: "secret", BaseURL: server.URL, Model: "test-model"})
	response, err := adapter.Generate(context.Background(), GenerateRequest{})
	if err != nil || response.Text() != "recovered" || hits.Load() != 2 {
		t.Fatalf("malformed 2xx response was not recovered: response=%+v err=%v hits=%d", response, err, hits.Load())
	}
}

func TestAdapterMalformedSuccessfulResponseFailsWithSafeDiagnostics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("x-request-id", "req-decode")
		_, _ = w.Write([]byte("<html>gateway failure</html>"))
	}))
	defer server.Close()
	adapter := NewResponsesAdapter(AdapterConfig{Provider: "custom", APIKey: "secret", BaseURL: server.URL, Model: "test-model"})
	adapter.http.maxRetries = 0
	_, err := adapter.Generate(context.Background(), GenerateRequest{})
	var providerErr *ProviderError
	if !errors.Is(err, ErrUnavailable) || !errors.As(err, &providerErr) ||
		providerErr.StatusCode != http.StatusOK || providerErr.RequestID != "req-decode" ||
		providerErr.BodyBytes == 0 || providerErr.BodySHA256 == "" {
		t.Fatalf("decode diagnostic missing: provider=%#v err=%v", providerErr, err)
	}
	if strings.Contains(err.Error(), "gateway failure") {
		t.Fatalf("raw malformed provider body leaked into error: %v", err)
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

func TestProtocolEndpointPreservesGatewayPrefix(t *testing.T) {
	for _, tc := range []struct{ base, path, want string }{
		{"https://gateway.example/v1/", "/responses", "https://gateway.example/v1/responses"},
		{"https://gateway.example/team/anthropic/v1", "/messages", "https://gateway.example/team/anthropic/v1/messages"},
		{"https://gateway.example/team%2Fone/v1", "/responses", "https://gateway.example/team%2Fone/v1/responses"},
	} {
		got, err := providerEndpoint(tc.base, tc.path)
		if err != nil || got != tc.want {
			t.Fatalf("endpoint=%s error=%v", got, err)
		}
	}
}

func TestProtocolsDoNotForwardCredentialsOnRedirect(t *testing.T) {
	for _, protocol := range []string{ProtocolResponses, ProtocolAnthropic} {
		t.Run(protocol, func(t *testing.T) {
			var hits atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
			defer target.Close()
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			}))
			defer gateway.Close()
			h := newAdapterHTTP("secret", gateway.URL, time.Second)
			h.protocol = protocol
			var response any
			err := h.doJSON(context.Background(), "/messages", map[string]any{}, nil, &response)
			if !errors.Is(err, ErrBadRequest) || hits.Load() != 0 {
				t.Fatalf("redirect forwarded credentials: %v hits=%d", err, hits.Load())
			}
		})
	}
}
