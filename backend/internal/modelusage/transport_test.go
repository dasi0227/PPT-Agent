package modelusage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type usageTestTransport func(*http.Request) (*http.Response, error)

func (f usageTestTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type usageTestBody struct {
	io.Reader
	closeErr error
}

func (b usageTestBody) Close() error { return b.closeErr }

type usageErrorReader struct{ err error }

func (r usageErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestRecorderFailureCannotChangeProviderResponseOrErrors(t *testing.T) {
	providerErr := errors.New("provider failed")
	recorderErr := errors.New("usage store failed")
	for _, mode := range []string{"success", "request_error", "read_error", "close_error"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			ctx := WithRecorder(context.Background(), func(context.Context, model.ModelCallUsage) error {
				calls++
				return recorderErr
			})
			const body = `{"usage":{"input_tokens":10,"output_tokens":2}}`
			transport := NewTransport(usageTestTransport(func(*http.Request) (*http.Response, error) {
				if mode == "request_error" {
					return nil, providerErr
				}
				reader := io.Reader(strings.NewReader(body))
				var closeErr error
				if mode == "read_error" {
					reader = io.MultiReader(reader, usageErrorReader{providerErr})
				}
				if mode == "close_error" {
					closeErr = providerErr
				}
				return &http.Response{StatusCode: http.StatusOK, Body: usageTestBody{reader, closeErr}}, nil
			}), "provider", "model")
			request, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://model.test", nil)
			response, err := transport.RoundTrip(request)
			if mode == "request_error" {
				if err != providerErr || calls != 1 {
					t.Fatalf("request error changed: %v calls=%d", err, calls)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			raw, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			_ = response.Body.Close()
			if string(raw) != body || calls != 1 || (mode == "read_error" && readErr != providerErr) || (mode != "read_error" && readErr != nil) ||
				(mode == "close_error" && closeErr != providerErr) || (mode != "close_error" && closeErr != nil) {
				t.Fatalf("accounting changed response: body=%s read=%v close=%v calls=%d", raw, readErr, closeErr, calls)
			}
		})
	}
}

func TestTransportNormalizesProviderUsageWithoutChangingResponses(t *testing.T) {
	for _, test := range []struct {
		name, body string
		want       model.TokenUsage
	}{
		{"responses", `{"model":"actual","usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":80},"output_tokens":20,"total_tokens":120}}`, model.TokenUsage{InputTokens: 100, OutputTokens: 20, TotalTokens: 120}},
		{"messages", `{"model":"actual","usage":{"input_tokens":10,"cache_creation_input_tokens":30,"cache_read_input_tokens":50,"output_tokens":2}}`, model.TokenUsage{InputTokens: 90, OutputTokens: 2, TotalTokens: 92}},
		{"jev", `{"model":"actual","answers":{},"usage":{"input_tokens":3,"output_tokens":2}}`, model.TokenUsage{InputTokens: 3, OutputTokens: 2, TotalTokens: 5}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			var calls []model.ModelCallUsage
			ctx := WithRecorder(context.Background(), func(_ context.Context, call model.ModelCallUsage) error {
				calls = append(calls, call)
				return nil
			})
			client := &http.Client{Transport: NewTransport(nil, test.name, "requested")}
			request, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, strings.NewReader(`{}`))
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			_ = response.Body.Close()
			if string(raw) != test.body || len(calls) != 1 || calls[0].CallID == "" || calls[0].Model != "actual" || calls[0].Provider != test.name || calls[0].Usage == nil || *calls[0].Usage != test.want {
				t.Fatalf("body=%s calls=%+v", raw, calls)
			}
		})
	}
}

func TestTransportKeepsAttemptsAndRunContextsSeparate(t *testing.T) {
	var attempt atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempt.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":"retry"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"usage":{"input_tokens":100,"output_tokens":20,"total_tokens":120}}`)
	}))
	defer server.Close()
	client := &http.Client{Transport: NewTransport(nil, "provider", "model")}
	var first, second []model.ModelCallUsage
	for _, destination := range []*[]model.ModelCallUsage{&first, &first, &second, nil} {
		ctx := context.Background()
		if destination != nil {
			ctx = WithRecorder(ctx, func(_ context.Context, call model.ModelCallUsage) error {
				*destination = append(*destination, call)
				return nil
			})
		}
		request, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, strings.NewReader(`{}`))
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
	if len(first) != 2 || first[0].Usage != nil || first[1].Usage == nil || first[0].CallID == first[1].CallID || len(second) != 1 {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
}

func TestMissingOrInvalidUsageIsUnknownRatherThanZero(t *testing.T) {
	for _, body := range []string{`{}`, `{"usage":null}`, `{"usage":{"output_tokens":2}}`, `{"usage":{"input_tokens":null,"output_tokens":2}}`, `{"usage":{"input_tokens":-1,"output_tokens":2}}`, `{"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":1}}`} {
		if usage, _ := readUsage([]byte(body), "model"); usage != nil {
			t.Fatalf("accepted invalid usage: %s %+v", body, usage)
		}
	}
	usage, _ := readUsage([]byte(`{"usage":{"input_tokens":0,"output_tokens":0}}`), "model")
	if usage == nil || *usage != (model.TokenUsage{}) {
		t.Fatalf("explicit zero is valid: %+v", usage)
	}
	usage, modelName := readUsage([]byte(`{"model":{},"usage":{"input_tokens":3,"output_tokens":2}}`), "requested")
	if usage == nil || usage.TotalTokens != 5 || modelName != "requested" {
		t.Fatalf("invalid response metadata discarded valid consumption: %+v model=%s", usage, modelName)
	}
}
