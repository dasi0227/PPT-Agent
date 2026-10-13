package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

const maxDynamicSkillsPerCall = 8

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
	return s.resolve(ids, model.MaxRunSkills, false)
}

func (s *SkillService) ResolveDynamic(ids []string) ([]model.RunSkill, error) {
	return s.resolve(ids, maxDynamicSkillsPerCall, true)
}

func (s *SkillService) resolve(ids []string, maxCount int, dynamic bool) ([]model.RunSkill, error) {
	argumentCode := "SKILL_SELECTION_INVALID"
	if dynamic {
		argumentCode = model.CodeToolArgumentInvalid
	}
	if dynamic && len(ids) == 0 {
		return nil, model.NewAgentError(argumentCode, "load_skill", errors.New("at least one skill ID is required"))
	}
	if len(ids) > maxCount {
		return nil, model.NewAgentError(argumentCode, "load_skill", fmt.Errorf("at most %d skills may be selected", maxCount))
	}
	seen := map[string]bool{}
	out := make([]model.RunSkill, 0, len(ids))
	for _, id := range ids {
		if !validRepositoryID(id) || seen[id] {
			return nil, model.NewAgentError(argumentCode, "load_skill", errors.New("skill ids must be valid and unique"))
		}
		skill, readErr := s.read(id)
		if readErr != nil && !errors.Is(readErr, store.ErrResourceNotFound) {
			return nil, model.AsAgentError(readErr, model.CodeInternal, "load_skill")
		}
		if readErr != nil {
			return nil, model.NewAgentError("SKILL_NOT_FOUND", "load_skill", fmt.Errorf("skill %q repository entry does not exist: %w", id, readErr))
		}
		if skill.Disabled {
			if dynamic {
				return nil, model.RepositoryDisabledError("skill", id)
			}
			return nil, model.NewAgentError("SKILL_DISABLED", "load_skill", errors.New("disabled skills cannot be loaded"))
		}
		if err := model.RepositoryContentError("skill", id, skill.ResourceContentState); err != nil {
			return nil, err
		}
		seen[id] = true
		out = append(out, model.RunSkill{
			ID: skill.ID, Name: skill.Name, Description: skill.Description, Content: skill.Content,
			LocalPath: skill.LocalPath, OpenURL: skill.OpenURL,
		})
	}
	return out, nil
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
