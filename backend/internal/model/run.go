// Package model 定义零依赖的领域类型（ARCH-BACKEND-003）：无 ORM/框架 import。
package model

// Scope 是 Run 的作用域，决定 harness 动态工具门控裁剪的工具集（ARCH-HARNESS-001）。
type Scope string

const (
	ScopeCurrent  Scope = "current"
	ScopePage     Scope = "page"
	ScopeOverview Scope = "overview"
	ScopeRepo     Scope = "repo"
)

// Mode 是 Run 的模式，进一步收窄工具集与 needs_input 倾向。
type Mode string

const (
	ModeNormal Mode = "normal"
	ModeTalk   Mode = "talk"
	ModeAsk    Mode = "ask"
)

// Kind 是 Run 的种类。
type Kind string

const (
	KindOutline  Kind = "outline"
	KindGenerate Kind = "generate"
	KindEdit     Kind = "edit"
	KindCommand  Kind = "command"
)

// RunStatus 是 Run 生命周期状态游标（单一真相，无冗余标志 ARCH-RUN-001）。
type RunStatus string

const (
	RunPending  RunStatus = "pending"
	RunRunning  RunStatus = "running"
	RunWaiting  RunStatus = "waiting"
	RunDone     RunStatus = "done"
	RunFailed   RunStatus = "failed"
	RunCanceled RunStatus = "canceled"
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
