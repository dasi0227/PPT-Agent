package service

import "errors"

// ErrSlideTargetNotFound is returned before a Run is persisted when a stable
// slide scope does not belong to the thread's project.
var ErrSlideTargetNotFound = errors.New("service: slide target not found")

// ErrInvalidProject：项目创建参数非法。
var ErrInvalidProject = errors.New("service: invalid project")

// ErrRunActive：project 有活跃 run 时，手动写操作（如 slide PATCH）被互斥拒绝（映射 409 RUN_ACTIVE）。
var ErrRunActive = errors.New("service: project has an active run")

var ErrRunScopeUnsupported = errors.New("service: run scope unsupported")

var ErrScreenshotNotFound = errors.New("service: render screenshot not found")

var ErrThemeNotFound = errors.New("service: theme not found")

var ErrGitCommitActive = errors.New("service: project has an active Git commit")

var ErrGitCommitInvalid = errors.New("service: invalid Git commit request")

var ErrGitCommitToolUnsupported = errors.New("service: model does not support Git commit tool calls")
