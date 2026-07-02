package tools

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// FinishTool 声明任务完成，退出 ReAct 循环（ARCH-HARNESS-STOP-002）。所有 scope 可用。
type FinishTool struct{}

func NewFinishTool() *FinishTool { return &FinishTool{} }

func (t *FinishTool) Name() string          { return "finish" }
func (t *FinishTool) Class() Class          { return ClassControl }
func (t *FinishTool) Scopes() []model.Scope { return nil }
func (t *FinishTool) Description() string   { return "声明任务完成，退出 ReAct 循环。" }

func (t *FinishTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"summary"},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
		},
	}
}

func (t *FinishTool) Execute(_ context.Context, args map[string]any) (Result, error) {
	summary, _ := args["summary"].(string)
	return Result{OK: true, IsFinish: true, Summary: summary, Observation: "finished"}, nil
}
