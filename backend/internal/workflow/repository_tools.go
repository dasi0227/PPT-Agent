package workflow

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

type ComponentLoader interface {
	Get(string) (model.Component, error)
}

type SkillLoader interface {
	Get(string) (model.RepositorySkill, error)
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
	id := input.Args["id"].(string)
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
	available := input.ActiveSkills.RememberComponent(snapshot)
	return repositoryLoadSuccess(available, LoadedResource{
		Kind: "component", ID: snapshot.ID, Name: snapshot.Name,
		LocalPath: snapshot.LocalPath, OpenURL: snapshot.OpenURL,
	})
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
	id := input.Args["id"].(string)
	skill, err := t.loader.Get(id)
	if err != nil {
		return repositoryLoadFailure("skill", id, err)
	}
	if skill.Disabled {
		return repositoryLoadFailure("skill", id, model.RepositoryDisabledError("skill", id))
	}
	if err := model.RepositoryContentError("skill", id, skill.ResourceContentState); err != nil {
		return repositoryLoadFailure("skill", id, err)
	}
	snapshot := model.RunSkill{ID: skill.ID, Name: skill.Name, Description: skill.Description, Content: skill.Content, LocalPath: skill.LocalPath, OpenURL: skill.OpenURL}
	for _, selected := range input.Context.Command.Skills {
		if selected.ID == id {
			snapshot = selected
			break
		}
	}
	available := input.ActiveSkills.RememberSkill(snapshot)
	return repositoryLoadSuccess(available, LoadedResource{
		Kind: "skill", ID: snapshot.ID, Name: snapshot.Name,
		LocalPath: snapshot.LocalPath, OpenURL: snapshot.OpenURL,
	})
}

func repositoryLoadSuccess(available bool, resource LoadedResource) ToolResult {
	result := SuccessfulToolResult("resource loaded")
	status := "success"
	if available {
		status = "already_available"
	}
	result.Data = map[string]any{"status": status}
	result.LoadedResources = []LoadedResource{resource}
	return result
}

func repositoryLoadFailure(kind, id string, err error) ToolResult {
	var failure *model.AgentError
	if errors.As(err, &failure) {
		if model.IsCommonErrorCode(failure.Code) {
			return internalToolFailure(err)
		}
		return internalToolFailure(model.NewAgentError(model.CodeInternal, "load_"+kind, err))
	}
	if errors.Is(err, store.ErrResourceNotFound) || errors.Is(err, fs.ErrNotExist) {
		return detailedToolFailure(model.CodeNotFound, fmt.Sprintf("%s %q repository entry or body file does not exist", kind, id), map[string]any{"measure": "Choose an existing " + kind + " ID from the repository catalog or restore its missing body file. Do not invent IDs, repeat an unchanged load, or read the PPT outline to locate repository entries."})
	}
	return internalToolFailure(err)
}
