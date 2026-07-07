package service

import "errors"

// ErrInvalidPageIndex：编辑请求的 page_index 越界或缺失（映射 400 BAD_REQUEST，AC-CMD-PAGE-003）。
// 在创建 Run 之前返回，保证不落 Run、不落文件。
var ErrInvalidPageIndex = errors.New("service: invalid or missing page_index")

// ErrInvalidProject：项目创建参数非法。
var ErrInvalidProject = errors.New("service: invalid project")

// ErrRunActive：project 有活跃 run 时，手动写操作（如 slide PATCH）被互斥拒绝（映射 409 RUN_ACTIVE）。
var ErrRunActive = errors.New("service: project has an active run")
