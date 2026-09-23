package workflow

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func agentRequestForState(input RuntimeInput, state *RunState, schemas []ToolSchema) AgentRequest {
	skills, components := state.activeSkills.Snapshot()
	return AgentRequest{
		RunID: state.runID, LoopID: state.loopID, Phase: state.phase, Mode: state.mode,
		Context: state.pack, Plan: state.plan, Changes: state.changeSet(), Evidence: state.ledger.Entries(state.changeSet()),
		Requirements: state.requirements, Work: state.work, ContextBriefing: state.contextBriefing,
		RenderedImages: state.renderedImages, ActiveSkills: skills,
		LoadedComponents: components,
		Messages:         append([]llm.Message{}, state.messages...), Tools: schemas, ImageResolver: input.ImageResolver,
		Continuation: state.continuation, InstructionInMessages: containsRunInstruction(state.messages, state.runID),
	}
}

// prepareAgentRequest appends current facts without rewriting earlier turns.
// Rebuilding after a retry is idempotent; compaction removes metadata with its
// messages, so missing sections are restored rather than assumed visible.
func prepareAgentRequest(req AgentRequest) AgentRequest {
	req.Mode = effectivePromptMode(req.Mode, req.Context.Command.Mode)
	req.Context.Command.Mode = req.Mode
	req.Messages = append([]llm.Message{}, req.Messages...)
	if !req.InstructionInMessages && !containsRunInstruction(req.Messages, req.RunID) {
		command := req.Context.Command
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleUser, Content: referenceMessageParts(
			command.Instruction, req.Context.Project.ID, command.Attachments, command.DOMSelections, command.ReferenceOrder,
		), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: req.RunID}})
	}
	sections := contextengine.ModelSections(req.Context)
	var state map[string]any
	_ = json.Unmarshal([]byte(runtimeTaskStateForRequest(req)), &state)
	for key, value := range state {
		sections["task/"+key] = value
	}
	sections["mentioned_pages"] = contextengine.ModelValue(req.Context.Command.MentionedPages)
	// Explicit resource snapshots become individual once-per-content sections.
	for _, skill := range req.ActiveSkills {
		sections["skill/"+skill.ID] = skillBody(skill)
	}
	delivered := map[string]bool{}
	for _, component := range req.LoadedComponents {
		delivered[component.ID] = true
	}
	for _, component := range req.Context.Command.Components {
		if !delivered[component.ID] {
			sections["component/"+component.ID] = componentBody(component)
		}
	}
	activeIDs := []string{}
	for _, skill := range req.ActiveSkills {
		activeIDs = append(activeIDs, skill.ID)
	}
	sort.Strings(activeIDs)
	sections["task/active_skills"] = activeIDs
	last := map[string]*llm.MessageMetadata{}
	for _, message := range req.Messages {
		if m := message.Metadata; m != nil && m.Origin == "runtime" && m.Kind == "context" {
			last[m.Key] = m
		}
		if m := message.Metadata; m != nil && m.Origin == "runtime" {
			for _, stamp := range m.Resources {
				if !strings.HasPrefix(stamp.Key, "skill/") && !strings.HasPrefix(stamp.Key, "component/") {
					continue
				}
				last[stamp.Key] = &llm.MessageMetadata{Origin: "runtime", Kind: "context", Key: stamp.Key, Hash: stamp.Hash}
			}
		}
	}
	// Absence explicitly clears a previous resource selection.
	for key := range last {
		if _, exists := sections[key]; !exists && !strings.HasPrefix(key, "skill/") && !strings.HasPrefix(key, "component/") {
			sections[key] = nil
		}
	}
	keys := make([]string, 0, len(sections))
	for key := range sections {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if sections[key] == nil && last[key] == nil {
			continue
		}
		raw, _ := json.Marshal(sections[key])
		if last[key] == nil && (string(raw) == "[]" || string(raw) == "{}" || string(raw) == `""`) {
			continue
		}
		hash := hashBytes(raw)
		runID := ""
		if strings.HasPrefix(key, "task/") || key == "run_command" {
			runID = req.RunID
		}
		if previous := last[key]; previous != nil && previous.Hash == hash && previous.RunID == runID {
			continue
		}
		body, _ := json.Marshal(map[string]any{"section": key, "value": json.RawMessage(raw)})
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleUser, Content: llm.TextContent("<runtime_context>\n" + string(body) + "\n</runtime_context>"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "context", Key: key, Hash: hash, RunID: runID}})
	}
	return req
}

func providerMessages(req AgentRequest) []llm.Message {
	return append([]llm.Message{{Role: llm.RoleSystem, Content: llm.TextContent(runtimeSystemPromptForRequest(req))}}, req.Messages...)
}

func runtimeControlMetadata(kind, runID string) *llm.MessageMetadata {
	return &llm.MessageMetadata{Origin: "runtime", Kind: kind, RunID: runID}
}
