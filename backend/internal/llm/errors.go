package llm

import "errors"

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
