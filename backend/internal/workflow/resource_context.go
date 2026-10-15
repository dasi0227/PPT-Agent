package workflow

import (
	"encoding/json"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// Rebuild availability from the conversation that will actually be sent. A
// checkpoint hash, summary or detached status receipt is not source content.
func visibleResourceHashes(messages []llm.Message) map[string]string {
	return availableResourceHashes(messages, nil)
}

func availableResourceHashes(messages []llm.Message, current map[string]string) map[string]string {
	out := map[string]string{}
	known := map[llm.ResourceStamp]bool{}
	calls := map[string]llm.ToolCall{}
	for _, message := range messages {
		if message.Role == llm.RoleAssistant {
			for _, call := range message.ToolCalls {
				calls[call.ID] = call
			}
		}
		m := message.Metadata
		if message.Role != llm.RoleTool || m == nil || m.Origin != "runtime" || m.Kind != "resource" {
			continue
		}
		var reply struct {
			Status          string          `json:"status"`
			Decision        string          `json:"decision"`
			Content         json.RawMessage `json:"content"`
			Stdout          *string         `json:"stdout"`
			OutputTruncated bool            `json:"output_truncated"`
		}
		if json.Unmarshal([]byte(message.Text()), &reply) != nil || reply.Status != "success" || reply.Decision == "refuse" {
			continue
		}
		var body []byte
		if len(reply.Content) > 0 && string(reply.Content) != "null" {
			body = reply.Content
			var source string
			if json.Unmarshal(reply.Content, &source) == nil {
				body = []byte(source)
			}
		} else if reply.Stdout != nil && !reply.OutputTruncated {
			body = []byte(*reply.Stdout)
		}
		call, paired := calls[message.ToolCallID]
		derived := paired && m.ResourceBasis != nil && m.ResourceBasis.Hash != "" &&
			known[*m.ResourceBasis] && m.ResourceCallHash == resourceCallHash(call.Name, call.Args) &&
			(call.Name == "edit_html" || call.Name == "run_command")
		for _, resource := range m.Resources {
			if resource.Hash == "" {
				continue
			}
			complete := body != nil && observedResourceHash(resource.Key, body) == resource.Hash
			// edit_html returns only status; its complete input is still visible
			// in the paired assistant call, and saving does not rewrite it.
			if paired && call.Name == "edit_html" && resource.Key == "ppt/"+resourceForTool(call.Name, stringValue(call.Args["slide_id"])).Key() {
				if source, ok := call.Args["content"].(string); ok && spec.ContentHash([]byte(source)) == resource.Hash {
					complete = true
				}
			}
			if complete || derived {
				out[resource.Key] = resource.Hash
				known[resource] = true
			}
		}
	}
	// An external reversion can restore an earlier version whose complete
	// content is still in context. It is just as usable as the latest receipt.
	for key, hash := range current {
		if known[llm.ResourceStamp{Key: key, Hash: hash}] {
			out[key] = hash
		}
	}
	return out
}

func observedResourceHash(key string, raw []byte) string {
	if strings.HasPrefix(key, "file/") || key == "ppt/deck:outline" || strings.HasPrefix(key, "ppt/slide:") && strings.HasSuffix(key, ":html") {
		return spec.ContentHash(raw)
	}
	if key == "ppt/deck:manifest" || key == "ppt/deck:design" || strings.HasPrefix(key, "ppt/slide:") && strings.HasSuffix(key, ":spec") {
		return spec.ResourceBytesHash(raw)
	}
	return ""
}

func resourceCallHash(name string, args map[string]any) string {
	raw, err := json.Marshal(struct {
		Name string
		Args map[string]any
	}{name, args})
	if err != nil {
		return ""
	}
	return spec.ContentHash(raw)
}

// Remember methods retain one complete body per resource identity.
func (s *ActiveSkillSet) RememberSkill(skill model.RunSkill) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.Skills {
		if old.ID == skill.ID {
			s.Skills[i] = skill
			return
		}
	}
	s.Skills = append(s.Skills, skill)
}

func (s *ActiveSkillSet) RememberComponent(component model.RunComponent) {
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
