// Package modelusage accounts for actual HTTP model attempts independently of
// provider protocol, retries, response validation, and tool execution.
package modelusage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/google/uuid"
)

type recorderKey struct{}

// Recorder observes consumption; recording errors belong to its implementation,
// never to the model request's response/error channel.
type Recorder func(model.ModelCallUsage)

func WithRecorder(ctx context.Context, record Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, record)
}

type transport struct {
	base     http.RoundTripper
	provider string
	model    string
}

func NewTransport(base http.RoundTripper, provider, modelName string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &transport{base: base, provider: provider, model: modelName}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	record, _ := req.Context().Value(recorderKey{}).(Recorder)
	if record == nil {
		return t.base.RoundTrip(req)
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	started := time.Now()
	call := model.ModelCallUsage{CallID: uuid.NewString(), Provider: t.provider, Model: t.model}
	response, err := t.base.RoundTrip(req)
	if err != nil {
		call.DurationMS = time.Since(started).Milliseconds()
		record(call)
		return response, err
	}
	body := response.Body
	if body == nil {
		body = http.NoBody
	}
	response.Body = &usageBody{ReadCloser: body, record: record, call: call, started: started}
	return response, nil
}

const maximumUsageBody = 16 << 20

type usageBody struct {
	io.ReadCloser
	record    Recorder
	call      model.ModelCallUsage
	started   time.Time
	buffer    bytes.Buffer
	truncated bool
	mu        sync.Mutex
	once      sync.Once
}

func (b *usageBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.mu.Lock()
	remaining := maximumUsageBody - b.buffer.Len()
	if n > remaining {
		b.truncated = true
	}
	if remaining > 0 {
		_, _ = b.buffer.Write(p[:min(n, remaining)])
	}
	b.mu.Unlock()
	if err != nil {
		b.finish()
	}
	return n, err
}

func (b *usageBody) Close() error {
	err := b.ReadCloser.Close()
	b.finish()
	return err
}

func (b *usageBody) finish() {
	b.once.Do(func() {
		b.mu.Lock()
		if !b.truncated {
			b.call.Usage, b.call.Model = readUsage(b.buffer.Bytes(), b.call.Model)
		}
		b.mu.Unlock()
		b.call.DurationMS = time.Since(b.started).Milliseconds()
		// Recording is observational: its failure cannot turn a valid provider
		// response into a read error, cancellation, or retry of a billable call.
		b.record(b.call)
	})
}

// Cache creation/read counts are additive in Messages; cached-token details in
// Responses are already included in input_tokens and must not be added again.
func readUsage(raw []byte, modelName string) (*model.TokenUsage, string) {
	var wire struct {
		Model json.RawMessage            `json:"model"`
		Usage map[string]json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(raw, &wire) != nil || wire.Usage == nil {
		return nil, modelName
	}
	var actualModel string
	if json.Unmarshal(wire.Model, &actualModel) == nil && strings.TrimSpace(actualModel) != "" {
		modelName = strings.TrimSpace(actualModel)
	}
	read := func(keys ...string) (int64, bool) {
		for _, key := range keys {
			if value, exists := wire.Usage[key]; exists {
				var n *int64
				if json.Unmarshal(value, &n) != nil || n == nil || *n < 0 || *n > 1<<53-1 {
					return 0, false
				}
				return *n, true
			}
		}
		return 0, false
	}
	input, inputOK := read("input_tokens", "prompt_tokens")
	output, outputOK := read("output_tokens", "completion_tokens")
	if !inputOK || !outputOK {
		return nil, modelName
	}
	for _, key := range []string{"cache_creation_input_tokens", "cache_read_input_tokens"} {
		if _, exists := wire.Usage[key]; exists {
			n, ok := read(key)
			if !ok || n > 1<<53-1-input {
				return nil, modelName
			}
			input += n
		}
	}
	if output > 1<<53-1-input {
		return nil, modelName
	}
	total := input + output
	if _, exists := wire.Usage["total_tokens"]; exists {
		n, ok := read("total_tokens")
		if !ok || n < total || n > 1<<53-1 {
			return nil, modelName
		}
		total = n
	}
	return &model.TokenUsage{InputTokens: input, OutputTokens: output, TotalTokens: total}, modelName
}
