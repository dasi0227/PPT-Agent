package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	maxSkillFileBytes       = maxRepositoryFileSize
	maxDynamicSkillsPerCall = 8
	maxDynamicSkillBytes    = 192 << 10
)

var skillIDPattern = repositoryIDPattern

type SkillService struct {
	root string
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type skillRegistry struct {
	Disabled []string `json:"disabled"`
}

func NewSkillService(workRoot WorkRoot) *SkillService {
	return &SkillService{root: filepath.Join(string(workRoot), "skills")}
}

func (s *SkillService) List() ([]model.RepositorySkill, error) {
	disabled, err := s.readRegistry()
	if err != nil {
		return nil, err
	}
	ids, err := repositoryIDs(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]model.RepositorySkill, 0, len(ids))
	for _, id := range ids {
		skill, readErr := s.read(id)
		if readErr == nil {
			skill.Disabled = disabled[id]
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
	disabled, err := s.readRegistry()
	if err != nil {
		return model.RepositorySkill{}, err
	}
	skill, err := s.read(id)
	if err != nil {
		return model.RepositorySkill{}, err
	}
	skill.Disabled = disabled[id]
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
	disabled, err := s.readRegistry()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]model.RunSkill, 0, len(ids))
	total := 0
	for _, id := range ids {
		if !validRepositoryID(id) || seen[id] {
			return nil, model.NewAgentError("SKILL_SELECTION_INVALID", "load_skill", errors.New("skill ids must be valid and unique"))
		}
		if disabled[id] {
			return nil, model.NewAgentError("SKILL_DISABLED", "load_skill", errors.New("disabled skills cannot be loaded"))
		}
		skill, readErr := s.read(id)
		if readErr != nil {
			return nil, model.NewAgentError("SKILL_NOT_FOUND", "load_skill", readErr)
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
	registry, err := s.readRegistry()
	if err != nil {
		return model.RepositorySkill{}, err
	}
	if disabled {
		registry[id] = true
	} else {
		delete(registry, id)
	}
	values := make([]string, 0, len(registry))
	for value := range registry {
		values = append(values, value)
	}
	sort.Strings(values)
	raw, err := json.MarshalIndent(skillRegistry{Disabled: values}, "", "  ")
	if err != nil {
		return model.RepositorySkill{}, err
	}
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return model.RepositorySkill{}, err
	}
	temp, err := os.CreateTemp(s.root, ".registry-*.json")
	if err != nil {
		return model.RepositorySkill{}, err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err = temp.Write(append(raw, '\n')); err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tempPath, filepath.Join(s.root, "registry.json"))
	}
	if err != nil {
		return model.RepositorySkill{}, err
	}
	skill.Disabled = disabled
	return skill, nil
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
	return model.RepositorySkill{
		ID: id, Name: meta.Name, Description: meta.Description, Content: body,
		LocalPath: path, OpenURL: repositoryOpenURL(path),
	}, nil
}

func (s *SkillService) readRegistry() (map[string]bool, error) {
	path := filepath.Join(s.root, "registry.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maxRepositoryFileSize {
		return nil, ErrRepositoryCorrupt
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var registry skillRegistry
	if err := decoder.Decode(&registry); err != nil {
		return nil, fmt.Errorf("skill registry: %w", ErrRepositoryCorrupt)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("skill registry: %w", ErrRepositoryCorrupt)
	}
	out := make(map[string]bool, len(registry.Disabled))
	for _, id := range registry.Disabled {
		if !validRepositoryID(id) || out[id] {
			return nil, fmt.Errorf("skill registry: %w", ErrRepositoryCorrupt)
		}
		out[id] = true
	}
	return out, nil
}

func parseSkillMarkdown(raw []byte) (skillFrontmatter, string, error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return skillFrontmatter{}, "", errors.New("skill frontmatter is required")
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return skillFrontmatter{}, "", errors.New("skill frontmatter is not terminated")
	}
	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(text[4:4+end]), &meta); err != nil {
		return skillFrontmatter{}, "", err
	}
	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)
	body := strings.TrimSpace(text[4+end+5:])
	if meta.Name == "" || meta.Description == "" || body == "" {
		return skillFrontmatter{}, "", errors.New("skill name, description, and body are required")
	}
	return meta, body, nil
}
