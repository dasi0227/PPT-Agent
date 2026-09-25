package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const maxReferencedComponentBytes = 192 << 10

type ComponentService struct{ resources *ResourceService }

func NewComponentService(workRoot WorkRoot, st ResourceStore) *ComponentService {
	return &ComponentService{resources: NewResourceService(workRoot, st)}
}

func (s *ComponentService) LoadComponents(context.Context) ([]model.Component, error) {
	components, err := s.List()
	if err != nil {
		return nil, err
	}
	enabled := make([]model.Component, 0, len(components))
	for _, component := range components {
		if !component.Disabled && component.ContentState == "ready" {
			enabled = append(enabled, component)
		}
	}
	return enabled, nil
}

func (s *ComponentService) List() ([]model.Component, error) {
	rows, err := s.resources.store.ListResources(context.Background(), "component")
	if err != nil {
		return nil, err
	}
	out := make([]model.Component, 0, len(rows))
	for _, r := range rows {
		value, err := s.Get(r.ID)
		if err != nil {
			return nil, err
		}
		value.HTML = ""
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *ComponentService) Get(id string) (model.Component, error) {
	r, raw, path, state, err := s.resources.Inspect(context.Background(), "component", id)
	if err != nil {
		return model.Component{}, err
	}
	tags := make([]model.ComponentTag, 0, len(r.Tags))
	for _, tag := range r.Tags {
		tags = append(tags, model.ComponentTag(tag))
	}
	result := model.Component{ResourceContentState: state, ID: id, Name: r.Name, Description: r.Description, Tags: tags, Disabled: r.Disabled, HTML: string(raw), LocalPath: path, OpenURL: repositoryOpenURL(path)}
	return result, nil
}

func (s *ComponentService) ResolveByNames(names []string) ([]model.RunComponent, error) {
	if len(names) > model.MaxRunComponents {
		return nil, componentResolveError(
			"COMPONENT_SELECTION_INVALID",
			fmt.Errorf("at most %d components may be referenced", model.MaxRunComponents),
			"",
		)
	}

	components, err := s.List()
	if err != nil {
		return nil, err
	}
	enabledByName := make(map[string][]model.Component, len(components))
	disabledByName := make(map[string]bool, len(components))
	for _, component := range components {
		name := strings.TrimSpace(component.Name)
		if component.Disabled {
			disabledByName[name] = true
			continue
		}
		if component.ContentState != "ready" {
			continue
		}
		enabledByName[name] = append(enabledByName[name], component)
	}

	seen := make(map[string]bool, len(names))
	out := make([]model.RunComponent, 0, len(names))
	totalBytes := 0
	for _, requested := range names {
		name := strings.TrimSpace(requested)
		if name == "" {
			return nil, componentResolveError(
				"COMPONENT_SELECTION_INVALID",
				errors.New("component names must be non-empty"),
				name,
			)
		}
		if seen[name] {
			continue
		}
		seen[name] = true

		matches := enabledByName[name]
		switch {
		case len(matches) > 1:
			return nil, componentResolveError(
				"COMPONENT_NAME_AMBIGUOUS",
				fmt.Errorf("multiple enabled components use name %q; rename them in the repository", name),
				name,
			)
		case len(matches) == 0 && disabledByName[name]:
			return nil, componentResolveError(
				"COMPONENT_DISABLED",
				fmt.Errorf("component %q is disabled", name),
				name,
			)
		case len(matches) == 0:
			return nil, componentResolveError(
				"COMPONENT_NOT_FOUND",
				fmt.Errorf("component %q was not found", name),
				name,
			)
		}

		component, readErr := s.Get(matches[0].ID)
		if readErr != nil || component.ContentState != "ready" {
			if readErr == nil {
				readErr = ErrRepositoryCorrupt
			}
			return nil, componentResolveError("COMPONENT_NOT_FOUND", readErr, name)
		}
		totalBytes += len(component.HTML)
		if totalBytes > maxReferencedComponentBytes {
			return nil, componentResolveError(
				"CONTEXT_BUDGET_EXCEEDED",
				errors.New("referenced component HTML exceeds the context budget"),
				name,
			)
		}
		out = append(out, model.RunComponent{
			ID: component.ID, Name: strings.TrimSpace(component.Name),
			Description: component.Description, HTML: component.HTML,
			LocalPath: component.LocalPath, OpenURL: component.OpenURL,
		})
	}
	return out, nil
}

func componentResolveError(code string, cause error, name string) *model.AgentError {
	err := model.NewAgentError(code, "resolve_components", cause)
	if name != "" {
		err.Details["component_name"] = name
	}
	return err
}

func (s *ComponentService) UpdateMetadata(id, name, description string, values []model.ComponentTag) (model.Component, error) {
	tags := make([]string, 0, len(values))
	for _, tag := range values {
		tags = append(tags, string(tag))
	}
	if _, err := s.resources.Patch(context.Background(), "component", id, ResourcePatch{Name: &name, Description: &description, Tags: &tags}); err != nil {
		return model.Component{}, err
	}
	return s.Get(id)
}

func (s *ComponentService) SetDisabled(id string, disabled bool) (model.Component, error) {
	if _, err := s.resources.Patch(context.Background(), "component", id, ResourcePatch{Disabled: &disabled}); err != nil {
		return model.Component{}, err
	}
	return s.Get(id)
}

func (s *ComponentService) Delete(id string) error {
	return s.resources.Delete(context.Background(), "component", id)
}
