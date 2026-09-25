package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const (
	maxSkillFileBytes       = 256 << 10
	maxDynamicSkillsPerCall = 8
	maxDynamicSkillBytes    = 192 << 10
)

type SkillService struct{ resources *ResourceService }

func (s *SkillService) LoadSkills(context.Context) ([]model.RepositorySkill, error) {
	values, err := s.List()
	if err != nil {
		return nil, err
	}
	out := []model.RepositorySkill{}
	for _, v := range values {
		if !v.Disabled && v.ContentState == "ready" {
			out = append(out, v)
		}
	}
	return out, nil
}

func NewSkillService(workRoot WorkRoot, st ResourceStore) *SkillService {
	return &SkillService{resources: NewResourceService(workRoot, st)}
}

func (s *SkillService) List() ([]model.RepositorySkill, error) {
	rows, err := s.resources.store.ListResources(context.Background(), "skill")
	if err != nil {
		return nil, err
	}
	out := make([]model.RepositorySkill, 0, len(rows))
	for _, r := range rows {
		value, err := s.Get(r.ID)
		if err != nil {
			return nil, err
		}
		value.Content = ""
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
		if readErr != nil || skill.ContentState != "ready" {
			if readErr == nil {
				readErr = ErrRepositoryCorrupt
			}
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
	if _, err := s.resources.Patch(context.Background(), "skill", id, ResourcePatch{Disabled: &disabled}); err != nil {
		return model.RepositorySkill{}, err
	}
	return s.Get(id)
}

func (s *SkillService) UpdateMetadata(id, name, description string, values []model.SkillTag) (model.RepositorySkill, error) {
	tags := make([]string, 0, len(values))
	for _, tag := range values {
		tags = append(tags, string(tag))
	}
	if _, err := s.resources.Patch(context.Background(), "skill", id, ResourcePatch{Name: &name, Description: &description, Tags: &tags}); err != nil {
		return model.RepositorySkill{}, err
	}
	return s.Get(id)
}

func (s *SkillService) Delete(id string) error {
	return s.resources.Delete(context.Background(), "skill", id)
}

func (s *SkillService) read(id string) (model.RepositorySkill, error) {
	r, raw, path, state, err := s.resources.Inspect(context.Background(), "skill", id)
	if err != nil {
		return model.RepositorySkill{}, err
	}
	tags := make([]model.SkillTag, 0, len(r.Tags))
	for _, tag := range r.Tags {
		tags = append(tags, model.SkillTag(tag))
	}
	result := model.RepositorySkill{ResourceContentState: state, ID: id, Name: r.Name, Description: r.Description, Tags: tags, Disabled: r.Disabled, Content: string(raw), LocalPath: path, OpenURL: repositoryOpenURL(path)}
	return result, nil
}
