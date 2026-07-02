// Package harness 是业务无关的认知引擎：ReAct 循环 + 动态工具门控 + 停止条件 + 上下文预算。
// 不依赖具体业务 agent；agent 层构造配置来驱动它（ARCH-HARNESS）。
package harness

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Gate 按 scope/mode 动态裁剪工具集（ARCH-HARNESS-001）。
// 越权工具 MUST NOT 出现在返回集合中——这是 scope 隔离的机制级实现。
func Gate(all []tools.Tool, scope model.Scope, mode model.Mode) []tools.Tool {
	out := make([]tools.Tool, 0, len(all))
	for _, t := range all {
		if !scopeAllows(t, scope) {
			continue
		}
		if !modeAllows(t, mode) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// scopeAllows：Scopes() 为空表示全 scope 可用（如 finish）；否则 scope 必须命中。
func scopeAllows(t tools.Tool, scope model.Scope) bool {
	scopes := t.Scopes()
	if len(scopes) == 0 {
		return true
	}
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// modeAllows：
//   - talk / prompt：不注册任何读写产物工具，只留控制类（finish）——只说不做（ARCH-RUN-006）。
//   - recap：只读 + 控制。
//   - normal / ask：对应 scope 的全部工具。
func modeAllows(t tools.Tool, mode model.Mode) bool {
	switch mode {
	case model.ModeTalk:
		return t.Class() == tools.ClassControl
	default:
		return true
	}
}
