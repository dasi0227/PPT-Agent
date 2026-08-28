package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	defaultProviderTimeout = 180 * time.Second
	defaultMaxImageBytes   = 4 * 1024 * 1024
	defaultMaxRetries      = 5
)

type adapterHTTP struct {
	client     *http.Client
	apiKey     string
	baseURL    string
	maxRetries int
}

func newAdapterHTTP(apiKey, baseURL string, timeout time.Duration) adapterHTTP {
	if timeout <= 0 {
		timeout = defaultProviderTimeout
	}
	return adapterHTTP{
		client: &http.Client{Timeout: timeout}, apiKey: apiKey,
		baseURL: strings.TrimRight(baseURL, "/"), maxRetries: defaultMaxRetries,
	}
}

func (h adapterHTTP) doJSON(
	ctx context.Context,
	path string,
	body any,
	onRetry func(int),
	out any,
) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("%w: encode request", ErrBadRequest)
	}
	endpoint, err := providerEndpoint(h.baseURL, path)
	if err != nil {
		return fmt.Errorf("%w: invalid provider endpoint", ErrBadRequest)
	}
	var lastErr error
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		if attempt > 0 {
			if onRetry != nil {
				onRetry(attempt)
			}
			timer := time.NewTimer(providerBackoff(attempt))
			select {
			case <-ctx.Done():
				timer.Stop()
				return providerContextError(ctx)
			case <-timer.C:
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
		if err != nil {
			return fmt.Errorf("%w: create request", ErrBadRequest)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
		resp, err := h.client.Do(req)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return providerContextError(ctx)
			}
			lastErr = fmt.Errorf("%w: provider request failed", ErrUnavailable)
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out)
			_ = resp.Body.Close()
			if decodeErr != nil {
				return fmt.Errorf("%w: decode provider response", ErrUnavailable)
			}
			return nil
		}
		rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
		if readErr != nil {
			rawBody = nil
		}
		if resp.StatusCode == http.StatusRequestTimeout ||
			resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = providerHTTPError(resp, rawBody, ErrUnavailable)
			continue
		}
		return providerHTTPError(resp, rawBody, ErrBadRequest)
	}
	if lastErr == nil {
		lastErr = ErrUnavailable
	}
	return lastErr
}

func providerContextError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return context.Canceled
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: provider request timed out", ErrUnavailable)
	}
	if ctx.Err() == nil {
		return context.Canceled
	}
	return ctx.Err()
}

func providerEndpoint(baseURL, path string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid base URL")
	}
	want := "/" + strings.TrimLeft(path, "/")
	basePath := strings.TrimRight(parsed.Path, "/")
	if basePath == "/v1" && strings.HasPrefix(want, "/v1/") {
		want = strings.TrimPrefix(want, "/v1")
	}
	parsed.Path = basePath + want
	return parsed.String(), nil
}

func providerBackoff(attempt int) time.Duration {
	return time.Duration(attempt*attempt) * 200 * time.Millisecond
}

func providerHTTPError(resp *http.Response, rawBody []byte, kind error) error {
	out := &ProviderError{
		Kind:       kind,
		StatusCode: resp.StatusCode,
		RequestID:  providerRequestID(resp.Header),
		BodyBytes:  len(rawBody),
	}
	if len(rawBody) > 0 {
		sum := sha256.Sum256(rawBody)
		out.BodySHA256 = hex.EncodeToString(sum[:8])
	}
	code, typ, message := parseProviderErrorBody(rawBody)
	out.Code = sanitizeProviderDiagnostic(code, 96)
	out.Type = sanitizeProviderDiagnostic(typ, 96)
	out.Message = sanitizeProviderDiagnostic(message, 512)
	return out
}

func providerRequestID(header http.Header) string {
	for _, key := range []string{
		"x-request-id",
		"x-ds-request-id",
		"x-deepseek-request-id",
		"request-id",
		"cf-ray",
	} {
		if value := strings.TrimSpace(header.Get(key)); value != "" {
			return sanitizeProviderDiagnostic(value, 128)
		}
	}
	return ""
}

func parseProviderErrorBody(raw []byte) (code, typ, message string) {
	if len(raw) == 0 || !json.Valid(raw) {
		return "", "", ""
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", "", ""
	}
	if errorValue, ok := body["error"].(map[string]any); ok {
		return diagnosticField(errorValue["code"]), diagnosticField(errorValue["type"]), diagnosticField(errorValue["message"])
	}
	return diagnosticField(body["code"]), diagnosticField(body["type"]), diagnosticField(body["message"])
}

func diagnosticField(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

var providerSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bauthorization["']?\s*[:=]\s*["']?(?:bearer\s+)?[^"'\s,;}]+["']?`),
	regexp.MustCompile(`(?i)\b(?:api[_-]?key|token|secret)["']?\s*[:=]\s*["']?[^"'\s,;}]+["']?`),
	regexp.MustCompile(`(?i)\bsk-[a-z0-9._-]+\b`),
}

func sanitizeProviderDiagnostic(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.Join(strings.Fields(value), " ")
	for _, pattern := range providerSecretPatterns {
		value = pattern.ReplaceAllString(value, "[REDACTED]")
	}
	if limit > 0 && len(value) > limit {
		value = value[:limit] + "...[truncated]"
	}
	return value
}

func validateContinuation(continuation *ProviderContinuation, provider, model string) error {
	if continuation == nil {
		return nil
	}
	if continuation.Provider != provider || continuation.Model != model {
		return fmt.Errorf("%w: continuation belongs to another provider or model", ErrBadRequest)
	}
	return nil
}

func prepareProviderImage(data ImageData, capabilities Capabilities) ([]byte, string, error) {
	if len(data.Bytes) == 0 {
		return nil, "", errors.New("image is empty")
	}
	maxBytes := capabilities.MaxImageBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxImageBytes
	}
	supports := func(mimeType string) bool {
		for _, candidate := range capabilities.ImageInputMIMEs {
			if candidate == mimeType {
				return true
			}
		}
		return false
	}
	if supports(data.MIMEType) && len(data.Bytes) <= maxBytes {
		return data.Bytes, data.MIMEType, nil
	}
	source, _, err := image.Decode(bytes.NewReader(data.Bytes))
	if err != nil {
		return nil, "", errors.New("image cannot be decoded")
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
					return nil, "", errors.New("image cannot be encoded")
				}
				if encoded.Len() <= maxBytes {
					return encoded.Bytes(), "image/jpeg", nil
				}
			}
		}
		if supports("image/png") {
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, target); err != nil {
				return nil, "", errors.New("image cannot be encoded")
			}
			if encoded.Len() <= maxBytes {
				return encoded.Bytes(), "image/png", nil
			}
		}
	}
	return nil, "", errors.New("image exceeds provider byte limit")
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

func decodeToolArguments(raw string) (map[string]any, error) {
	args := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return args, nil
	}
	if err := json.Unmarshal([]byte(raw), &args); err == nil {
		return args, nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	if err := decoder.Decode(&args); err != nil {
		return nil, err
	}
	rest := strings.TrimSpace(raw[decoder.InputOffset():])
	for _, r := range rest {
		if r != '}' {
			return nil, errors.New("unexpected trailing tool arguments")
		}
	}
	return args, nil
}
