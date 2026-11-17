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
	result := model.Theme{ResourceContentState: state, ID: id, Name: r.Name, Description: r.Description, Tags: tags, Disabled: r.Disabled, CSS: string(raw), LocalPath: path, OpenURL: repositoryOpenURL(path)}
	if state.ContentState == "ready" {
		result.StyleHash = runtimeassets.Hash(raw)
		result.Appearance = runtimeassets.Appearance(id, raw)
	}
	return result, nil
}

func (s *ThemeService) CSS(id string) ([]byte, error) {
	return s.resources.Content(context.Background(), "theme", id)
}

func validateThemeTokens(css []byte) error {
	if missing := designsystem.LintTokens(css); len(missing) > 0 {
		return fmt.Errorf("%w: missing required theme tokens: %s", ErrRepositoryCorrupt, strings.Join(missing, ", "))
	}
	return nil
}
