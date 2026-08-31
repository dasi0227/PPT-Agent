package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ThemeService struct {
	root string
}

type themeManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func NewThemeService(workRoot WorkRoot) *ThemeService {
	return &ThemeService{root: filepath.Join(string(workRoot), "assets", "themes")}
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
	manifestRaw, _, err := readRepositoryFile(s.root, id, "manifest.json", maxRepositoryFileSize)
	if err != nil {
		return model.Theme{}, repositoryReadError("theme", id, err)
	}
	cssRaw, cssPath, err := readRepositoryFile(s.root, id, "theme.css", maxRepositoryFileSize)
	if err != nil {
		return model.Theme{}, repositoryReadError("theme", id, err)
	}
	var manifest themeManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return model.Theme{}, repositoryReadError("theme", id, ErrRepositoryCorrupt)
	}
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Description = strings.TrimSpace(manifest.Description)
	if _, description, found := strings.Cut(manifest.Description, "："); found {
		manifest.Description = strings.TrimSpace(description)
	}
	if manifest.Name == "" || manifest.Description == "" {
		return model.Theme{}, repositoryReadError("theme", id, ErrRepositoryCorrupt)
	}
	return model.Theme{
		ID: id, Name: manifest.Name, Description: manifest.Description,
		CSS: string(cssRaw), CSSURL: "/api/v1/themes/" + id + "/css",
		LocalPath: cssPath, OpenURL: repositoryOpenURL(cssPath),
	}, nil
}

func (s *ThemeService) CSS(id string) ([]byte, error) {
	raw, _, err := readRepositoryFile(s.root, id, "theme.css", maxRepositoryFileSize)
	return raw, err
}

func (s *ThemeService) Exists(id string) bool {
	_, err := s.Get(id)
	return err == nil
}

func (s *ThemeService) Delete(id string) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	return deleteRepositoryDirectory(s.root, id)
}

func themeNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
