package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type RoadConfig struct {
	Main        config.MainRoadLLMConfig
	Side        config.SideRoadLLMConfig
	Fingerprint string
}

type SettingsProfile struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Protocol string `json:"protocol"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	HasKey   bool   `json:"has_key"`
}
type ModelSettings struct {
	Revision  string                      `json:"revision"`
	Providers []config.ProviderDefinition `json:"providers"`
	Protocols []string                    `json:"protocols"`
	Profiles  []SettingsProfile           `json:"llm"`
	Main      config.MainRoadLLMConfig    `json:"main_road"`
	Side      config.SideRoadLLMConfig    `json:"side_road"`
}
type ProfileEdit struct {
	PreviousName string  `json:"previous_name,omitempty"`
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	Protocol     string  `json:"protocol"`
	BaseURL      string  `json:"base_url"`
	Model        string  `json:"model"`
	Key          *string `json:"key,omitempty"`
}
type SettingsEdit struct {
	Revision string                   `json:"revision"`
	Profiles []ProfileEdit            `json:"llm"`
	Main     config.MainRoadLLMConfig `json:"main_road"`
	Side     config.SideRoadLLMConfig `json:"side_road"`
}
type SettingsError struct{ Code, Message string }

func (e *SettingsError) Error() string         { return e.Message }
func settingsError(code, message string) error { return &SettingsError{code, message} }

// The live registry delegates to this manager; snapshots never change after publication.
// Neither config nor adapters are returned by the settings HTTP projection.
type ModelConfigManager struct {
	mu       sync.RWMutex
	path     string
	diskHash [32]byte
	config   config.LLMConfig
	current  *Registry
}

func NewConfiguredRegistry(path string, cfg config.LLMConfig) (*Registry, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(absolute)
	if err != nil {
		return nil, errors.New("read model configuration failed")
	}
	config.NormalizeLLMConfig(&cfg)
	snapshot, err := buildSnapshot(cfg)
	if err != nil {
		return nil, err
	}
	return &Registry{manager: &ModelConfigManager{path: absolute, diskHash: sha256.Sum256(raw), config: cfg, current: snapshot}}, nil
}
func buildSnapshot(cfg config.LLMConfig) (*Registry, error) {
	if err := config.ValidateLLMConfig(cfg); err != nil {
		return nil, err
	}
	profiles := make([]ProfileConfig, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		profiles = append(profiles, ProfileConfig{Name: p.Name, Provider: p.Provider, Protocol: p.Protocol, BaseURL: p.BaseURL, Model: p.Model, Key: p.Key})
	}
	r, err := NewRegistry(cfg.MainRoad.Default, profiles)
	if err != nil {
		return nil, err
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, errors.New("encode model configuration failed")
	}
	digest := sha256.Sum256(raw)
	r.revision = uuid.NewString()
	r.routing = RoadConfig{Main: cfg.MainRoad, Side: cfg.SideRoad, Fingerprint: hex.EncodeToString(digest[:])}
	return r, nil
}
func (r *Registry) Snapshot() *Registry {
	if r == nil || r.manager == nil {
		return r
	}
	r.manager.mu.RLock()
	defer r.manager.mu.RUnlock()
	return r.manager.current
}
func (r *Registry) Settings() (ModelSettings, error) {
	if r == nil || r.manager == nil {
		return ModelSettings{}, errors.New("model settings are unavailable")
	}
	m := r.manager
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.public(), nil
}

// ReloadSettings adopts a valid disk configuration without changing existing snapshots.
// Refreshing unchanged content (including comment-only edits) keeps the revision stable.
func (r *Registry) ReloadSettings() (ModelSettings, error) {
	if r == nil || r.manager == nil {
		return ModelSettings{}, errors.New("model settings are unavailable")
	}
	m := r.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, err := os.ReadFile(m.path)
	if err != nil {
		return ModelSettings{}, settingsError("SETTINGS_READ_FAILED", "无法读取配置文件，原设置仍然有效。")
	}
	digest := sha256.Sum256(raw)
	if digest == m.diskHash {
		return m.public(), nil
	}
	cfg, err := config.ParseLLMConfig(raw)
	if err != nil {
		return ModelSettings{}, settingsError("SETTINGS_INVALID", "配置文件无效，原设置仍然有效："+err.Error())
	}
	next, err := buildSnapshot(cfg)
	if err != nil {
		return ModelSettings{}, settingsError("SETTINGS_INVALID", "无法加载配置，原设置仍然有效："+err.Error())
	}
	if next.routing.Fingerprint != m.current.routing.Fingerprint {
		m.config = cfg
		m.current = next
	}
	m.diskHash = digest
	return m.public(), nil
}

func (m *ModelConfigManager) public() ModelSettings {
	out := ModelSettings{Revision: m.current.revision, Providers: config.ModelProviders(), Protocols: []string{ProtocolResponses, ProtocolAnthropic}, Profiles: []SettingsProfile{}, Main: m.config.MainRoad, Side: m.config.SideRoad}
	for _, p := range m.config.Profiles {
		out.Profiles = append(out.Profiles, SettingsProfile{Name: p.Name, Provider: p.Provider, Protocol: p.Protocol, BaseURL: p.BaseURL, Model: p.Model, HasKey: p.Key != ""})
	}
	return out
}
func (r *Registry) SaveSettings(edit SettingsEdit) (ModelSettings, error) {
	if r == nil || r.manager == nil {
		return ModelSettings{}, errors.New("model settings are unavailable")
	}
	m := r.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if edit.Revision != m.current.revision {
		return ModelSettings{}, settingsError("SETTINGS_REVISION_CONFLICT", "设置已在其他页面更新，请重新读取后再保存。")
	}
	raw, err := os.ReadFile(m.path)
	if err != nil {
		return ModelSettings{}, settingsError("SETTINGS_WRITE_FAILED", "无法读取配置文件，原设置仍然有效。")
	}
	if sha256.Sum256(raw) != m.diskHash {
		return ModelSettings{}, settingsError("SETTINGS_FILE_CHANGED", "配置文件已在外部修改，请点击右上角刷新，加载最新设置后再编辑保存。")
	}
	old := map[string]config.LLMProfile{}
	used := map[string]bool{}
	for _, p := range m.config.Profiles {
		old[p.Name] = p
	}
	cfg := config.LLMConfig{MainRoad: edit.Main, SideRoad: edit.Side}
	for i, p := range edit.Profiles {
		previous, exists := old[p.PreviousName]
		if p.PreviousName != "" && (!exists || used[p.PreviousName]) {
			return ModelSettings{}, settingsError("SETTINGS_INVALID", fmt.Sprintf("llm[%d].previous_name 无效或被重复使用。", i))
		}
		used[p.PreviousName] = true
		key := ""
		if p.Key != nil {
			key = *p.Key
		} else if exists && previous.Provider == strings.TrimSpace(p.Provider) && previous.Protocol == strings.TrimSpace(p.Protocol) && previous.BaseURL == config.NormalizeModelBaseURL(p.BaseURL) {
			key = previous.Key
		}
		cfg.Profiles = append(cfg.Profiles, config.LLMProfile{Name: p.Name, Provider: p.Provider, Protocol: p.Protocol, BaseURL: p.BaseURL, Model: p.Model, Key: key})
	}
	config.NormalizeLLMConfig(&cfg)
	next, err := buildSnapshot(cfg)
	if err != nil {
		return ModelSettings{}, settingsError("SETTINGS_INVALID", err.Error())
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return ModelSettings{}, settingsError("SETTINGS_WRITE_FAILED", "无法序列化设置，原设置仍然有效。")
	}
	file, err := os.CreateTemp(filepath.Dir(m.path), ".model-settings-*.yaml")
	if err != nil {
		return ModelSettings{}, settingsError("SETTINGS_WRITE_FAILED", "无法写入配置文件，请检查目录权限。")
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temp, m.path)
	}
	if err != nil {
		return ModelSettings{}, settingsError("SETTINGS_WRITE_FAILED", "配置文件保存失败，原设置仍然有效。")
	}
	m.config = cfg
	m.current = next
	m.diskHash = sha256.Sum256(data)
	return m.public(), nil
}
