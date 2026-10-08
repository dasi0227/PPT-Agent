package workflow

import (
	"encoding/json"
	"html"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func agentRequestForState(input RuntimeInput, state *RunState, schemas []ToolSchema) AgentRequest {
	skills, components := state.activeSkills.Snapshot()
	return AgentRequest{
		RunID: state.runID, LoopID: state.loopID, Phase: state.phase, Mode: state.mode,
		Context: state.pack, Plan: state.plan, Changes: state.changeSet(), Evidence: state.ledger.Entries(state.changeSet()),
		ProjectState: state.projectState, RetrievedInfo: state.retrievedInfo,
		RenderedImages: state.renderedImages, ReadImages: append([]RunReadImage(nil), state.readImages...), ActiveSkills: skills,
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
	req.ReadImages = currentRunImages(req.ReadImages, req.RenderedImages)
	req.Messages = appendRunImages(req)
	req.RuntimeContext = runtimeContextMessages(req)
	return req
}

type runtimeModule struct {
	id, description string
	value           any
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
	retrieved := req.RetrievedInfo
	if retrieved == nil {
		retrieved = []RetrievedInfo{}
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
		{"available_skills", "Lists the available skills with their names, descriptions and tags.", availableSkills},
		{"available_components", "Lists the available components with their names, descriptions and tags.", availableComponents},
		{"active_skills", "Provides the full content of currently active skills, grouped by skill ID.", skills},
		{"active_components", "Provides the full content of currently loaded components, grouped by component ID.", components},
		{"run_state", "Provides the current run mode, authorized slides and execution phase.", struct {
			Mode  model.RunMode `json:"run_mode"`
			Scope []string      `json:"run_scope"`
			Phase RunPhase      `json:"run_phase"`
		}{req.Mode, append([]string{}, req.Context.Command.Scope.SlideIDs...), req.Phase}},
		{"project_state", "Provides current resource availability, slide render freshness and cumulative changes in this run.", req.ProjectState},
		{"plan", "Provides the current plan, step progress and approval state.", plan},
		{"retrieved_info", "Provides supplementary reference information retrieved for the current task.", retrieved},
	}
	messages := make([]llm.Message, 0, len(modules))
	for _, module := range modules {
		raw, err := json.MarshalIndent(module.value, "", "  ")
		if err != nil {
			panic(err)
		} // Only closed, JSON-safe projection types above.
		stamps := []llm.ResourceStamp{}
		switch module.id {
		case "active_skills":
			for _, skill := range req.ActiveSkills {
				stamps = append(stamps, resourceStamp("skill/"+skill.ID, skillBody(skill)))
			}
		case "active_components":
			for key, body := range components {
				stamps = append(stamps, resourceStamp(key, map[string]string{"id": strings.TrimPrefix(key, "component/"), "content": body}))
			}
			sort.Slice(stamps, func(i, j int) bool { return stamps[i].Key < stamps[j].Key })
		}
		messages = append(messages, llm.Message{Role: llm.RoleUser,
			Content:  llm.TextContent(`<runtime_context id="` + html.EscapeString(module.id) + `" desc="` + html.EscapeString(module.description) + `">` + "\n" + string(raw) + "\n</runtime_context>"),
			Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "context", Key: module.id, Hash: hashBytes(raw), RunID: req.RunID, Resources: stamps},
		})
	}
	return messages
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
