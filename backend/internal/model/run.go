// Package model 定义零依赖的领域类型（ARCH-BACKEND-003）：无 ORM/框架 import。
package model

// RunStatus 是 Run 生命周期状态游标（单一真相，无冗余标志 ARCH-RUN-001）。
type RunStatus string

const (
	RunPending    RunStatus = "pending"
	RunRunning    RunStatus = "running"
	RunWaiting    RunStatus = "waiting"
	RunPaused     RunStatus = "paused"
	RunRecovering RunStatus = "recovering"
	RunDone       RunStatus = "done"
	RunFailed     RunStatus = "failed"
	RunCanceled   RunStatus = "canceled"
)

// Terminal 报告状态是否为终态（不可再注入输入 API-RUN-001）。
func (s RunStatus) Terminal() bool {
	switch s {
	case RunDone, RunFailed, RunCanceled:
		return true
	default:
		return false
	}
}
