package workflow

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// Resolve identities and cascading deletions before freezing the approval diff.
// The mutation service reads the original outline, while writes affect only the
// isolated candidate. Neither approval nor recovery reruns these operations.
func normalizeCommandOutline(before, after commandexec.Tree) error {
	old, existed := before[".outline.json"]
	next, exists := after[".outline.json"]
	if existed == exists && old.Hash() == next.Hash() {
		return nil
	}
	if exists {
		if next.Kind != "file" {
			return fmt.Errorf("outline must be a regular file")
		}
		workspace := &commandOutlineWorkspace{before: before, after: after}
		return (pptmutation.Service{Workspace: workspace}).EditResource(pptmutation.ResourceEdit{Resource: "outline", Content: string(next.Content)})
	}
	if !existed {
		return nil
	}
	parsed, err := spec.ParseStrictSourceJSON(old.Content, "outline")
	if err != nil {
		return err
	}
	for _, loc := range spec.FlattenOutline(*parsed.(*spec.Outline)) {
		delete(after, model.SlideSpecPath(loc.Slide.ID))
		delete(after, model.SlideHTMLPath(loc.Slide.ID))
	}
	return nil
}

type commandOutlineWorkspace struct{ before, after commandexec.Tree }

func (w *commandOutlineWorkspace) Read(path string) ([]byte, error) {
	node, ok := w.before[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if node.Kind != "file" {
		return nil, fmt.Errorf("%s must be a regular file", path)
	}
	return node.Content, nil
}
func (w *commandOutlineWorkspace) Write(path string, content []byte) error {
	node := w.after[path]
	node.Kind, node.Content = "file", content
	w.after[path] = node
	return nil
}
func (w *commandOutlineWorkspace) Delete(path string) error { delete(w.after, path); return nil }

func (p preparedCommand) session(projectDir string) *RunSession {
	outline, exists := p.Before[".outline.json"]
	s := &RunSession{projectDir: projectDir, commandBaseline: p.Decision.PreimageHash, artifacts: map[string]sessionArtifact{},
		readBaselines: map[string]sessionBaseline{".outline.json": {content: outline.Content, exists: exists}}}
	for path, node := range p.Changes {
		s.stageCommandNode(path, p.Before, node)
	}
	return s
}

func (p preparedCommand) matches(source string) bool {
	d := commandexec.Evaluate(source)
	return p.Err == nil && d.Outcome != commandexec.Deny && p.Decision.Source == source &&
		p.Decision.Outcome == commandexec.Confirm && p.Decision.Mutates && len(p.Changes) > 0 &&
		p.Decision.PreimageHash != "" && p.Decision.CommandHash == commandexec.ContentHash([]byte(d.CommandHash+commandChangesHash(p.Changes)))
}

func (r *Runtime) approveCommandCandidate(ctx context.Context, input RuntimeInput, state *RunState, call llm.ToolCall, decision *ToolDecision) (string, ToolResult, bool) {
	pending := state.pendingCommand
	if pending == nil {
		prepared, ok := decision.Prepared.(preparedCommand)
		if !ok || !prepared.matches(stringValue(call.Args["command"])) {
			return "invalid", failedToolResult(model.CodeInternal, "Command candidate is missing or invalid."), true
		}
		pending = &PendingCommandApproval{InteractionID: "cmdperm_" + call.ID, CallID: call.ID, Args: call.Args, Prepared: &prepared, ResumePhase: state.phase}
		state.pendingCommand = pending
	}
	prepared := pending.Prepared
	if pending.Answer == nil {
		prompter, ok := input.Prompter.(CommandPermissionPrompter)
		if !ok {
			return "not_answered", failedToolResult(model.CodeInternal, "command permission prompter is required"), true
		}
		state.resumePhase = pending.ResumePhase
		r.changePhase(input.Emitter, state, PhaseWaitingInput, "command changes require approval")
		if err := r.saveCheckpoint(ctx, input, state, checkpointBeforeCommandPermission); err != nil {
			return "not_answered", persistenceToolFailure(err), true
		}
		state.pauseActiveClock(r.clockNow())
		answer, err := prompter.AskCommandPermission(ctx, model.CommandPermissionRequestedPayload{
			PublicEventBase: publicBase(state.runID), InteractionID: pending.InteractionID,
			CallID: call.ID, Command: decision.Command, CommandHash: decision.CommandHash,
			Changes: prepared.Targets, Stdout: sanitizeCommandPreview(prepared.Execution.Stdout, state.publicTextContext()), Stderr: sanitizeCommandPreview(prepared.Execution.Stderr, state.publicTextContext()),
		})
		state.resumeActiveClock(r.clockNow())
		if err != nil {
			return "not_answered", commandFailure(prepared.Decision, prepared.Execution, err), true
		}
		if answer.InteractionID != pending.InteractionID || answer.CallID != call.ID || answer.CommandHash != decision.CommandHash ||
			(answer.Decision != "allow_once" && answer.Decision != "deny") {
			return "invalid", failedToolResult(model.CodeInternal, "command permission answer does not match the pending candidate"), true
		}
		pending.Answer = &answer
		r.changePhase(input.Emitter, state, pending.ResumePhase, "command permission answered")
		// Persist the decision with its candidate until the durable receipt and
		// observation exist. Recovery can apply it once without running the shell.
		if err := r.saveCheckpoint(ctx, input, state, checkpointAfterCommandPermission); err != nil {
			return "not_answered", persistenceToolFailure(err), true
		}
	}
	if pending.Answer.InteractionID != pending.InteractionID || pending.Answer.CallID != call.ID || pending.Answer.CommandHash != decision.CommandHash ||
		(pending.Answer.Decision != "allow_once" && pending.Answer.Decision != "deny") {
		return "invalid", failedToolResult(model.CodeInternal, "Stored command approval does not match the candidate."), true
	}
	if pending.Answer.Decision == "deny" {
		result := refusedCommandResult(*decision)
		setApprovalFeedback(&result, pending.Answer.Feedback, state.runID, call.ID, "command_feedback")
		return "deny", result, true
	}
	decision.Approved = true
	return "allow_once", ToolResult{}, false
}
