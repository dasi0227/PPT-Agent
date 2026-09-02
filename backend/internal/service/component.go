package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const maxReferencedComponentBytes = 192 << 10

type ComponentService struct {
	root     string
	metadata repositoryMetadataStore
}

type componentMeta struct {
	Name        string
	Description string
}

func NewComponentService(workRoot WorkRoot, stores ...repositoryMetadataStore) *ComponentService {
	return &ComponentService{
		root:     filepath.Join(string(workRoot), "assets", "components"),
		metadata: repositoryMetadataOrMemory(stores),
	}
}

func (s *ComponentService) LoadComponents(context.Context) ([]model.Component, error) {
	components, err := s.List()
	if err != nil {
		return nil, err
	}
	enabled := make([]model.Component, 0, len(components))
	for _, component := range components {
		if !component.Disabled {
			enabled = append(enabled, component)
		}
	}
	return enabled, nil
}

func (s *ComponentService) List() ([]model.Component, error) {
	ids, err := repositoryIDs(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]model.Component, 0, len(ids))
	for _, id := range ids {
		component, readErr := s.Get(id)
		if readErr == nil {
			component.HTML = ""
			out = append(out, component)
		}
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
	raw, path, err := readRepositoryFile(s.root, id, "index.html", maxRepositoryFileSize)
	if err != nil {
		return model.Component{}, repositoryReadError("component", id, err)
	}
	meta, err := parseComponentMeta(raw)
	if err != nil {
		return model.Component{}, repositoryReadError("component", id, err)
	}
	tagKeys, err := s.metadata.ListResourceTagKeys(context.Background(), resourceTypeComponent, id)
	if err != nil {
		return model.Component{}, err
	}
	tags, err := validateComponentTags(tagKeys)
	if err != nil {
		return model.Component{}, repositoryReadError("component", id, err)
	}
	disabled, err := s.metadata.GetResourceDisabled(context.Background(), resourceTypeComponent, id)
	if err != nil {
		return model.Component{}, err
	}
	return model.Component{
		ID: id, Name: meta.Name, Description: meta.Description, Tags: tags,
		HTML: string(raw), LocalPath: path, OpenURL: repositoryOpenURL(path),
		Disabled: disabled,
	}, nil
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
		if readErr != nil {
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
	name, description, err := validateRepositoryMetadata(name, description)
	if err != nil {
		return model.Component{}, err
	}
	original, path, err := readRepositoryFile(s.root, id, "index.html", maxRepositoryFileSize)
	if err != nil {
		return model.Component{}, repositoryReadError("component", id, err)
	}
	_, body, err := parseRepositoryFrontmatter(original, htmlFrontmatterStyle)
	if err != nil {
		return model.Component{}, repositoryReadError("component", id, err)
	}
	raw := make([]string, len(values))
	for index, tag := range values {
		raw[index] = string(tag)
	}
	tags, err := validateComponentTags(raw)
	if err != nil {
		return model.Component{}, err
	}
	raw = raw[:0]
	for _, tag := range tags {
		raw = append(raw, string(tag))
	}
	if err := replaceRepositoryFileMetadata(
		path,
		original,
		body,
		htmlFrontmatterStyle,
		repositoryFileMetadata{Name: name, Description: description},
		func() error {
			return s.metadata.ReplaceResourceTagKeys(context.Background(), resourceTypeComponent, id, raw)
		},
	); err != nil {
		return model.Component{}, err
	}
	return s.Get(id)
}

func validateComponentTags(values []string) ([]model.ComponentTag, error) {
	if len(values) > 2 {
		return nil, ErrRepositoryCorrupt
	}
	tags := make([]model.ComponentTag, len(values))
	seen := make(map[model.ComponentTag]struct{}, len(values))
	for index, value := range values {
		tag := model.ComponentTag(strings.TrimSpace(value))
		if !tag.Valid() {
			return nil, ErrRepositoryCorrupt
		}
		if _, exists := seen[tag]; exists {
			return nil, ErrRepositoryCorrupt
		}
		seen[tag] = struct{}{}
		tags[index] = tag
	}
	return tags, nil
}

func (s *ComponentService) SetDisabled(id string, disabled bool) (model.Component, error) {
	component, err := s.Get(id)
	if err != nil {
		return model.Component{}, err
	}
	if err := s.metadata.SetResourceDisabled(context.Background(), resourceTypeComponent, id, disabled, repositoryStateTimestamp()); err != nil {
		return model.Component{}, err
	}
	component.Disabled = disabled
	return component, nil
}

func (s *ComponentService) Delete(id string) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	if err := deleteRepositoryDirectory(s.root, id); err != nil {
		return err
	}
	return s.metadata.DeleteResourceMetadata(context.Background(), resourceTypeComponent, id)
}

func parseComponentMeta(raw []byte) (componentMeta, error) {
	metadata, _, err := parseRepositoryFrontmatter(raw, htmlFrontmatterStyle)
	if err != nil {
		return componentMeta{}, ErrRepositoryCorrupt
	}
	return componentMeta{Name: metadata.Name, Description: metadata.Description}, nil
}

func componentNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
