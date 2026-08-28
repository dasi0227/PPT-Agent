package llm

import (
	"errors"
	"fmt"
	"strings"
)

// 错误哨兵，映射到 api-overview 错误码表（由上层转译）。
var (
	// ErrBadRequest 对应 LLM 上游 4xx（鉴权/参数），不重试（ARCH-LLM 重试表）。
	ErrBadRequest = errors.New("llm bad request")
	// ErrUnavailable 对应限流/超时/5xx 重试耗尽。
	ErrUnavailable = errors.New("llm unavailable")
	// ErrBadToolCall 对应 function call 无法解析（非法 JSON / 缺必需参数），
	// 走最小失败退出，不进入重试死循环（ARCH-LLM-FC-001）。
	ErrBadToolCall = errors.New("llm bad tool call")
)

// ProviderError carries a bounded, sanitized upstream diagnostic. It is safe
// for development logs and trace projections, but still avoids request bodies,
// credentials, and full provider payloads.
type ProviderError struct {
	Kind       error
	StatusCode int
	Code       string
	Type       string
	Message    string
	RequestID  string
	BodySHA256 string
	BodyBytes  int
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{e.Kind.Error()}
	if e.StatusCode > 0 {
		parts = append(parts, fmt.Sprintf("provider_status=%d", e.StatusCode))
	}
	if e.Code != "" {
		parts = append(parts, "provider_code="+e.Code)
	}
	if e.Type != "" {
		parts = append(parts, "provider_type="+e.Type)
	}
	if e.Message != "" {
		parts = append(parts, "provider_message="+e.Message)
	}
	if e.RequestID != "" {
		parts = append(parts, "provider_request_id="+e.RequestID)
	}
	if e.BodySHA256 != "" {
		parts = append(parts, "response_sha256="+e.BodySHA256)
	}
	if e.BodyBytes > 0 {
		parts = append(parts, fmt.Sprintf("response_bytes=%d", e.BodyBytes))
	}
	return strings.Join(parts, ": ")
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}
