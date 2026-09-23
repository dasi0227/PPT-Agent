package workflow

import (
	"context"
	"encoding/json"
	"errors"
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

func (loadComponentTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "load_component",
		Description: "Load one enabled repository component HTML reference by stable id for adaptation. This does not inject or modify project files.",
		Parameters: objectSchema([]string{"id"}, map[string]any{
			"id": map[string]any{"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`},
		}),
	}
}

func (t loadComponentTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	id := strings.TrimSpace(stringValue(input.Args["id"]))
	if t.loader == nil {
		return failedToolResult(CodeResourceNotFound, "component repository is unavailable", false)
	}
	component, err := t.loader.Get(id)
	if err != nil {
		return failedToolResult(CodeResourceNotFound, "component was not found", false)
	}
	if component.Disabled {
		return failedToolResult(CodeResourceNotFound, "component is disabled", false)
	}
	result := SuccessfulToolResult("component loaded")
	result.LoadedResources = []LoadedResource{{
		Kind: "component", ID: component.ID, Name: component.Name,
		LocalPath: component.LocalPath, OpenURL: component.OpenURL,
	}}
	snapshot := model.RunComponent{ID: component.ID, Name: component.Name, Description: component.Description, HTML: component.HTML}
	for _, selected := range input.Context.Command.Components {
		if selected.ID == id {
			snapshot = selected
			break
		}
	}
	input.ActiveSkills.RememberComponent(snapshot)
	body := componentBody(snapshot)
	stamp := resourceStamp("component/"+id, body)
	if visibleResourceHashes(input.Messages)[stamp.Key] == stamp.Hash {
		result.Observation = "The requested component body is already present in the current context. Reuse it."
	} else {
		observation, _ := json.Marshal(map[string]any{"component": body, "replaces_previous": true})
		result.Observation = string(observation)
		result.ObservationMetadata = &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: []llm.ResourceStamp{stamp}}
	}
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
			return failedToolResult(agentErr.Code, agentErr.Error(), agentErr.Retryable)
		}
		return failedToolResult(CodeResourceNotFound, err.Error(), false)
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

func skillContext(skills []model.RunSkill) []map[string]string {
	out := make([]map[string]string, 0, len(skills))
	for _, skill := range skills {
		out = append(out, map[string]string{
			"id": skill.ID, "name": skill.Name, "description": skill.Description, "content": skill.Content,
		})
	}
	return out
}
