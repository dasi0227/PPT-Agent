package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

type projectCommandTool struct{}
type preparedCommand struct {
	Decision  commandexec.Decision
	Before    commandexec.Tree
	Changes   map[string]*commandexec.Node
	Execution commandexec.Result
	Targets   []model.PublicTarget
	Err       error `json:"-"`
}

func (projectCommandTool) Schema() ToolSchema { return tooldefinition.Get("run_command") }

func (projectCommandTool) Preflight(ctx context.Context, input DomainToolInput) ToolDecision {
	source, _ := input.Args["command"].(string)
	d := commandexec.Evaluate(source)
	prepared := preparedCommand{Decision: d}
	if d.Outcome != commandexec.Deny {
		prepared = prepareShellCommand(ctx, input, d)
	}
	return prepared.toolDecision()
}

func (p preparedCommand) toolDecision() ToolDecision {
	d := p.Decision
	return ToolDecision{Outcome: string(d.Outcome), Mutates: d.Mutates, CommandHash: d.CommandHash, Command: d.Display, ReasonCode: d.ReasonCode, PublicReason: d.PublicReason, TargetPaths: d.TargetPaths, PreimageHash: d.PreimageHash, Prepared: p}
}

func prepareShellCommand(ctx context.Context, input DomainToolInput, d commandexec.Decision) preparedCommand {
	p := preparedCommand{Decision: d}
	if _, err := commandexec.NewExecutor(input.ProjectDir); err != nil {
		p.Err = err
		return p
	}
	release := pptmutation.ReadLockProject(input.ProjectDir)
	before, err := commandexec.Snapshot(input.ProjectDir)
	release()
	if err != nil {
		p.Err = err
		return p
	}
	p.Before = before
	p.Decision.PreimageHash = before.Hash()
	p.Changes, p.Execution, p.Err = runIsolatedShell(ctx, input, p.Decision, before)
	if p.Err != nil {
		return p
	}
	if len(p.Changes) > 0 {
		p.Decision.Mutates = true
		p.Decision.Outcome = commandexec.Confirm
		p.Decision.ReasonCode = "PROJECT_WRITE"
		p.Decision.PublicReason = "命令将更改项目文件。"
		for path := range p.Changes {
			p.Decision.TargetPaths = append(p.Decision.TargetPaths, path)
		}
		sort.Strings(p.Decision.TargetPaths)
		session := p.session(input.ProjectDir)
		p.Targets = operationDiffTargets(session)
		// Store only affected preimages; the full snapshot hash still guards commit.
		preimages := commandexec.Tree{}
		for path := range p.Changes {
			if node, exists := p.Before[path]; exists {
				preimages[path] = node
			}
		}
		p.Before = preimages
		// Recovery approval is bound to the actual proposed changes, not merely source.
		p.Decision.CommandHash = commandexec.ContentHash([]byte(d.CommandHash + commandChangesHash(p.Changes)))
	}
	if p.Decision.Mutates && (input.Mode != model.ModeExecute || input.Phase != PhaseExecuting) {
		p.Decision.Outcome = commandexec.Deny
		p.Decision.ReasonCode = model.CodeCapabilityDenied
		p.Decision.PublicReason = "The current mode does not permit command writes."
	}
	return p
}

func runIsolatedShell(ctx context.Context, input DomainToolInput, d commandexec.Decision, before commandexec.Tree) (map[string]*commandexec.Node, commandexec.Result, error) {
	workspace, err := os.MkdirTemp("", "ppt-agent-command-*")
	if err != nil {
		return nil, commandexec.Result{}, err
	}
	defer commandexec.RemoveWorkspace(workspace)
	workspace, err = filepath.EvalSymlinks(workspace)
	if err != nil {
		return nil, commandexec.Result{}, err
	}
	if err = before.CopyTo(workspace, input.ProjectDir); err != nil {
		return nil, commandexec.Result{}, err
	}
	initial, err := commandexec.Snapshot(workspace)
	if err != nil {
		return nil, commandexec.Result{}, err
	}
	executor, err := commandexec.NewExecutor(workspace)
	if err != nil {
		return nil, commandexec.Result{}, err
	}
	execution, err := executor.Execute(ctx, d)
	execution.Stdout = strings.ReplaceAll(execution.Stdout, workspace, input.ProjectDir)
	execution.Stderr = strings.ReplaceAll(execution.Stderr, workspace, input.ProjectDir)
	if err != nil {
		return nil, execution, err
	}
	after, err := commandexec.Snapshot(workspace)
	if err != nil {
		return nil, execution, err
	}
	if err := normalizeCommandOutline(before, after); err != nil {
		return nil, execution, toolResultError(candidateContentInvalid(err, Resource{Type: "deck", Part: "outline"}))
	}
	changes := map[string]*commandexec.Node{}
	for path, node := range after {
		old, exists := initial[path]
		if exists && old.Hash() == node.Hash() {
			continue
		}
		if commandexec.ProtectedPath(path) {
			return nil, execution, &commandexec.Error{Code: commandexec.CodeNotAllowed, Message: "Protected resource cannot be changed by a command: " + path}
		}
		if err := commandexec.ValidateLink(workspace, path, node); err != nil {
			return nil, execution, err
		}
		copy := node
		changes[path] = &copy
	}
	for path := range initial {
		if _, exists := after[path]; !exists {
			if commandexec.ProtectedPath(path) {
				return nil, execution, &commandexec.Error{Code: commandexec.CodeNotAllowed, Message: "Protected resource cannot be removed: " + path}
			}
			changes[path] = nil
		}
	}
	// Validate before asking for approval, without exposing the staged copy to other tools.
	if len(changes) > 0 {
		session := (preparedCommand{Decision: d, Before: before, Changes: changes}).session(input.ProjectDir)
		if err := session.ValidateStagedArtifacts(input.Context); err != nil {
			return nil, execution, toolResultError(candidateContentInvalid(err, Resource{Type: "file", Part: "content", Path: "project"}))
		}
	}
	return changes, execution, nil
}

func commandChangesHash(changes map[string]*commandexec.Node) string {
	tree := commandexec.Tree{}
	for path, node := range changes {
		if node == nil {
			tree[path] = commandexec.Node{Kind: "deleted"}
		} else {
			tree[path] = *node
		}
	}
	return tree.Hash()
}

func (projectCommandTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if input.Decision == nil {
		return failedToolResult(model.CodeInternal, "Command preflight is missing.")
	}
	prepared, ok := input.Decision.Prepared.(preparedCommand)
	if !ok || prepared.Decision.CommandHash != input.Decision.CommandHash {
		return failedToolResult(model.CodeInternal, "Invalid command preparation.")
	}
	d := prepared.Decision
	if prepared.Err != nil {
		return commandFailure(d, prepared.Execution, prepared.Err)
	}
	if err := ctx.Err(); err != nil {
		return commandFailure(d, prepared.Execution, err)
	}
	if d.Mutates && !input.Decision.Approved {
		return failedToolResult(model.CodeInternal, "Command changes require explicit approval.")
	}
	if d.Mutates && input.Session == nil {
		return failedToolResult(model.CodeInternal, "Command writes require an active write session.")
	}
	if d.Mutates {
		release := pptmutation.ReadLockProject(input.ProjectDir)
		current, err := commandexec.Snapshot(input.ProjectDir)
		release()
		if err != nil {
			return commandFailure(d, prepared.Execution, err)
		}
		if current.Hash() != d.PreimageHash {
			return commandConflictFailure(d, prepared.Execution)
		}
	}
	result := SuccessfulToolResult("command completed")
	if len(prepared.Changes) > 0 {
		input.Session.commandBaseline = d.PreimageHash
		for path, node := range prepared.Changes {
			input.Session.stageCommandNode(path, prepared.Before, node)
		}
		if err := input.Session.ValidateStagedArtifacts(input.Context); err != nil {
			return commandFailure(d, prepared.Execution, toolResultError(candidateContentInvalid(err, Resource{Type: "file", Part: "content", Path: "project"})))
		}
		result.OperationTargets = prepared.Targets
		for _, change := range input.Session.ChangeSet().All() {
			target := resourceForArtifact(change.Artifact)
			result.ChangedTargets = append(result.ChangedTargets, ChangedTarget{Type: target.Type, SlideID: target.SlideID, Part: target.Part, Path: target.Path, DisplayName: change.Artifact.Path, Hash: change.AfterHash, Insertions: change.Insertions, Deletions: change.Deletions})
		}
	}
	result.Data = commandResultData(prepared.Execution)
	result.Command = publicCommandExecution(d, prepared.Execution, "completed", "")
	if path := d.FullReadPath; path != "" && len(prepared.Changes) == 0 {
		if node, ok := prepared.Before[path]; ok && node.Kind == "file" && string(node.Content) == prepared.Execution.Stdout {
			result.ObservationMetadata = commandVersionMetadata(path, node.Content)
		}
	}
	result.Observation = modelToolObservation(result)
	return result
}

func commandResourceFailure(err error, resource Resource) ToolResult {
	var source *pptmutation.ResourceSourceError
	if errors.As(err, &source) {
		// Command edits retain their saved-source validity errors, including Spec.
		if source.Resource == "outline" && errors.Is(source.Err, os.ErrNotExist) {
			return outlineMissingFailure(resource)
		}
		return resourceReadFailure(source.Err, Resource{Type: "deck", Part: source.Resource})
	}
	if errors.Is(err, pptmutation.ErrSlideNotFound) || errors.Is(err, pptmutation.ErrOutlineNotExist) {
		return resourceReadToolFailure(err, resource)
	}
	if errors.Is(err, os.ErrNotExist) {
		return detailedToolFailure(model.CodeNotFound, "Command target "+resource.Key()+" does not exist.", map[string]any{"measure": "Restore or create the reported target only when needed and authorized, then retry the command."})
	}
	return internalToolFailure(err)
}

func commandFailure(decision commandexec.Decision, execution commandexec.Result, err error) ToolResult {
	code := commandexec.ErrorCode(err)
	if errors.Is(err, context.Canceled) {
		code = CodeCanceled
	}
	summary := strings.TrimSpace(err.Error())
	result := failedToolResult(code, summary)
	result.Data = commandResultData(execution)
	result.Command = publicCommandExecution(decision, execution, "failed", summary)
	result.Observation = modelToolObservation(result)
	return result
}

func commandResultData(result commandexec.Result) map[string]any {
	return map[string]any{"stdout": result.Stdout, "stderr": result.Stderr, "exit_code": result.ExitCode}
}

func publicCommandExecution(decision commandexec.Decision, result commandexec.Result, status, reason string) *CommandExecution {
	return &CommandExecution{
		Text: decision.Display, Status: status, ExitCode: result.ExitCode,

		DurationMS: result.DurationMS(),
		Stdout:     result.Stdout, Stderr: result.Stderr, Reason: reason,
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
	default:
		if id, ok := model.SlideIDFromSpecPath(path); ok {
			add(Resource{Type: "slide", Part: "spec", SlideID: id}, raw)
		} else if id, ok := model.SlideIDFromHTMLPath(path); ok {
			add(Resource{Type: "slide", Part: "html", SlideID: id}, raw)
		}
	}
	return &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: stamps}
}

func commandConflictFailure(decision commandexec.Decision, execution commandexec.Result) ToolResult {
	result := detailedToolFailure(model.CodeContentConflict, "The project changed after command preparation or approval.", map[string]any{"measure": "Inspect current files before resubmitting for fresh approval. If network access was used, check external effects before repeating it."})
	for key, value := range commandResultData(execution) {
		result.Data[key] = value
	}
	result.Command = publicCommandExecution(decision, execution, "failed", result.Summary)
	result.Observation = modelToolObservation(result)
	return result
}
