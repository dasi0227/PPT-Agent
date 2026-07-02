// Package tools 提供 harness 的工具原语：带 JSON schema 的确定性脚本。
// M1 只落 registry + patch/validate/finish 三原语；slide/style/asset 业务工具留到后续里程碑。
package tools

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Class 是工具类别，供动态门控按 scope/mode 裁剪（ARCH-HARNESS-001）。
type Class string

const (
	ClassRead     Class = "read"
	ClassWrite    Class = "write"
	ClassValidate Class = "validate"
	ClassControl  Class = "control"
)

// Artifact 是一次产物落盘引用（SSE artifact 事件的负载来源）。
type Artifact struct {
	Type string // slide_html / design / asset / file
	Ref  string // work_dir 相对路径
}

// Result 是工具执行结果，回灌为 ReAct observation。
type Result struct {
	OK          bool
	Observation string
	IsFinish    bool
	Summary     string
	Artifact    *Artifact
}

// Tool 是一个工具原语：暴露 function schema + 门控元数据 + 确定性执行。
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any // JSON Schema
	Class() Class
	// Scopes 返回该工具可用的 scope 集合；空表示所有 scope（如 finish）。
	Scopes() []model.Scope
	// Execute 校验参数后执行；参数非法或越界 MUST 整体失败并返回可操作错误 observation。
	Execute(ctx context.Context, args map[string]any) (Result, error)
}

// Schema 将 Tool 投影为注册给 LLM 的 function schema。
func Schema(t Tool) llm.ToolSchema {
	return llm.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.Parameters(),
	}
}
