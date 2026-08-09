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
