package llm

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

const (
	ProviderDeepSeek  = "deepseek"
	ProviderKimi      = "kimi"
	ProviderOpenAI    = "openai"
	ProtocolResponses = config.ProtocolResponses
	ProtocolAnthropic = config.ProtocolAnthropic
)

type ProfileConfig struct {
	Name     string
	Protocol string
	BaseURL  string
	Model    string
	Key      string
	Timeout  time.Duration
}

type Profile struct {
	name     string
	provider string
	protocol string
	model    string
	url      string
	adapter  Provider
}

func (p Profile) Name() string               { return p.name }
func (p Profile) ProviderName() string       { return p.provider }
func (p Profile) Protocol() string           { return p.protocol }
func (p Profile) Model() string              { return p.model }
func (p Profile) URL() string                { return p.url }
func (p Profile) Adapter() Provider          { return p.adapter }
func (p Profile) Capabilities() Capabilities { return p.adapter.Capabilities() }
func (p Profile) String() string {
	return fmt.Sprintf("LLMProfile{Name:%q,Provider:%q,Protocol:%q,Model:%q}", p.name, p.provider, p.protocol, p.model)
}
func (p Profile) GoString() string { return p.String() }

type PublicCapabilities struct {
	Vision              bool `json:"vision"`
	ToolCalls           bool `json:"tool_calls"`
	MultipleToolCalls   bool `json:"multiple_tool_calls"`
	ContextWindowTokens int  `json:"context_window_tokens"`
}

type PublicProfile struct {
	Name         string             `json:"name"`
	Model        string             `json:"model"`
	Provider     string             `json:"provider"`
	Protocol     string             `json:"protocol"`
	Capabilities PublicCapabilities `json:"capabilities"`
}

type PublicProfiles struct {
	Revision string          `json:"revision"`
	Default  string          `json:"default"`
	Profiles []PublicProfile `json:"profiles"`
}

type Registry struct {
	manager  *ModelConfigManager
	revision string
	routing  RoadConfig

	defaultName string
	profiles    map[string]Profile
	order       []string
}

func (r *Registry) String() string {
	r = r.Snapshot()
	if r == nil {
		return "LLMRegistry<nil>"
	}
	return fmt.Sprintf("LLMRegistry{Default:%q,Profiles:%d}", r.defaultName, len(r.profiles))
}

func (r *Registry) GoString() string { return r.String() }

func NewRegistry(defaultName string, configs []ProfileConfig) (*Registry, error) {
	registered := make([]Profile, 0, len(configs))
	for _, cfg := range configs {
		baseURL := config.NormalizeModelBaseURL(cfg.BaseURL)
		if err := config.ValidateModelAccess(cfg.Protocol, baseURL); err != nil {
			return nil, err
		}
		provider := config.InferModelProvider(cfg.Model)
		adapterConfig := AdapterConfig{Provider: provider, APIKey: cfg.Key, BaseURL: baseURL, Model: cfg.Model, Timeout: cfg.Timeout}
		var adapter Provider
		switch cfg.Protocol {
		case ProtocolResponses:
			adapter = NewResponsesAdapter(adapterConfig)
		case ProtocolAnthropic:
			adapter = NewAnthropicAdapter(adapterConfig)
		}
		registered = append(registered, Profile{
			name: cfg.Name, provider: provider, protocol: cfg.Protocol, model: cfg.Model, url: baseURL, adapter: adapter,
		})
	}
	return NewRegistryWithProfiles(defaultName, registered)
}

// NewRegistryWithProfiles supports deterministic tests without paid provider
// traffic. Production should use NewRegistry so adapter ownership stays fixed.
func NewRegistryWithProfiles(defaultName string, profiles []Profile, roads ...RoadConfig) (*Registry, error) {
	if len(profiles) == 0 {
		return nil, errors.New("LLM registry requires at least one profile")
	}
	registry := &Registry{
		defaultName: defaultName, profiles: make(map[string]Profile, len(profiles)),
		order: make([]string, 0, len(profiles)),
	}
	if len(roads) > 0 {
		registry.routing = roads[0]
	}
	for _, profile := range profiles {
		if profile.adapter == nil {
			return nil, fmt.Errorf("LLM profile %q has no provider adapter", profile.name)
		}
		if _, exists := registry.profiles[profile.name]; exists {
			return nil, fmt.Errorf("duplicate LLM profile name %q", profile.name)
		}
		registry.profiles[profile.name] = profile
		registry.order = append(registry.order, profile.name)
	}
	if _, ok := registry.profiles[defaultName]; !ok {
		return nil, errors.New("LLM default does not match a configured profile")
	}
	return registry, nil
}

func NewTestProfile(name, url string, adapter Provider) Profile {
	if adapter == nil {
		return Profile{name: name, url: url}
	}
	return Profile{
		name: name, provider: adapter.Name(), model: adapter.Model(), url: url, adapter: adapter,
	}
}

func (r *Registry) Default() string {
	r = r.Snapshot()
	if r == nil {
		return ""
	}
	return r.defaultName
}

func (r *Registry) Resolve(name string) (Profile, error) {
	r = r.Snapshot()
	if r == nil {
		return Profile{}, errors.New("MODEL_PROFILE_NOT_FOUND")
	}
	if name == "" {
		name = r.defaultName
	}
	profile, ok := r.profiles[name]
	if !ok {
		return Profile{}, errors.New("MODEL_PROFILE_NOT_FOUND")
	}
	return profile, nil
}

func (r *Registry) Public() PublicProfiles {
	r = r.Snapshot()
	out := PublicProfiles{Default: r.Default(), Profiles: []PublicProfile{}}
	if r == nil {
		return out
	}
	out.Revision = r.revision
	names := append([]string(nil), r.order...)
	if len(names) == 0 {
		for name := range r.profiles {
			names = append(names, name)
		}
		sort.Strings(names)
	}
	for _, name := range names {
		profile := r.profiles[name]
		capabilities := profile.Capabilities()
		out.Profiles = append(out.Profiles, PublicProfile{
			Name: profile.name, Model: profile.model, Provider: profile.provider, Protocol: profile.protocol,
			Capabilities: PublicCapabilities{
				Vision: capabilities.Vision, ToolCalls: capabilities.ToolCalls,
				MultipleToolCalls:   capabilities.MultipleToolCalls,
				ContextWindowTokens: capabilities.ContextWindowTokens,
			},
		})
	}
	return out
}

func productCapabilities() Capabilities {
	return Capabilities{
		Vision:              true,
		ToolCalls:           true,
		MultipleToolCalls:   true,
		ContextWindowTokens: 200_000,
		ImageInputMIMEs:     []string{"image/png", "image/jpeg", "image/webp"},
		MaxImageBytes:       defaultMaxImageBytes,
	}
}

func cloneCapabilities(value Capabilities) Capabilities {
	value.ImageInputMIMEs = append([]string(nil), value.ImageInputMIMEs...)
	return value
}
