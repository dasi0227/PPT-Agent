package llm

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

type retryTestTransport func(*http.Request) (*http.Response, error)

func (transport retryTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestAdapterBoundsAllTransientFailuresToTwoRetries(t *testing.T) {
	for _, kind := range []string{"transport", "http_503", "malformed_200"} {
		t.Run(kind, func(t *testing.T) {
			h := newAdapterHTTP("secret", "https://provider.test", 0)
			hits, retries := 0, 0
			h.client.Transport = retryTestTransport(func(*http.Request) (*http.Response, error) {
				hits++
				if kind == "transport" {
					return nil, errors.New("connection lost")
				}
				status := http.StatusServiceUnavailable
				if kind == "malformed_200" {
					status = http.StatusOK
				}
				return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("gateway failure"))}, nil
			})
			var response responsesResponse
			err := h.doJSONObserved(context.Background(), "/responses", map[string]any{}, func(int) { retries++ }, nil, &response)
			if !errors.Is(err, ErrUnavailable) || hits != 3 || retries != 2 {
				t.Fatalf("unbounded %s retry: err=%v hits=%d retries=%d", kind, err, hits, retries)
			}
		})
	}
}

func TestAdapterCancellationDuringBackoffDoesNotSendAnotherAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := newAdapterHTTP("secret", "https://provider.test", 0)
	hits := 0
	h.client.Transport = retryTestTransport(func(*http.Request) (*http.Response, error) {
		hits++
		return nil, errors.New("connection lost")
	})
	var response responsesResponse
	err := h.doJSONObserved(ctx, "/responses", map[string]any{}, func(int) { cancel() }, nil, &response)
	if !errors.Is(err, context.Canceled) || hits != 1 {
		t.Fatalf("cancellation sent another attempt: err=%v hits=%d", err, hits)
	}
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

func TestProtocolsDoNotForwardCredentialsOnRedirect(t *testing.T) {
	for _, protocol := range []string{ProtocolResponses, ProtocolAnthropic} {
		t.Run(protocol, func(t *testing.T) {
			var hits atomic.Int32
			h := newAdapterHTTP("secret", "https://redirect-source.test", time.Second)
			h.client.Transport = submissionTransport(func(request *http.Request) (*http.Response, error) {
				if request.URL.Host == "redirect-sink.test" {
					hits.Add(1)
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
				}
				return &http.Response{StatusCode: http.StatusTemporaryRedirect, Header: http.Header{"Location": {"https://redirect-sink.test/messages"}}, Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
			})
			h.protocol = protocol
			var response any
			err := h.doJSONObserved(context.Background(), "/messages", map[string]any{}, nil, nil, &response)
			if !errors.Is(err, ErrBadRequest) || hits.Load() != 0 {
				t.Fatalf("redirect forwarded credentials: %v hits=%d", err, hits.Load())
			}
		})
	}
}
