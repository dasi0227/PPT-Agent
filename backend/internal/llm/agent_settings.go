package llm

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/google/uuid"
)

type AgentSettings struct {
	Revision string `json:"revision"`
	config.AgentConfig
}

type AgentSettingsEdit struct {
	Revision                string `json:"revision"`
	ShowToolFailures        *bool  `json:"show_tool_failures"`
	RequireResourceApproval *bool  `json:"require_resource_approval"`
}

func (r *Registry) AgentSettings() (AgentSettings, error) {
	if r == nil || r.manager == nil {
		return AgentSettings{}, settingsError("SETTINGS_READ_FAILED", "智能体设置暂时不可用。")
	}
	m := r.manager
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.publicAgent(), nil
}

func (m *ModelConfigManager) publicAgent() AgentSettings {
	return AgentSettings{Revision: m.agentRevision, AgentConfig: m.fileConfig.Agent}
}

func (r *Registry) SaveAgentSettings(edit AgentSettingsEdit) (AgentSettings, error) {
	if r == nil || r.manager == nil {
		return AgentSettings{}, settingsError("SETTINGS_WRITE_FAILED", "智能体设置暂时不可用。")
	}
	if edit.ShowToolFailures == nil || edit.RequireResourceApproval == nil || edit.Revision == "" {
		return AgentSettings{}, settingsError("SETTINGS_INVALID", "智能体设置格式无效。")
	}
	m := r.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if edit.Revision != m.agentRevision {
		return AgentSettings{}, settingsError("SETTINGS_REVISION_CONFLICT", "智能体设置已在其他页面更新，请刷新后重试。")
	}
	next := m.fileConfig
	next.Agent = config.AgentConfig{ShowToolFailures: *edit.ShowToolFailures, RequireResourceApproval: *edit.RequireResourceApproval}
	if err := m.writeFileConfig(next); err != nil {
		return AgentSettings{}, err
	}
	m.agentRevision = uuid.NewString()
	return m.publicAgent(), nil
}
