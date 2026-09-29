package workflow

import "context"

// GitCommitExecutor binds the current Run's project and thread at the service boundary.
type GitCommitExecutor func(context.Context, string, map[string]any) (map[string]any, error)

type gitCommitTool struct{ execute GitCommitExecutor }

func (gitCommitTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "git_commit",
		Description: "Commit the current project's durable source files to its local Git repository. Inspect git status and git diff with run_command first, then provide an accurate title and summary items. Includes all project-whitelisted source changes, not just the current slide scope; excludes caches, exports and thumbnails. Does not push. Use when the user requests a commit or a saved project version; do not commit after every edit. If a commit result is uncertain, inspect history and report it rather than issuing another commit.",
		Parameters: objectSchema([]string{"title", "items"}, map[string]any{
			"title": map[string]any{"type": "string", "minLength": 1, "maxLength": 72, "description": "Single-line commit title: feat/fix/refactor/perf/chore/docs: concise summary."},
			"items": map[string]any{"type": "array", "minItems": 1, "maxItems": 6,
				"items":       map[string]any{"type": "string", "minLength": 1, "maxLength": 160},
				"description": "Plain-text change summaries without bullet prefixes."},
		}),
	}
}

func (t gitCommitTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if t.execute == nil {
		return failedToolResult("GIT_COMMIT_FAILED", "Project commit service is unavailable", false)
	}
	value, err := t.execute(ctx, input.CallID, input.Args)
	if err != nil {
		return failedToolResult("GIT_COMMIT_FAILED", err.Error(), false)
	}
	result := SuccessfulToolResult("已提交项目版本")
	if value["empty"] == true {
		result.Summary = "当前项目没有可提交的变更"
	}
	result.Data = value
	return result
}
