package service

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"gopkg.in/yaml.v3"
)

const maxSkillFileBytes = 64 << 10

var skillIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

type SkillService struct {
	root string
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func NewSkillService(workRoot WorkRoot) *SkillService {
	return &SkillService{root: filepath.Join(string(workRoot), "skills")}
}

func (s *SkillService) List() ([]model.RunSkill, error) {
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return []model.RunSkill{}, nil
	}
	if err != nil {
		return nil, err
	}
	skills := make([]model.RunSkill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !skillIDPattern.MatchString(entry.Name()) {
			continue
		}
		skill, err := s.read(entry.Name())
		if err == nil {
			skills = append(skills, skill)
		}
	}
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Name == skills[j].Name {
			return skills[i].ID < skills[j].ID
		}
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

func (s *SkillService) Resolve(ids []string) ([]model.RunSkill, error) {
	if len(ids) > model.MaxRunSkills {
		return nil, model.NewAgentError("SKILL_SELECTION_INVALID", "create_run", fmt.Errorf("at most %d skills may be selected", model.MaxRunSkills))
	}
	if len(ids) == 0 {
		return nil, nil
	}
	available, err := s.List()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.RunSkill, len(available))
	for _, skill := range available {
		byID[skill.ID] = skill
	}
	seen := make(map[string]bool, len(ids))
	selected := make([]model.RunSkill, 0, len(ids))
	for _, id := range ids {
		if !skillIDPattern.MatchString(id) || seen[id] {
			return nil, model.NewAgentError("SKILL_SELECTION_INVALID", "create_run", errors.New("skill ids must be valid and unique"))
		}
		skill, ok := byID[id]
		if !ok {
			agentErr := model.NewAgentError("SKILL_NOT_FOUND", "create_run", nil)
			agentErr.Details["skill_id"] = id
			return nil, agentErr
		}
		seen[id] = true
		selected = append(selected, skill)
	}
	return selected, nil
}

func (s *SkillService) read(id string) (model.RunSkill, error) {
	path := filepath.Join(s.root, id, "SKILL.md")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maxSkillFileBytes {
		return model.RunSkill{}, errors.New("invalid skill file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.RunSkill{}, err
	}
	meta, body, err := parseSkillMarkdown(raw)
	if err != nil {
		return model.RunSkill{}, err
	}
	openURL := (&url.URL{Scheme: "vscode", Host: "file", Path: path}).String()
	return model.RunSkill{
		ID: id, Name: meta.Name, Description: meta.Description, Content: body,
		LocalPath: path, OpenURL: openURL,
	}, nil
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
