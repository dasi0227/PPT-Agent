package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ComponentLoader interface {
	Get(string) (model.Component, error)
}

type SkillLoader interface {
	ResolveDynamic([]string) ([]model.RunSkill, error)
}

type ThemeLoader interface {
	Get(string) (model.Theme, error)
}

type loadComponentTool struct {
	loader ComponentLoader
}

const maxLoadedComponentBytes = 192 << 10

func (loadComponentTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "load_component",
		Description: "Load enabled repository component HTML references by stable ids for adaptation. Loads the whole batch or fails, with at most 192 KiB of HTML per call. This does not inject or modify project files.",
		Parameters: objectSchema([]string{"ids"}, map[string]any{
			"ids": map[string]any{
				"type": "array", "minItems": 1, "maxItems": 8, "uniqueItems": true,
				"items": map[string]any{"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`},
			},
		}),
	}
}

func (t loadComponentTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	if t.loader == nil {
		return failedToolResult("INTERNAL", "component repository is unavailable", false)
	}
	result := SuccessfulToolResult("components loaded")
	snapshots := []model.RunComponent{}
	totalBytes := 0
	for _, rawID := range input.Args["ids"].([]any) {
		id := rawID.(string)
		component, err := t.loader.Get(id)
		if err != nil {
			return repositoryLoadFailure("component", CodeResourceNotFound, fmt.Sprintf("component %q was not found", id))
		}
		if component.Disabled || component.ContentState != "ready" {
			return repositoryLoadFailure("component", CodeResourceNotFound, fmt.Sprintf("component %q is disabled or unavailable", id))
		}
		snapshot := model.RunComponent{ID: component.ID, Name: component.Name, Description: component.Description, HTML: component.HTML, LocalPath: component.LocalPath, OpenURL: component.OpenURL}
		// Explicitly selected resources remain pinned to the user's run snapshot.
		for _, selected := range input.Context.Command.Components {
			if selected.ID == id {
				snapshot = selected
				break
			}
		}
		totalBytes += len(snapshot.HTML)
		if totalBytes > maxLoadedComponentBytes {
			return detailedToolFailure(CodeContentTooLarge, fmt.Sprintf("component %q exceeds the batch HTML budget of 192 KiB", id), map[string]any{"next_action": "Call load_component with fewer or smaller components. The failed batch loaded no components; an individually oversized component must be replaced."})
		}
		snapshots = append(snapshots, snapshot)
		result.LoadedResources = append(result.LoadedResources, LoadedResource{
			Kind: "component", ID: component.ID, Name: component.Name,
			LocalPath: component.LocalPath, OpenURL: component.OpenURL,
		})
	}
	// Do not remember any component until the entire batch has passed validation.
	visible := visibleResourceHashes(input.Messages)
	bodies := []map[string]string{}
	available := []string{}
	stamps := []llm.ResourceStamp{}
	for _, snapshot := range snapshots {
		input.ActiveSkills.RememberComponent(snapshot)
		body := componentBody(snapshot)
		stamp := resourceStamp("component/"+snapshot.ID, body)
		if visible[stamp.Key] == stamp.Hash {
			available = append(available, snapshot.ID)
			continue
		}
		bodies = append(bodies, body)
		stamps = append(stamps, stamp)
	}
	observation, _ := json.Marshal(map[string]any{"loaded": len(bodies), "components": bodies, "already_available": available, "replaces_previous": true})
	result.Observation = string(observation)
	result.ObservationMetadata = &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: stamps}
	return result
}

type loadSkillTool struct {
	loader SkillLoader
}

func (loadSkillTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "load_skill",
		Description: "Load one or more enabled repository skills into the current run active skill set.",
		Parameters: objectSchema([]string{"ids"}, map[string]any{
			"ids": map[string]any{
				"type": "array", "minItems": 1, "maxItems": 8, "uniqueItems": true,
				"items": map[string]any{"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`},
			},
		}),
	}
}

func (t loadSkillTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	if t.loader == nil || input.ActiveSkills == nil {
		return failedToolResult(CodeRunSessionRequired, "active run skill state is unavailable", false)
	}
	rawIDs, ok := input.Args["ids"].([]any)
	if !ok {
		if values, typed := input.Args["ids"].([]string); typed {
			rawIDs = make([]any, len(values))
			for index := range values {
				rawIDs[index] = values[index]
			}
		}
	}
	if len(rawIDs) == 0 || len(rawIDs) > 8 {
		return failedToolResult(CodeContentInvalid, "ids must contain 1 to 8 unique skill ids", false)
	}
	ids := make([]string, 0, len(rawIDs))
	seen := map[string]bool{}
	for _, raw := range rawIDs {
		id, valid := raw.(string)
		id = strings.TrimSpace(id)
		if !valid || id == "" || seen[id] {
			return failedToolResult(CodeContentInvalid, "ids must contain unique skill ids", false)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	skills, err := t.loader.ResolveDynamic(ids)
	if err != nil {
		var agentErr *model.AgentError
		if errors.As(err, &agentErr) {
			return repositoryLoadFailure("skill", agentErr.Code, agentErr.Error())
		}
		return repositoryLoadFailure("skill", CodeResourceNotFound, err.Error())
	}
	// Explicitly selected resources remain pinned to the user's run snapshot.
	for i, skill := range skills {
		for _, selected := range input.Context.Command.Skills {
			if selected.ID == skill.ID {
				skills[i] = selected
				break
			}
		}
	}
	input.ActiveSkills.Add(skills)
	result := SuccessfulToolResult("skills loaded")
	result.LoadedResources = make([]LoadedResource, 0, len(skills))
	for _, skill := range skills {
		result.LoadedResources = append(result.LoadedResources, LoadedResource{
			Kind: "skill", ID: skill.ID, Name: skill.Name,
			LocalPath: skill.LocalPath, OpenURL: skill.OpenURL,
		})
	}
	visible := visibleResourceHashes(input.Messages)
	bodies := []map[string]string{}
	available := []string{}
	stamps := []llm.ResourceStamp{}
	for _, skill := range skills {
		body := skillBody(skill)
		stamp := resourceStamp("skill/"+skill.ID, body)
		if visible[stamp.Key] == stamp.Hash {
			available = append(available, skill.ID)
			continue
		}
		bodies = append(bodies, body)
		stamps = append(stamps, stamp)
	}
	observation, _ := json.Marshal(map[string]any{"loaded": len(bodies), "skills": bodies, "already_available": available, "replaces_previous": true})
	result.Observation = string(observation)
	result.ObservationMetadata = &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: stamps}
	return result
}

func repositoryLoadFailure(kind, code, summary string) ToolResult {
	switch code {
	case "SKILL_NOT_FOUND", "SKILL_DISABLED", "COMPONENT_NOT_FOUND", "COMPONENT_DISABLED":
		code = CodeResourceNotFound
	case "SKILL_SELECTION_INVALID", "COMPONENT_SELECTION_INVALID":
		code = CodeToolArgumentInvalid
	case CodeContextBudget:
		return detailedToolFailure(CodeContentTooLarge, summary, map[string]any{"next_action": "Load fewer or smaller " + kind + " entries. The failed batch loaded no entries; do not retry the same oversized batch."})
	case CodeResourceNotFound:
	default:
		return failedToolResult(code, summary, false)
	}
	return detailedToolFailure(code, summary, map[string]any{
		"next_action": "Choose enabled " + kind + " IDs from the current repository catalog. Remove missing or disabled IDs before calling load_" + kind + " again; do not invent IDs or read the PPT outline to locate repository entries.",
	})
}

func skillContext(skills []model.RunSkill) []map[string]string {
	out := make([]map[string]string, 0, len(skills))
	for _, skill := range skills {
		out = append(out, map[string]string{
			"id": skill.ID, "name": skill.Name, "description": skill.Description, "content": skill.Content,
		})
	}
	return out
}
