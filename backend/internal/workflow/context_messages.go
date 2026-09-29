package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func agentRequestForState(input RuntimeInput, state *RunState, schemas []ToolSchema) AgentRequest {
	skills, components := state.activeSkills.Snapshot()
	_, outlineErr := os.Stat(filepath.Join(input.ProjectDir, ".outline.json"))
	return AgentRequest{
		OutlineExists: outlineErr == nil,
		RunID:         state.runID, LoopID: state.loopID, Phase: state.phase, Mode: state.mode,
		Context: state.pack, Plan: state.plan, Changes: state.changeSet(), Evidence: state.ledger.Entries(state.changeSet()),
		Requirements: state.requirements, Work: state.work, ContextBriefing: state.contextBriefing,
		RenderedImages: state.renderedImages, ReadImages: append([]RunReadImage(nil), state.readImages...), ActiveSkills: skills,
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
	if !req.InstructionInMessages {
		req.Messages = appendRunInstruction(req.Messages, req.Context.Command, req.Context.Project.ID, req.RunID)
	}
	sections := contextengine.ModelSections(req.Context)
	sections["task/outline_exists"] = req.OutlineExists
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
		stamps := []llm.ResourceStamp{}
		switch key {
		case "project_context":
			stamps = append(stamps, llm.ResourceStamp{Key: "ppt/" + (Resource{Type: "deck", Part: "manifest"}).Key(), Hash: spec.ResourceHash(req.Context.PresentationManifest.Manifest)})
		case "design_context":
			stamps = append(stamps, llm.ResourceStamp{Key: "ppt/" + (Resource{Type: "deck", Part: "design"}).Key(), Hash: spec.ResourceHash(req.Context.Design.Design)})
		}
		if strings.HasPrefix(key, "page/") && len(req.Context.Target.SlideIDs) == 1 && req.Context.Target.SlideSpec != nil && key == "page/"+req.Context.Target.SlideIDs[0] {
			stamps = append(stamps, llm.ResourceStamp{Key: "ppt/" + (Resource{Type: "slide", Part: "spec", SlideID: req.Context.Target.SlideIDs[0]}).Key(), Hash: spec.ResourceHash(*req.Context.Target.SlideSpec)})
		}
		body, _ := json.Marshal(map[string]any{"section": key, "value": json.RawMessage(raw)})
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleUser, Content: llm.TextContent("<runtime_context>\n" + string(body) + "\n</runtime_context>"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "context", Key: key, Hash: hash, RunID: runID, Resources: stamps}})
	}
	req.Messages = appendRunImages(req)
	return req
}

func appendRunInstruction(messages []llm.Message, command model.RunCommand, projectID, runID string) []llm.Message {
	if containsRunInstruction(messages, runID) {
		return messages
	}
	return append(messages, llm.Message{Role: llm.RoleUser, Content: referenceMessageParts(
		command.Instruction, projectID, command.Attachments, command.DOMSelections, command.ReferenceOrder,
	), Metadata: &llm.MessageMetadata{Origin: "user", Kind: "instruction", RunID: runID}})
}

func providerMessages(req AgentRequest) []llm.Message {
	return append([]llm.Message{{Role: llm.RoleSystem, Content: llm.TextContent(runtimeSystemPromptForRequest(req))}}, req.Messages...)
}

func runtimeControlMetadata(kind, runID string) *llm.MessageMetadata {
	return &llm.MessageMetadata{Origin: "runtime", Kind: kind, RunID: runID}
}
