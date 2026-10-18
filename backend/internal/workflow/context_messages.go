package workflow

import (
	"encoding/json"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func agentRequestForState(input RuntimeInput, state *RunState, schemas []ToolSchema) AgentRequest {
	skills, components := state.activeSkills.Snapshot()
	return AgentRequest{
		RunID: state.runID, LoopID: state.loopID, Phase: state.phase, Mode: state.mode,
		Context: state.pack, Plan: state.plan, Changes: state.changeSet(), Evidence: state.ledger.Entries(),
		ProjectState:     state.projectState,
		ActiveSkills:     skills,
		LoadedComponents: components,
		Messages:         append([]llm.Message{}, state.messages...), Tools: schemas, ImageResolver: input.ImageResolver,
		Continuation: state.continuation, InstructionInMessages: containsRunInstruction(state.messages, state.runID),
	}
}

// Messages contains conversation only. RuntimeContext is rebuilt independently
// on every request and must never be stored in the transcript or checkpoint.
func prepareAgentRequest(req AgentRequest) AgentRequest {
	req.Mode = effectivePromptMode(req.Mode, req.Context.Command.Mode)
	req.Context.Command.Mode = req.Mode
	req.Messages = llm.WithoutRequestContext(req.Messages)
	if !req.InstructionInMessages {
		req.Messages = appendRunInstruction(req.Messages, req.Context.Command, req.Context.Project.ID, req.RunID)
	}
	req.ProjectState = req.ProjectState.withAvailableResources(req.Messages)
	req.RuntimeContext = runtimeContextMessages(req)
	return req
}

type runtimeModule struct {
	id    string
	value any
}

func runtimeContextMessages(req AgentRequest) []llm.Message {
	skills, components := map[string]string{}, map[string]string{}
	for _, skill := range req.ActiveSkills {
		skills["skill/"+skill.ID] = skill.Content
	}
	for _, component := range req.LoadedComponents {
		components["component/"+component.ID] = component.HTML
	}
	for _, component := range req.Context.Command.Components {
		components["component/"+component.ID] = component.HTML
	}
	availableSkills := req.Context.Skills
	if availableSkills == nil {
		availableSkills = []contextengine.SkillCandidate{}
	}
	availableComponents := req.Context.Components
	if availableComponents == nil {
		availableComponents = []contextengine.ComponentCandidate{}
	}
	var plan any
	if p := req.Plan; p != nil {
		plan = struct {
			Title    string     `json:"title"`
			Content  string     `json:"content"`
			Status   PlanStatus `json:"status"`
			Approved bool       `json:"approved"`
			Steps    []PlanStep `json:"steps"`
		}{p.Title, p.Content, p.Status, p.ApprovalID != "" && p.ApprovedContentHash != "" && p.ApprovedContentHash == p.ContentHash() && (p.Status == PlanActive || p.Status == PlanCompleted), clonePlanSteps(p.Steps)}
	}
	modules := []runtimeModule{
		{"available_skills", availableSkills},
		{"available_components", availableComponents},
		{"active_skills", skills},
		{"active_components", components},
		{"run_state", struct {
			Mode                    model.RunMode `json:"run_mode"`
			Scope                   []string      `json:"run_scope"`
			IncludeRunCreatedSlides bool          `json:"include_run_created_slides"`
			Phase                   RunPhase      `json:"run_phase"`
		}{req.Mode, append([]string{}, req.Context.Command.Scope.SlideIDs...), req.Context.Command.Scope.IncludeRunCreatedSlides, req.Phase}},
		{"project_state", req.ProjectState},
		{"current_plan", plan},
	}
	var body strings.Builder
	body.WriteString("<runtime_context>\n")
	for _, module := range modules {
		raw, err := json.MarshalIndent(module.value, "", "  ")
		if err != nil {
			panic(err)
		} // Only closed, JSON-safe projection types above.
		body.WriteString("<" + module.id + ">\n" + string(raw) + "\n</" + module.id + ">\n")
	}
	body.WriteString("</runtime_context>")
	text := body.String()
	return []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent(text),
		Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "context", Hash: hashBytes([]byte(text)), RunID: req.RunID}}}
}

func appendRunInstruction(messages []llm.Message, command model.RunCommand, projectID, runID string) []llm.Message {
	if containsRunInstruction(messages, runID) {
		return messages
	}
	return append(messages, llm.Message{Role: llm.RoleUser, Content: referenceMessageParts(
		instructionWithMentions(command), projectID, command.Attachments, command.DOMSelections, command.ReferenceOrder,
	), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: runID}})
}

// Restored composer inputs can carry validated mentions without the inline ID.
// Keep that relationship in the user message, never in runtime state.
func instructionWithMentions(command model.RunCommand) string {
	text := command.Instruction
	for _, page := range command.MentionedPages {
		if !strings.Contains(text, "⟨"+page.SlideID+"⟩") {
			text += "\n@" + page.Title + "⟨" + page.SlideID + "⟩"
		}
	}
	return text
}

func providerMessages(req AgentRequest) []llm.Message {
	messages := []llm.Message{{Role: llm.RoleSystem, Content: llm.TextContent(runtimeSystemPromptForRequest(req))}}
	return append(messages, requestContextMessages(req)...)
}

func runtimeControlMetadata(kind, runID string) *llm.MessageMetadata {
	return &llm.MessageMetadata{Origin: "runtime", Kind: kind, RunID: runID}
}

func (state *RunState) modelVisibleMessages() []llm.Message {
	return append(append([]llm.Message{}, state.runtimeContext...), requestConversation(state.messages, state.runID)...)
}
