package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type failingResponseBody struct {
	cause      error
	beforeRead func()
}

func (b failingResponseBody) Read(p []byte) (int, error) {
	if b.beforeRead != nil {
		b.beforeRead()
	}
	return copy(p, `{"output":[]}`), b.cause
}

func (failingResponseBody) Close() error { return nil }

func TestResponseReadFailuresKeepCauseAndAttemptDiagnostics(t *testing.T) {
	for _, cause := range []error{context.DeadlineExceeded, io.ErrUnexpectedEOF, errors.New("read failed https://private-provider.test?api_key=secret")} {
		t.Run(providerFailureKind(cause), func(t *testing.T) {
			h := newAdapterHTTP("secret", "https://provider.test", 0)
			hits := 0
			h.client.Transport = retryTestTransport(func(*http.Request) (*http.Response, error) {
				hits++
				return &http.Response{StatusCode: 200, Header: http.Header{"X-Request-Id": {"req-read"}}, Body: failingResponseBody{cause: cause}}, nil
			})
			var diagnostics []RequestDiagnostic
			var response responsesResponse
			err := h.doJSONObserved(context.Background(), "/responses", map[string]any{}, nil, func(d RequestDiagnostic) { diagnostics = append(diagnostics, d) }, &response)
			var upstream *ProviderError
			if hits != 2 || !errors.Is(err, ErrUnavailable) || !errors.Is(err, cause) || !errors.As(err, &upstream) {
				t.Fatalf("read failure lost cause/retry policy: hits=%d err=%v", hits, err)
			}
			if upstream.Phase != "body_read_failed" || upstream.StatusCode != 200 || upstream.RequestID != "req-read" || upstream.BodyBytes == 0 {
				t.Fatalf("missing response read diagnostic: %#v", upstream)
			}
			for _, d := range diagnostics {
				if d.Phase == "body_received" || d.Phase == "body_decode_failed" {
					t.Fatalf("read failure reported as complete/decode: %+v", diagnostics)
				}
			}
			last := diagnostics[len(diagnostics)-1]
			if last.Phase != "body_read_failed" || last.FailureKind != providerFailureKind(cause) || last.Attempt != 1 {
				t.Fatalf("wrong final attempt diagnostic: %+v", last)
			}
			if strings.Contains(err.Error(), "private-provider") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("private cause leaked into log text: %v", err)
			}
		})
	}
}

func TestCancellationWhileReadingFinalAttemptIsNotDecodeFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := newAdapterHTTP("secret", "https://provider.test", 0)
	hits := 0
	h.client.Transport = retryTestTransport(func(*http.Request) (*http.Response, error) {
		hits++
		body := failingResponseBody{cause: io.ErrUnexpectedEOF}
		if hits == 2 {
			body.cause, body.beforeRead = context.Canceled, cancel
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: body}, nil
	})
	var response responsesResponse
	err := h.doJSONObserved(ctx, "/responses", map[string]any{}, nil, nil, &response)
	if hits != 2 || !errors.Is(err, context.Canceled) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("final read cancellation misclassified: hits=%d err=%v", hits, err)
	}
	if diagnostic := ProviderFailureDiagnostic(err); diagnostic["phase"] != "body_read_failed" || diagnostic["class"] != "canceled" {
		t.Fatalf("read cancellation missing from trace: %+v", diagnostic)
	}
}

func TestNonTransientHTTPStatusStaysNonRetryableWhenBodyReadFails(t *testing.T) {
	h := newAdapterHTTP("secret", "https://provider.test", 0)
	hits := 0
	h.client.Transport = retryTestTransport(func(*http.Request) (*http.Response, error) {
		hits++
		return &http.Response{StatusCode: 401, Header: http.Header{}, Body: failingResponseBody{cause: io.ErrUnexpectedEOF}}, nil
	})
	var response responsesResponse
	err := h.doJSONObserved(context.Background(), "/responses", map[string]any{}, nil, nil, &response)
	if hits != 1 || !errors.Is(err, ErrBadRequest) || !errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("HTTP authentication failure changed retry policy: hits=%d err=%v", hits, err)
	}
}
