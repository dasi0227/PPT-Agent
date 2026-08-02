package run

import "errors"

var (
	// ErrLockTimeout：锁等待超时，Run 应转 failed（ARCH-RUN-LOCK-005）。
	ErrLockTimeout = errors.New("run: project lock acquire timeout")
	// ErrRunNotFound：run 不存在。
	ErrRunNotFound = errors.New("run: not found")
	// ErrRunNotRunning：run 非可注入状态（映射 409 RUN_NOT_RUNNING，API-RUN-001）。
	ErrRunNotRunning = errors.New("run: not running")
	// ErrReplyMismatch：reply_to 或结构化答案未匹配当前 pending question。
	ErrReplyMismatch           = errors.New("run: reply does not match the pending question")
	ErrContextStoreUnavailable = errors.New("run: context manifest store unavailable")
)
