package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const (
	maxSkillFileBytes       = maxRepositoryFileSize
	maxDynamicSkillsPerCall = 8
	maxDynamicSkillBytes    = 192 << 10
)

var skillIDPattern = repositoryIDPattern

type SkillService struct {
	root     string
	metadata repositoryMetadataStore
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func NewSkillService(workRoot WorkRoot, stores ...repositoryMetadataStore) *SkillService {
	return &SkillService{
		root:     filepath.Join(string(workRoot), "assets", "skills"),
		metadata: repositoryMetadataOrMemory(stores),
	}
}

func (s *SkillService) List() ([]model.RepositorySkill, error) {
	ids, err := repositoryIDs(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]model.RepositorySkill, 0, len(ids))
	for _, id := range ids {
		skill, readErr := s.read(id)
		if readErr == nil {
			skill.Content = ""
			out = append(out, skill)
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

func (s *SkillService) Get(id string) (model.RepositorySkill, error) {
	skill, err := s.read(id)
	if err != nil {
		return model.RepositorySkill{}, err
	}
	return skill, nil
}

func (s *SkillService) Resolve(ids []string) ([]model.RunSkill, error) {
	return s.resolve(ids, model.MaxRunSkills, 0)
}

func (s *SkillService) ResolveDynamic(ids []string) ([]model.RunSkill, error) {
	return s.resolve(ids, maxDynamicSkillsPerCall, maxDynamicSkillBytes)
}

func (s *SkillService) resolve(ids []string, maxCount, maxBytes int) ([]model.RunSkill, error) {
	if len(ids) > maxCount {
		return nil, model.NewAgentError("SKILL_SELECTION_INVALID", "load_skill", fmt.Errorf("at most %d skills may be selected", maxCount))
	}
	seen := map[string]bool{}
	out := make([]model.RunSkill, 0, len(ids))
	total := 0
	for _, id := range ids {
		if !validRepositoryID(id) || seen[id] {
			return nil, model.NewAgentError("SKILL_SELECTION_INVALID", "load_skill", errors.New("skill ids must be valid and unique"))
		}
		skill, readErr := s.read(id)
		if readErr != nil {
			return nil, model.NewAgentError("SKILL_NOT_FOUND", "load_skill", readErr)
		}
		if skill.Disabled {
			return nil, model.NewAgentError("SKILL_DISABLED", "load_skill", errors.New("disabled skills cannot be loaded"))
		}
		total += len(skill.Content)
		if maxBytes > 0 && total > maxBytes {
			return nil, model.NewAgentError("CONTEXT_BUDGET_EXCEEDED", "load_skill", errors.New("skill content exceeds the dynamic context budget"))
		}
		seen[id] = true
		out = append(out, model.RunSkill{
			ID: skill.ID, Name: skill.Name, Description: skill.Description, Content: skill.Content,
			LocalPath: skill.LocalPath, OpenURL: skill.OpenURL,
		})
	}
	return out, nil
}

func (s *SkillService) SetDisabled(id string, disabled bool) (model.RepositorySkill, error) {
	skill, err := s.read(id)
	if err != nil {
		return model.RepositorySkill{}, err
	}
	if err := s.metadata.SetResourceDisabled(context.Background(), resourceTypeSkill, id, disabled, repositoryStateTimestamp()); err != nil {
		return model.RepositorySkill{}, err
	}
	skill.Disabled = disabled
	return skill, nil
}

func (s *SkillService) UpdateMetadata(id, name, description string, values []model.SkillTag) (model.RepositorySkill, error) {
	name, description, err := validateRepositoryMetadata(name, description)
	if err != nil {
		return model.RepositorySkill{}, err
	}
	original, path, err := readRepositoryFile(s.root, id, "SKILL.md", maxSkillFileBytes)
	if err != nil {
		return model.RepositorySkill{}, repositoryReadError("skill", id, err)
	}
	_, body, err := parseRepositoryFrontmatter(original, mdFrontmatterStyle)
	if err != nil {
		return model.RepositorySkill{}, repositoryReadError("skill", id, err)
	}
	if strings.TrimSpace(body) == "" {
		return model.RepositorySkill{}, repositoryReadError("skill", id, fmt.Errorf("%w: skill body is required", ErrRepositoryCorrupt))
	}
	raw := make([]string, len(values))
	for index, tag := range values {
		raw[index] = string(tag)
	}
	tags, err := validateSkillTags(raw)
	if err != nil {
		return model.RepositorySkill{}, err
	}
	raw = raw[:0]
	for _, tag := range tags {
		raw = append(raw, string(tag))
	}
	if err := replaceRepositoryFileMetadata(
		path,
		original,
		body,
		mdFrontmatterStyle,
		repositoryFileMetadata{Name: name, Description: description},
		func() error {
			return s.metadata.ReplaceResourceTagKeys(context.Background(), resourceTypeSkill, id, raw)
		},
	); err != nil {
		return model.RepositorySkill{}, err
	}
	return s.Get(id)
}

func validateSkillTags(values []string) ([]model.SkillTag, error) {
	if len(values) > 2 {
		return nil, ErrRepositoryCorrupt
	}
	tags := make([]model.SkillTag, len(values))
	seen := make(map[model.SkillTag]struct{}, len(values))
	for index, value := range values {
		tag := model.SkillTag(strings.TrimSpace(value))
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

func (s *SkillService) Delete(id string) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	if err := deleteRepositoryDirectory(s.root, id); err != nil {
		return err
	}
	return s.metadata.DeleteResourceMetadata(context.Background(), resourceTypeSkill, id)
}

func (s *SkillService) read(id string) (model.RepositorySkill, error) {
	raw, path, err := readRepositoryFile(s.root, id, "SKILL.md", maxSkillFileBytes)
	if err != nil {
		return model.RepositorySkill{}, repositoryReadError("skill", id, err)
	}
	meta, body, err := parseSkillMarkdown(raw)
	if err != nil {
		return model.RepositorySkill{}, repositoryReadError("skill", id, err)
	}
	tagKeys, err := s.metadata.ListResourceTagKeys(context.Background(), resourceTypeSkill, id)
	if err != nil {
		return model.RepositorySkill{}, err
	}
	tags, err := validateSkillTags(tagKeys)
	if err != nil {
		return model.RepositorySkill{}, repositoryReadError("skill", id, err)
	}
	disabled, err := s.metadata.GetResourceDisabled(context.Background(), resourceTypeSkill, id)
	if err != nil {
		return model.RepositorySkill{}, err
	}
	return model.RepositorySkill{
		ID: id, Name: meta.Name, Description: meta.Description, Tags: tags, Content: body,
		Disabled:  disabled,
		LocalPath: path, OpenURL: repositoryOpenURL(path),
	}, nil
}

func parseSkillMarkdown(raw []byte) (skillFrontmatter, string, error) {
	metadata, body, err := parseRepositoryFrontmatter(raw, mdFrontmatterStyle)
	if err != nil {
		return skillFrontmatter{}, "", err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return skillFrontmatter{}, "", fmt.Errorf("%w: skill body is required", ErrRepositoryCorrupt)
	}
	return skillFrontmatter{Name: metadata.Name, Description: metadata.Description}, body, nil
}
