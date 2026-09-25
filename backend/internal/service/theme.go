package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
)

type ThemeService struct{ resources *ResourceService }

func NewThemeService(workRoot WorkRoot, st ResourceStore) *ThemeService {
	return &ThemeService{resources: NewResourceService(workRoot, st)}
}

func (s *ThemeService) List() ([]model.Theme, error) {
	rows, err := s.resources.store.ListResources(context.Background(), "theme")
	if err != nil {
		return nil, err
	}
	out := make([]model.Theme, 0, len(rows))
	for _, r := range rows {
		value, err := s.Get(r.ID)
		if err != nil {
			return nil, err
		}
		value.CSS = ""
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

func (s *ThemeService) Get(id string) (model.Theme, error) {
	r, raw, path, state, err := s.resources.Inspect(context.Background(), "theme", id)
	if err != nil {
		return model.Theme{}, err
	}
	tags := make([]model.ThemeTag, 0, len(r.Tags))
	for _, tag := range r.Tags {
		tags = append(tags, model.ThemeTag(tag))
	}
	result := model.Theme{ResourceContentState: state, ID: id, Name: r.Name, Description: r.Description, Tags: tags, Disabled: r.Disabled, CSS: string(raw), LocalPath: path, OpenURL: repositoryOpenURL(path), CSSURL: "/api/v1/themes/" + id + "/css"}
	if state.ContentState == "ready" {
		result.StyleHash = runtimeassets.Hash(raw)
		result.Appearance = runtimeassets.Appearance(id, raw)
	}
	return result, nil
}

func (s *ThemeService) SetDisabled(id string, disabled bool) (model.Theme, error) {
	if _, err := s.resources.Patch(context.Background(), "theme", id, ResourcePatch{Disabled: &disabled}); err != nil {
		return model.Theme{}, err
	}
	return s.Get(id)
}

func (s *ThemeService) UpdateMetadata(id, name, description string, values []model.ThemeTag) (model.Theme, error) {
	tags := make([]string, 0, len(values))
	for _, tag := range values {
		tags = append(tags, string(tag))
	}
	if _, err := s.resources.Patch(context.Background(), "theme", id, ResourcePatch{Name: &name, Description: &description, Tags: &tags}); err != nil {
		return model.Theme{}, err
	}
	return s.Get(id)
}

func (s *ThemeService) CSS(id string) ([]byte, error) {
	return s.resources.Content(context.Background(), "theme", id)
}

func (s *ThemeService) Exists(id string) bool {
	value, err := s.Get(id)
	return err == nil && value.ContentState == "ready"
}

func (s *ThemeService) Delete(id string) error {
	return s.resources.Delete(context.Background(), "theme", id)
}

func validateThemeTokens(css []byte) error {
	if missing := designsystem.LintTokens(css); len(missing) > 0 {
		return fmt.Errorf("%w: missing required theme tokens: %s", ErrRepositoryCorrupt, strings.Join(missing, ", "))
	}
	return nil
}
