package tools

import (
	"context"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ValidateTool 显式校验沙箱内某文件是否合规，返回 observation（不改文件）。
type ValidateTool struct {
	sandbox   *Sandbox
	validator Validator
}

func NewValidateTool(sandbox *Sandbox, v Validator) *ValidateTool {
	if v == nil {
		v = NopValidator{}
	}
	return &ValidateTool{sandbox: sandbox, validator: v}
}

func (t *ValidateTool) Name() string          { return "validate" }
func (t *ValidateTool) Class() Class          { return ClassValidate }
func (t *ValidateTool) Scopes() []model.Scope { return nil }
func (t *ValidateTool) Description() string {
	return "校验指定文件是否合规，返回校验 observation（不修改文件）。"
}

func (t *ValidateTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"file"},
		"properties": map[string]any{
			"file": map[string]any{"type": "string", "description": "work_dir 内相对路径"},
		},
	}
}

func (t *ValidateTool) Execute(_ context.Context, args map[string]any) (Result, error) {
	file, ok := args["file"].(string)
	if !ok || file == "" {
		return fail("参数错误：缺少 file"), nil
	}
	raw, err := t.sandbox.Read(file)
	if err != nil {
		return fail(fmt.Sprintf("读取失败：%v", err)), nil
	}
	if okv, reason := t.validator.Validate(file, raw); !okv {
		return fail(fmt.Sprintf("校验未通过：%s", reason)), nil
	}
	return Result{OK: true, Observation: fmt.Sprintf("%s 校验通过", file)}, nil
}
