package workflow

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func visibleResourceHashes(messages []llm.Message) map[string]string {
	out := map[string]string{}
	for _, message := range messages {
		m := message.Metadata
		if m == nil || m.Origin != "runtime" {
			continue
		}
		if m.Kind == "context" {
			out[m.Key] = m.Hash
		}
		for _, resource := range m.Resources {
			out[resource.Key] = resource.Hash
		}
	}
	return out
}

// Remember methods report whether the complete body was already active. Catalog
// entries and conversation history never establish active resource availability.
func (s *ActiveSkillSet) RememberSkill(skill model.RunSkill) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.Skills {
		if old.ID == skill.ID {
			s.Skills[i] = skill
			return old.Content == skill.Content
		}
	}
	s.Skills = append(s.Skills, skill)
	return false
}

func (s *ActiveSkillSet) RememberComponent(component model.RunComponent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.Components {
		if old.ID == component.ID {
			s.Components[i] = component
			return old.HTML == component.HTML
		}
	}
	s.Components = append(s.Components, component)
	return false
}

func (s *ActiveSkillSet) Snapshot() ([]model.RunSkill, []model.RunComponent) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.RunSkill{}, s.Skills...), append([]model.RunComponent{}, s.Components...)
}

// A completed receipt can be newer than the restored checkpoint. Rehydrate its
// exact body without rereading a repository that may have changed since loading.
func (state *RunState) restoreResourceActivation(receipt persistedToolResult) {
	if !receipt.Result.OK || !receipt.Result.Applied || receipt.ActiveSkill == nil && receipt.ActiveComponent == nil {
		return
	}
	if state.activeSkills == nil {
		state.activeSkills = &ActiveSkillSet{}
	}
	if skill := receipt.ActiveSkill; skill != nil {
		state.activeSkills.RememberSkill(*skill)
	}
	if component := receipt.ActiveComponent; component != nil {
		state.activeSkills.RememberComponent(*component)
	}
}
