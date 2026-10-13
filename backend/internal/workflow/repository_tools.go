package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
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
	return tooldefinition.Get("load_component")
}

func (t loadComponentTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	if t.loader == nil || input.ActiveSkills == nil {
		return failedToolResult("INTERNAL", "component repository is unavailable")
	}
	result := SuccessfulToolResult("components loaded")
	snapshots := []model.RunComponent{}
	for _, rawID := range input.Args["ids"].([]any) {
		id := rawID.(string)
		component, err := t.loader.Get(id)
		if err != nil {
			return repositoryLoadFailure("component", id, err)
		}
		if component.Disabled {
			return repositoryLoadFailure("component", id, model.RepositoryDisabledError("component", id))
		}
		if err := model.RepositoryContentError("component", id, component.ResourceContentState); err != nil {
			return repositoryLoadFailure("component", id, err)
		}
		snapshot := model.RunComponent{ID: component.ID, Name: component.Name, Description: component.Description, HTML: component.HTML, LocalPath: component.LocalPath, OpenURL: component.OpenURL}
		// Explicitly selected resources remain pinned to the user's run snapshot.
		for _, selected := range input.Context.Command.Components {
			if selected.ID == id {
				snapshot = selected
				break
			}
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
	observation, _ := json.Marshal(map[string]any{"components": bodies, "already_available": available})
	result.Observation = string(observation)
	result.ObservationMetadata = &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: stamps}
	return result
}

type loadSkillTool struct {
	loader SkillLoader
}

func (loadSkillTool) Schema() ToolSchema {
	return tooldefinition.Get("load_skill")
}

func (t loadSkillTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	if t.loader == nil || input.ActiveSkills == nil {
		return failedToolResult(model.CodeInternal, "active run skill state is unavailable")
	}
	rawIDs := input.Args["ids"].([]any)
	ids := make([]string, 0, len(rawIDs))
	for _, raw := range rawIDs {
		ids = append(ids, raw.(string))
	}
	skills, err := t.loader.ResolveDynamic(ids)
	if err != nil {
		return repositoryLoadFailure("skill", "", err)
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
	observation, _ := json.Marshal(map[string]any{"skills": bodies, "already_available": available})
	result.Observation = string(observation)
	result.ObservationMetadata = &llm.MessageMetadata{Origin: "runtime", Kind: "resource", Resources: stamps}
	return result
}

func repositoryLoadFailure(kind, id string, err error) ToolResult {
	code := map[string]string{"component": "COMPONENT_NOT_FOUND", "skill": "SKILL_NOT_FOUND"}[kind]
	var failure *model.AgentError
	if errors.As(err, &failure) {
		if failure.Code == code || model.IsCommonErrorCode(failure.Code) {
			return internalToolFailure(err)
		}
		return internalToolFailure(model.NewAgentError(model.CodeInternal, "load_"+kind, err))
	}
	if errors.Is(err, store.ErrResourceNotFound) || errors.Is(err, fs.ErrNotExist) {
		return detailedToolFailure(code, fmt.Sprintf("%s %q repository entry or body file does not exist", kind, id), map[string]any{"measure": "Choose an existing " + kind + " ID from the repository catalog or restore its missing body file. Do not invent IDs, repeat an unchanged load, or read the PPT outline to locate repository entries."})
	}
	return internalToolFailure(err)
}
