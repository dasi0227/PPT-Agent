package workflow

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func skillBody(skill model.RunSkill) map[string]string {
	return map[string]string{"id": skill.ID, "name": skill.Name, "description": skill.Description, "content": skill.Content}
}

func componentBody(component model.RunComponent) map[string]string {
	return map[string]string{"id": component.ID, "name": component.Name, "description": component.Description, "html": component.HTML}
}

func resourceStamp(key string, body any) llm.ResourceStamp {
	raw, _ := json.Marshal(body)
	return llm.ResourceStamp{Key: key, Hash: hashBytes(raw)}
}

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

func (s *ActiveSkillSet) RememberComponent(component model.RunComponent) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.Components {
		if old.ID == component.ID {
			s.Components[i] = component
			return
		}
	}
	s.Components = append(s.Components, component)
}

// Selected components are sent on first use. After compaction their bodies are
// read on demand; a durable snapshot must not be mistaken for visible context.
func rememberSelectedComponents(state *RunState) {
	visible := visibleResourceHashes(state.messages)
	for _, component := range state.pack.Command.Components {
		stamp := resourceStamp("component/"+component.ID, componentBody(component))
		if visible[stamp.Key] == stamp.Hash {
			state.activeSkills.RememberComponent(component)
		}
	}
}

func (s *ActiveSkillSet) Snapshot() ([]model.RunSkill, []model.RunComponent) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.RunSkill{}, s.Skills...), append([]model.RunComponent{}, s.Components...)
}
