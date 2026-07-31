package run

import "errors"

var (
	// ErrLockTimeout：锁等待超时，Run 应转 failed（ARCH-RUN-LOCK-005）。
	ErrLockTimeout = errors.New("run: project lock acquire timeout")
	// ErrRunNotFound：run 不存在。
	ErrRunNotFound = errors.New("run: not found")
	// ErrRunNotRunning：run 非可注入状态（映射 409 RUN_NOT_RUNNING，API-RUN-001）。
	ErrRunNotRunning = errors.New("run: not running")
	// ErrReplyMismatch：reply_to 未匹配任何未应答的 needs_input（映射 409 CONFLICT，API-RUN-003）。
	ErrReplyMismatch           = errors.New("run: reply_to does not match a pending needs_input")
	ErrContextStoreUnavailable = errors.New("run: context manifest store unavailable")
)
