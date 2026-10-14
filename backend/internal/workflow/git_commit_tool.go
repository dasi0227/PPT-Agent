package workflow

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

// GitCommitExecutor binds the current Run's project and thread at the service boundary.
type GitCommitExecutor func(context.Context, string, map[string]any) (map[string]any, error)

type gitCommitTool struct{ execute GitCommitExecutor }

func (gitCommitTool) Schema() ToolSchema {
	return tooldefinition.Get("git_commit")
}

func (t gitCommitTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	if t.execute == nil {
		return failedToolResult(model.CodeInternal, "Project commit service is unavailable")
	}
	value, err := t.execute(ctx, input.CallID, input.Args)
	if err != nil {
		return internalToolFailure(err)
	}
	result := SuccessfulToolResult("已提交项目版本")
	if value["empty"] == true {
		result.Summary = "当前项目没有可提交的变更"
	}
	result.Data = value
	visible := map[string]any{"summary": result.Summary}
	if value["empty"] == true {
		visible["summary"] = "当前项目没有可提交的变更。"
	} else {
		visible["hash"], visible["branch"] = value["hash"], value["branch"]
	}
	result.Observation = modelSuccessObservation(visible)
	return result
}
