package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"os"
	"path/filepath"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
)

type projectCommandTool struct{}

func (projectCommandTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "run_command", OutputSchema: toolOutputSchema("run_command"),
		Description: "Run a restricted project-local command against the current durable project state. Supported reads: ls, cat, head, tail, find, grep, jq, rg, pwd, stat, sed -n, wc, git status, git diff, and git log. The only write form is a confirmed single-file sed -i substitution in execute mode. Read the full exact target text with single-file cat first; read_resource also supplies exact text for outline and HTML, but JSON objects do not authorize text edits. Command output is saved verbatim with domain validation, without formatting. A stale or unseen version is rejected.",
		Parameters: objectSchema([]string{"command"}, map[string]any{
			"command": map[string]any{
				"type": "string", "minLength": 1, "maxLength": 4096,
				"description": "Command text to inspect project files or Git state, using the supported commands and project-local paths. Editing is limited to a single-file sed -i substitution in execute mode after reading the full current target; Runtime requests user approval before execution.",
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
	if decision.Mutates && decision.Outcome != commandexec.Deny {
		raw, _, readErr := readArtifact(input.ProjectDir, input.Session, projectFileRef(decision.TargetPaths[0]))
		if readErr != nil || !commandVersionMatches(input, decision.TargetPaths[0], raw) {
			decision.Outcome, decision.ReasonCode = commandexec.Deny, CodeContentConflict
			decision.PublicReason = "The exact text of " + decision.TargetPaths[0] + " has not been read or has changed. " + commandReadAction(decision.TargetPaths[0])
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
	release := pptmutation.ReadLockProject(input.ProjectDir)
	defer release()
	result, err := executor.Execute(ctx, decision.Graph)
	if err != nil {
		return commandFailure(decision, result, err)
	}
	toolResult := SuccessfulToolResult("command completed")
	toolResult.Data = commandResultData(result)
	toolResult.Command = publicCommandExecution(decision, result, "completed", "")
	toolResult.Observation = modelToolObservation(toolResult)
	// Only an untransformed complete single-file read authorizes later edits.
	if len(decision.Graph.Groups) == 1 && len(decision.Graph.Groups[0].Commands) == 1 && len(decision.TargetPaths) == 1 && !result.OutputTruncated {
		args := decision.Graph.Groups[0].Commands[0].Args
		if len(args) == 2 && args[0] == "cat" {
			path := decision.TargetPaths[0]
			raw, readErr := os.ReadFile(filepath.Join(input.ProjectDir, filepath.FromSlash(path)))
			if readErr == nil && string(raw) == result.Stdout {
				toolResult.ObservationMetadata = commandVersionMetadata(path, raw)
			}
		}
	}
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
	if commandexec.ContentHash(before) != decision.PreimageHash || !commandVersionMatches(input, path, before) {
		return commandConflictFailure(decision, commandexec.Result{})
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
	toolResult.ObservationMetadata = commandVersionMetadata(path, updated)
	toolResult.Observation = modelToolObservation(toolResult)
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
	result.Observation = modelToolObservation(result)
	return result
}

func commandResultData(result commandexec.Result) map[string]any {
	data := map[string]any{"stdout": result.Stdout, "stderr": result.Stderr, "exit_code": result.ExitCode}
	if result.OutputTruncated {
		data["output_truncated"] = true
	}
	return data
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

func commandVersionMetadata(path string, raw []byte) *llm.MessageMetadata {
	stamps := []llm.ResourceStamp{{Key: "file/" + path, Hash: commandexec.ContentHash(raw)}}
	add := func(resource Resource, content []byte) {
		stamps = append(stamps, llm.ResourceStamp{Key: "ppt/" + resource.Key(), Hash: resourceContentHash(resource, content)})
	}
	switch path {
	case ".manifest.json":
		add(Resource{Type: "deck", Part: "manifest"}, raw)
	case ".design.json":
		add(Resource{Type: "deck", Part: "design"}, raw)
	case ".outline.json":
		add(Resource{Type: "deck", Part: "outline"}, raw)
	case model.SpecCollectionPath:
		var entries map[string]json.RawMessage
		if json.Unmarshal(raw, &entries) == nil {
			for id, content := range entries {
				add(Resource{Type: "slide", Part: "spec", SlideID: id}, content)
			}
		}
	default:
		id := strings.TrimSuffix(path, ".html")
		if id != path && stableSlideID.MatchString(id) {
			add(Resource{Type: "slide", Part: "html", SlideID: id}, raw)
		}
	}
	return &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: stamps}
}

func commandVersionMatches(input DomainToolInput, path string, raw []byte) bool {
	key := "file/" + path
	seen := input.SeenVersions[key]
	if seen == "" {
		seen = visibleResourceHashes(input.Messages)[key]
	}
	if seen != "" && seen == commandexec.ContentHash(raw) {
		return true
	}
	resource := Resource{}
	switch path {
	case ".outline.json":
		resource = Resource{Type: "deck", Part: "outline"}
	default:
		id := strings.TrimSuffix(path, ".html")
		if id != path && stableSlideID.MatchString(id) {
			resource = Resource{Type: "slide", Part: "html", SlideID: id}
		}
	}
	if resource.Type == "" {
		return false
	}
	hash, err := modelSeenResourceVersion(input, resource)
	return err == nil && hash != "" && hash == resourceContentHash(resource, raw)
}

func commandReadAction(path string) string {
	command := "cat '" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
	raw, _ := json.Marshal(map[string]any{"command": command})
	return fmt.Sprintf("Read the full target with run_command %s, then regenerate the edit from its current exact text; do not blindly retry the old substitution.", raw)
}

func commandConflictFailure(decision commandexec.Decision, execution commandexec.Result) ToolResult {
	path := decision.TargetPaths[0]
	result := detailedToolFailure(CodeContentConflict, "The exact text of "+path+" changed since the model read or approval.", map[string]any{"next_action": commandReadAction(path)})
	for key, value := range commandResultData(execution) {
		result.Data[key] = value
	}
	result.Command = publicCommandExecution(decision, execution, "failed", result.Summary)
	result.Observation = modelToolObservation(result)
	return result
}
