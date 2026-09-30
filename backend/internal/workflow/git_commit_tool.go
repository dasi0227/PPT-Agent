package workflow

import (
	"context"
	"encoding/json"
)

// GitCommitExecutor binds the current Run's project and thread at the service boundary.
type GitCommitExecutor func(context.Context, string, map[string]any) (map[string]any, error)

type gitCommitTool struct{ execute GitCommitExecutor }

func (gitCommitTool) Schema() ToolSchema {
	return GitCommitToolSchema()
}

// GitCommitToolSchema is shared by the agent loop and the /commit command.
// Both submit the same arguments to the project's local commit service.
func GitCommitToolSchema() ToolSchema {
	return ToolSchema{
		Name: "git_commit", OutputSchema: toolOutputSchema("git_commit"),
		Description: "Commit the current project's durable source files to its local Git repository. Ground the title and summary items in the supplied staged changes; if no change evidence was supplied, inspect git status and git diff with run_command first. Includes all project-whitelisted source changes, not just the current slide scope; excludes caches, exports and thumbnails. Does not push. Use when the user requests a commit or a saved project version; do not commit after every edit. If a commit result is uncertain, inspect history and report it rather than issuing another commit. In /commit, call this tool exactly once without accompanying text; the host executes the commit and ends the command without another model turn.",
		Parameters: objectSchema([]string{"title", "items"}, map[string]any{
			"title": map[string]any{"type": "string", "minLength": 1, "maxLength": 72, "description": "Single-line Chinese title summarizing the actual source changes being committed: <feat|fix|refactor|perf|chore|docs>: <primary change>, without a trailing period or invented completion claims."},
			"items": map[string]any{"type": "array", "minItems": 1, "maxItems": 6,
				"items":       map[string]any{"type": "string", "minLength": 1, "maxLength": 160, "description": "One distinct actual change in one Chinese sentence, without a bullet prefix or unsupported claims of testing or completion."},
				"description": "Commit body entries summarizing the inspected source changes. Runtime formats them as a bullet list; do not include future work."},
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
	visible := map[string]any{"summary": result.Summary}
	if value["empty"] == true {
		visible["summary"] = "当前项目没有可提交的变更。"
	} else {
		visible["hash"], visible["branch"] = value["hash"], value["branch"]
	}
	raw, _ := json.Marshal(visible)
	result.Observation = string(raw)
	return result
}
