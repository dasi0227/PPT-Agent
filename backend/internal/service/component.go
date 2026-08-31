package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"golang.org/x/net/html"
)

type ComponentService struct {
	root string
}

type componentMeta struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Tags        []model.ComponentTag `json:"tags"`
}

func NewComponentService(workRoot WorkRoot) *ComponentService {
	return &ComponentService{root: filepath.Join(string(workRoot), "assets", "components")}
}

func (s *ComponentService) LoadComponents(context.Context) ([]model.Component, error) {
	return s.List()
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
	return model.Component{
		ID: id, Name: meta.Name, Description: meta.Description, Tags: meta.Tags,
		HTML: string(raw), LocalPath: path, OpenURL: repositoryOpenURL(path),
	}, nil
}

func (s *ComponentService) Delete(id string) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	return deleteRepositoryDirectory(s.root, id)
}

func parseComponentMeta(raw []byte) (componentMeta, error) {
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return componentMeta{}, ErrRepositoryCorrupt
	}
	var metaRaw string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if metaRaw == "" && node.Type == html.ElementNode && node.Data == "script" {
			id, contentType := "", ""
			for _, attr := range node.Attr {
				switch strings.ToLower(attr.Key) {
				case "id":
					id = attr.Val
				case "type":
					contentType = attr.Val
				}
			}
			if id == "meta" && contentType == "application/json" && node.FirstChild != nil {
				metaRaw = node.FirstChild.Data
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if strings.TrimSpace(metaRaw) == "" {
		return componentMeta{}, ErrRepositoryCorrupt
	}
	decoder := json.NewDecoder(strings.NewReader(metaRaw))
	decoder.DisallowUnknownFields()
	var meta componentMeta
	if err := decoder.Decode(&meta); err != nil {
		return componentMeta{}, ErrRepositoryCorrupt
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return componentMeta{}, ErrRepositoryCorrupt
	}
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)
	if meta.Name == "" || meta.Description == "" {
		return componentMeta{}, ErrRepositoryCorrupt
	}
	seenTags := make(map[model.ComponentTag]struct{}, len(meta.Tags))
	for index := range meta.Tags {
		meta.Tags[index] = model.ComponentTag(strings.TrimSpace(string(meta.Tags[index])))
		if !meta.Tags[index].Valid() {
			return componentMeta{}, ErrRepositoryCorrupt
		}
		if _, exists := seenTags[meta.Tags[index]]; exists {
			return componentMeta{}, ErrRepositoryCorrupt
		}
		seenTags[meta.Tags[index]] = struct{}{}
	}
	return meta, nil
}

func componentNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
