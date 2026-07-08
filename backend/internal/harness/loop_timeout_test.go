package harness

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// context.DeadlineExceeded：DeepSeek 客户端整体超时后 http.Client.Do 会返回一个
// wrap 了 context.DeadlineExceeded 的错误，harness 应把它映射为 LLM_TIMEOUT，与
// LLM_BAD_REQUEST 语义严格区分——前者可重试或调低复杂度，后者是参数/权限问题。
func TestStopConditionsLLMTimeoutFromDeadline(t *testing.T) {
	fake := &llmtest.FakeClient{CallToolErr: fmt.Errorf("call llm: %w", context.DeadlineExceeded)}
	loop := New(fake, Config{RunID: "r1", Scope: model.ScopeCurrent, Mode: model.ModeNormal, Tools: []tools.Tool{finishTool()}})
	out := loop.Run(context.Background(), &captureEmitter{}, nil)
	if out.Status != OutcomeLLMError {
		t.Fatalf("want llm_error status, got %s", out.Status)
	}
	if out.Code != CodeLLMTimeout {
		t.Fatalf("want code=%s, got %s", CodeLLMTimeout, out.Code)
	}
}

// http.Client 因 Timeout 触发时，net/http 返回的 err 字面量含 "Client.Timeout"，
// 且未必 wrap context.DeadlineExceeded；用字符串兜底以覆盖这个真实现场。
func TestStopConditionsLLMTimeoutFromClientTimeoutString(t *testing.T) {
	fake := &llmtest.FakeClient{CallToolErr: errors.New(
		"llm unavailable: decode: context deadline exceeded (Client.Timeout or context cancellation while reading body)",
	)}
	loop := New(fake, Config{RunID: "r1", Scope: model.ScopeCurrent, Mode: model.ModeNormal, Tools: []tools.Tool{finishTool()}})
	out := loop.Run(context.Background(), &captureEmitter{}, nil)
	if out.Code != CodeLLMTimeout {
		t.Fatalf("want code=%s from client timeout string, got %s (msg=%s)", CodeLLMTimeout, out.Code, out.Message)
	}
}

// 其它 llm 错误（如 ErrBadToolCall / ErrBadRequest）应继续走 LLM_BAD_REQUEST，
// 避免把参数问题误报为超时导致用户不停重试。
func TestStopConditionsLLMBadCallStaysBadRequest(t *testing.T) {
	fake := &llmtest.FakeClient{CallToolErr: llm.ErrBadToolCall}
	loop := New(fake, Config{RunID: "r1", Scope: model.ScopeCurrent, Mode: model.ModeNormal, Tools: []tools.Tool{finishTool()}})
	out := loop.Run(context.Background(), &captureEmitter{}, nil)
	if out.Code != CodeLLMBadCall {
		t.Fatalf("want code=%s, got %s", CodeLLMBadCall, out.Code)
	}
}
