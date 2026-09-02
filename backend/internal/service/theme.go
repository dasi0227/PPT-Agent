package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ThemeService struct {
	root     string
	metadata repositoryMetadataStore
}

func NewThemeService(workRoot WorkRoot, stores ...repositoryMetadataStore) *ThemeService {
	return &ThemeService{
		root:     filepath.Join(string(workRoot), "assets", "themes"),
		metadata: repositoryMetadataOrMemory(stores),
	}
}

func (s *ThemeService) List() ([]model.Theme, error) {
	ids, err := repositoryIDs(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]model.Theme, 0, len(ids))
	for _, id := range ids {
		theme, readErr := s.Get(id)
		if readErr == nil {
			theme.CSS = ""
			out = append(out, theme)
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

func (s *ThemeService) Get(id string) (model.Theme, error) {
	cssRaw, cssPath, err := readRepositoryFile(s.root, id, "theme.css", maxRepositoryFileSize)
	if err != nil {
		return model.Theme{}, repositoryReadError("theme", id, err)
	}
	if err := validateThemeTokens(cssRaw); err != nil {
		return model.Theme{}, repositoryReadError("theme", id, err)
	}
	metadata, _, err := parseRepositoryFrontmatter(cssRaw, cssFrontmatterStyle)
	if err != nil {
		return model.Theme{}, repositoryReadError("theme", id, err)
	}
	tagKeys, err := s.metadata.ListResourceTagKeys(context.Background(), resourceTypeTheme, id)
	if err != nil {
		return model.Theme{}, err
	}
	tags, err := validateThemeTags(tagKeys)
	if err != nil {
		return model.Theme{}, repositoryReadError("theme", id, err)
	}
	return model.Theme{
		ID: id, Name: metadata.Name, Description: metadata.Description,
		Tags: tags,
		CSS:  string(cssRaw), CSSURL: "/api/v1/themes/" + id + "/css",
		LocalPath: cssPath, OpenURL: repositoryOpenURL(cssPath),
	}, nil
}

func (s *ThemeService) UpdateMetadata(id, name, description string, values []model.ThemeTag) (model.Theme, error) {
	name, description, err := validateRepositoryMetadata(name, description)
	if err != nil {
		return model.Theme{}, err
	}
	original, path, err := readRepositoryFile(s.root, id, "theme.css", maxRepositoryFileSize)
	if err != nil {
		return model.Theme{}, repositoryReadError("theme", id, err)
	}
	_, body, err := parseRepositoryFrontmatter(original, cssFrontmatterStyle)
	if err != nil {
		return model.Theme{}, repositoryReadError("theme", id, err)
	}
	if err := validateThemeTokens(original); err != nil {
		return model.Theme{}, repositoryReadError("theme", id, err)
	}
	raw := make([]string, len(values))
	for index, tag := range values {
		raw[index] = string(tag)
	}
	tags, err := validateThemeTags(raw)
	if err != nil {
		return model.Theme{}, err
	}
	values = tags
	raw = raw[:0]
	for _, tag := range values {
		raw = append(raw, string(tag))
	}
	if err := replaceRepositoryFileMetadata(
		path,
		original,
		body,
		cssFrontmatterStyle,
		repositoryFileMetadata{Name: name, Description: description},
		func() error {
			return s.metadata.ReplaceResourceTagKeys(context.Background(), resourceTypeTheme, id, raw)
		},
	); err != nil {
		return model.Theme{}, err
	}
	return s.Get(id)
}

func validateThemeTags(values []string) ([]model.ThemeTag, error) {
	if len(values) > 2 {
		return nil, ErrRepositoryCorrupt
	}
	tags := make([]model.ThemeTag, len(values))
	seen := make(map[model.ThemeTag]struct{}, len(values))
	for index, value := range values {
		tag := model.ThemeTag(strings.TrimSpace(value))
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

func (s *ThemeService) CSS(id string) ([]byte, error) {
	raw, _, err := readRepositoryFile(s.root, id, "theme.css", maxRepositoryFileSize)
	if err != nil {
		return nil, repositoryReadError("theme", id, err)
	}
	if err := validateThemeTokens(raw); err != nil {
		return nil, repositoryReadError("theme", id, err)
	}
	return raw, nil
}

func (s *ThemeService) Exists(id string) bool {
	_, err := s.Get(id)
	return err == nil
}

func (s *ThemeService) Delete(id string) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	if err := deleteRepositoryDirectory(s.root, id); err != nil {
		return err
	}
	return s.metadata.DeleteResourceMetadata(context.Background(), resourceTypeTheme, id)
}

func themeNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func validateThemeTokens(css []byte) error {
	if missing := designsystem.LintTokens(css); len(missing) > 0 {
		return fmt.Errorf("%w: missing required theme tokens: %s", ErrRepositoryCorrupt, strings.Join(missing, ", "))
	}
	return nil
}
