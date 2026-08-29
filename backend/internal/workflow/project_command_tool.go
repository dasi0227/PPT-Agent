package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type projectCommandTool struct{}

func (projectCommandTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "run_command",
		Description: "Run a restricted project-local command. Use read_ppt for PPT resources changed in the current Run. Supported reads: ls, cat, head, tail, find, grep, jq, rg, pwd, stat, sed -n, wc, git status, git diff, and git log. The only write form is a confirmed single-file sed -i substitution in execute mode.",
		Parameters: objectSchema([]string{"command"}, map[string]any{
			"command": map[string]any{
				"type": "string", "minLength": 1, "maxLength": 4096,
				"description": "Restricted command text. Runtime classifies access and owns approval.",
			},
		}),
	}
}

func (projectCommandTool) Preflight(_ context.Context, input DomainToolInput) ToolDecision {
	source, _ := input.Args["command"].(string)
	policy, err := commandexec.NewPolicy(input.ProjectDir)
	if err != nil {
		return ToolDecision{Outcome: string(commandexec.Deny), ReasonCode: commandexec.CodePathInvalid, PublicReason: err.Error()}
	}
	decision := policy.Evaluate(source, input.Mode == model.ModeExecute && input.Phase == PhaseExecuting)
	if decision.Mutates && input.Session != nil && len(decision.TargetPaths) == 1 {
		ref := ArtifactRef{Kind: ArtifactProjectFile, Path: decision.TargetPaths[0]}
		if content, readErr := input.Session.Read(ref); readErr == nil {
			decision.PreimageHash = commandexec.ContentHash(content)
		} else {
			decision.Outcome = commandexec.Deny
			decision.ReasonCode = commandexec.CodePathInvalid
			decision.PublicReason = readErr.Error()
		}
	}
	return ToolDecision{
		Outcome: string(decision.Outcome), Mutates: decision.Mutates,
		CommandHash: decision.CommandHash, Command: decision.Display,
		ReasonCode: decision.ReasonCode, PublicReason: decision.PublicReason,
		TargetPaths:  append([]string(nil), decision.TargetPaths...),
		PreimageHash: decision.PreimageHash, Prepared: decision,
	}
}

func (projectCommandTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if input.Decision == nil {
		return failedToolResult(commandexec.CodeInvariantViolation, "command preflight decision is missing", false)
	}
	decision, ok := input.Decision.Prepared.(commandexec.Decision)
	if !ok || decision.CommandHash != input.Decision.CommandHash {
		return failedToolResult(commandexec.CodeInvariantViolation, "command preflight decision is invalid", false)
	}
	executor, err := commandexec.NewExecutor(input.ProjectDir)
	if err != nil {
		return commandFailure(decision, commandexec.Result{}, err)
	}
	if decision.Mutates {
		return executeProjectFileEdit(ctx, input, executor, decision)
	}
	result, err := executor.Execute(ctx, decision.Graph)
	if err != nil {
		return commandFailure(decision, result, err)
	}
	toolResult := SuccessfulToolResult("command completed")
	toolResult.Data = commandResultData(result)
	toolResult.Command = publicCommandExecution(decision, result, "completed", "")
	observation, _ := json.Marshal(toolResult)
	toolResult.Observation = string(observation)
	return toolResult
}

func executeProjectFileEdit(
	ctx context.Context,
	input DomainToolInput,
	executor *commandexec.Executor,
	decision commandexec.Decision,
) ToolResult {
	if input.Session == nil || len(decision.TargetPaths) != 1 ||
		len(decision.Graph.Groups) != 1 || len(decision.Graph.Groups[0].Commands) != 1 {
		return failedToolResult(commandexec.CodeInvariantViolation, "sed edit requires one staged project file", false)
	}
	path := decision.TargetPaths[0]
	ref := ArtifactRef{Kind: ArtifactProjectFile, ID: path, Path: path}
	before, err := input.Session.Read(ref)
	if err != nil {
		return commandFailure(decision, commandexec.Result{}, err)
	}
	if commandexec.ContentHash(before) != decision.PreimageHash {
		return commandFailure(decision, commandexec.Result{}, errors.New("approved command target changed before execution"))
	}
	updated, execution, err := executor.ExecuteSedBytes(ctx, decision.Graph.Groups[0].Commands[0], before)
	if err != nil {
		return commandFailure(decision, execution, err)
	}
	change, err := input.Session.Write(ref, "run_command", updated)
	if err != nil {
		return commandFailure(decision, execution, err)
	}
	toolResult := SuccessfulToolResult("command completed")
	toolResult.Data = commandResultData(execution)
	toolResult.ChangedTargets = []ChangedTarget{{
		Type: "file", Part: "content", Path: path, DisplayName: path,
		Hash: change.AfterHash, Insertions: change.Insertions, Deletions: change.Deletions,
	}}
	toolResult.Command = publicCommandExecution(decision, execution, "completed", "")
	observation, _ := json.Marshal(toolResult)
	toolResult.Observation = string(observation)
	return toolResult
}

func commandFailure(decision commandexec.Decision, execution commandexec.Result, err error) ToolResult {
	code := commandexec.ErrorCode(err)
	if errors.Is(err, context.Canceled) {
		code = CodeCanceled
	}
	summary := strings.TrimSpace(err.Error())
	result := failedToolResult(code, summary, false)
	result.Data = commandResultData(execution)
	result.Command = publicCommandExecution(decision, execution, "failed", summary)
	return result
}

func commandResultData(result commandexec.Result) map[string]any {
	return map[string]any{
		"stdout": result.Stdout, "stderr": result.Stderr, "exit_code": result.ExitCode,
		"duration_ms": result.DurationMS(), "output_truncated": result.OutputTruncated,
	}
}

func publicCommandExecution(decision commandexec.Decision, result commandexec.Result, status, reason string) *CommandExecution {
	return &CommandExecution{
		Text: decision.Display, Status: status, ExitCode: result.ExitCode,
		Sensitive:  decision.ReasonCode == "SENSITIVE_PROJECT_READ",
		DurationMS: result.DurationMS(), OutputTruncated: result.OutputTruncated,
		Stdout: result.Stdout, Stderr: result.Stderr, Reason: reason,
	}
}

func projectFileRef(path string) ArtifactRef {
	return ArtifactRef{Kind: ArtifactProjectFile, ID: filepath.ToSlash(path), Path: filepath.ToSlash(path)}
}
