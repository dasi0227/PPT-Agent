package decision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/modelusage"
)

type Error struct {
	Kind   string
	Status int
}

func (e *Error) Error() string { return fmt.Sprintf("decision %s (HTTP %d)", e.Kind, e.Status) }
func Failure(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return "unavailable"
}

type Client struct {
	endpoint, model, key string
	http                 *http.Client
}

func (c *Client) String() string   { return "DecisionClient{redacted}" }
func (c *Client) GoString() string { return c.String() }

// NewSnapshot 校验必填的 JEV 配置，并固定当前凭证。
func NewSnapshot(cfg config.JevConfig) (Snapshot, error) {
	if err := config.ValidateJevConfig(&cfg); err != nil {
		return Snapshot{}, err
	}
	hash := sha256.Sum256([]byte("jev-images-v1\x00" + cfg.BaseURL + "\x00" + cfg.Model + "\x00" + cfg.Key))
	return Snapshot{Identity: hex.EncodeToString(hash[:]), Provider: &Client{endpoint: cfg.BaseURL + "/systemone", model: cfg.Model, key: cfg.Key, http: &http.Client{Timeout: 20 * time.Second, Transport: modelusage.NewTransport(nil, "jev", cfg.Model), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}}, nil
}

// Conservative UTF-8 byte bounds also protect CJK requests without pretending to
// have the provider's tokenizer. Callers split batches instead of truncating.
func Fits(req Request) bool {
	state, err := json.Marshal(req.State)
	if err != nil {
		return false
	}
	total := len(state)
	for _, q := range req.Questions {
		b, e := json.Marshal(q)
		if e != nil || len(state)+len(b) > 28000 {
			return false
		}
		total += len(b)
	}
	return total <= 56000
}
func (c *Client) Evaluate(ctx context.Context, req Request) (Response, error) {
	if err := Validate(req); err != nil {
		return Response{}, &Error{Kind: "request"}
	}
	payload, err := buildJevPayload(ctx, req, c.model)
	if err != nil {
		return Response{}, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload.Data))
		if err != nil {
			return Response{}, &Error{Kind: "request"}
		}
		request.Header.Set("Authorization", "Bearer "+c.key)
		request.Header.Set("Content-Type", payload.ContentType)
		if payload.HasImages {
			request.Header.Set("TypeSafe-Danger-Zone-Images", "true")
		}
		response, err := c.http.Do(request)
		delay := 250 * time.Millisecond
		if err != nil {
			if ctx.Err() != nil {
				return Response{}, ctx.Err()
			}
			if attempt == 1 {
				return Response{}, &Error{Kind: "transport"}
			}
		} else {
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
			response.Body.Close()
			if ctx.Err() != nil {
				return Response{}, ctx.Err()
			}
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				if readErr != nil {
					return Response{}, &Error{Kind: "transport"}
				}
				var wire struct {
					Model   string                     `json:"model"`
					Answers map[string]json.RawMessage `json:"answers"`
					Usage   *Usage                     `json:"usage"`
				}
				if json.Unmarshal(body, &wire) != nil || wire.Model == "" || len(wire.Answers) == 0 || wire.Usage == nil || wire.Usage.InputTokens < 0 || wire.Usage.OutputTokens < 0 {
					return Response{}, &Error{Kind: "protocol"}
				}
				for id := range wire.Answers {
					if _, ok := req.Questions[id]; !ok {
						return Response{}, &Error{Kind: "protocol"}
					}
				}
				out := Response{Model: wire.Model, Usage: *wire.Usage, Answers: map[string]Answer{}, Invalid: map[string]string{}}
				for id, q := range req.Questions {
					a, e := decodeAnswer(wire.Answers[id], q)
					if e != nil {
						out.Invalid[id] = e.Error()
					} else {
						out.Answers[id] = a
					}
				}
				return out, nil
			}
			retry := response.StatusCode == 429 || response.StatusCode >= 500
			if !retry || attempt == 1 {
				kind := "http"
				if response.StatusCode == 401 || response.StatusCode == 403 {
					kind = "authentication"
				}
				return Response{}, &Error{Kind: kind, Status: response.StatusCode}
			}
			if seconds, e := strconv.Atoi(response.Header.Get("Retry-After")); e == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			} else if deadline, e := http.ParseTime(response.Header.Get("Retry-After")); e == nil && time.Until(deadline) > 0 {
				delay = time.Until(deadline)
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Response{}, ctx.Err()
		case <-timer.C:
		}
	}
	return Response{}, &Error{Kind: "unavailable"}
}
