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
	root     string
	metadata repositoryMetadataStore
}

type componentMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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

func (s *ComponentService) SetTags(id string, values []model.ComponentTag) (model.Component, error) {
	if _, err := s.Get(id); err != nil {
		return model.Component{}, err
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
	if err := s.metadata.ReplaceResourceTagKeys(context.Background(), resourceTypeComponent, id, raw); err != nil {
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
	return meta, nil
}

func componentNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
