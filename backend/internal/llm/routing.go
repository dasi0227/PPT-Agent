package llm

import (
	"context"
	"errors"
	"fmt"
)

var ErrFallbackActivated = errors.New("model fallback activated; rebuild request")

// RouteState is safe model identity. ConfigHash is internal checkpoint data only.
type RouteState struct {
	Revision     string `json:"revision"`
	ConfigHash   string `json:"config_hash,omitempty"`
	Purpose      string `json:"purpose"`
	Initial      string `json:"initial"`
	Active       string `json:"active"`
	Provider     string `json:"provider"`
	Protocol     string `json:"protocol"`
	Model        string `json:"model"`
	FallbackUsed bool   `json:"fallback_used"`
}
type ModelExecution struct {
	Profile      string `json:"profile"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	FallbackUsed bool   `json:"fallback_used"`
}

func ExecutionOf(provider Provider) ModelExecution {
	if route, ok := provider.(*RoutedProvider); ok {
		state := route.State()
		return ModelExecution{state.Active, state.Provider, state.Model, state.FallbackUsed}
	}
	return ModelExecution{Provider: provider.Name(), Model: provider.Model()}
}

// One RoutedProvider belongs to exactly one sequential operation, never a shared registry.
type RoutedProvider struct {
	snapshot   *Registry
	initial    Profile
	active     Profile
	fallback   *Profile
	purpose    string
	switched   bool
	OnFallback func(context.Context, RouteState) error
}

func (r *Registry) RoutedProfile(purpose, selected string) (Profile, error) {
	r = r.Snapshot()
	if r == nil {
		return Profile{}, errors.New("MODEL_PROFILE_NOT_FOUND")
	}
	first, backup := selected, r.routing.Main.Fallback
	if purpose != "main" {
		value, known := r.routing.Side.Uses()[purpose]
		if !known {
			return Profile{}, errors.New("unknown model purpose")
		}
		first = value
		backup = r.routing.Side.Fallback
		if first == "" {
			first = r.routing.Side.Default
		}
	}
	if first == "" {
		first = r.defaultName
	}
	primary, err := r.Resolve(first)
	if err != nil {
		return Profile{}, err
	}
	route := &RoutedProvider{snapshot: r, initial: primary, active: primary, purpose: purpose}
	if backup != "" && backup != primary.Name() {
		fallback, err := r.Resolve(backup)
		if err != nil {
			return Profile{}, err
		}
		route.fallback = &fallback
	}
	result := primary
	result.adapter = route
	return result, nil
}
func (p *RoutedProvider) Name() string               { return p.active.ProviderName() }
func (p *RoutedProvider) Model() string              { return p.active.Model() }
func (p *RoutedProvider) Capabilities() Capabilities { return p.active.Capabilities() }
func (p *RoutedProvider) State() RouteState {
	return RouteState{Revision: p.snapshot.revision, ConfigHash: p.snapshot.routing.Fingerprint, Purpose: p.purpose, Initial: p.initial.Name(), Active: p.active.Name(), Provider: p.Name(), Protocol: p.active.Protocol(), Model: p.Model(), FallbackUsed: p.switched}
}
func (p *RoutedProvider) Snapshot() *Registry { return p.snapshot }
func (p *RoutedProvider) Restore(state RouteState) error {
	if state.ConfigHash != p.snapshot.routing.Fingerprint || state.Initial != p.initial.Name() || state.Purpose != p.purpose {
		return errors.New("模型配置已改变，无法恢复原任务，请新建一轮。")
	}
	if state.FallbackUsed {
		if p.fallback == nil || state.Active != p.fallback.Name() || state.Provider != p.fallback.ProviderName() || state.Protocol != p.fallback.Protocol() || state.Model != p.fallback.Model() {
			return errors.New("备用模型配置已改变，请新建一轮。")
		}
		p.active = *p.fallback
		p.switched = true
	}
	return nil
}
func (p *RoutedProvider) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	if err := ctx.Err(); err != nil {
		return GenerateResponse{}, err
	}
	if req.Continuation != nil && (req.Continuation.Provider != p.Name() || req.Continuation.Model != p.Model()) {
		req.Continuation = nil
	}
	if err := validateRouteCapabilities(p.active, req, true); err != nil {
		return GenerateResponse{}, err
	}
	response, err := p.active.Adapter().Generate(ctx, req)
	if err == nil || !errors.Is(err, ErrUnavailable) || ctx.Err() != nil || p.switched || p.fallback == nil {
		return response, err
	}
	if err := validateRouteCapabilities(*p.fallback, req, !req.PauseOnFallback); err != nil {
		return GenerateResponse{}, err
	}
	p.active = *p.fallback
	p.switched = true
	if p.OnFallback != nil {
		if err := p.OnFallback(ctx, p.State()); err != nil {
			return GenerateResponse{}, err
		}
	}
	if req.PauseOnFallback {
		return GenerateResponse{}, ErrFallbackActivated
	}
	req.Continuation = nil
	return p.active.Adapter().Generate(ctx, req)
}
func validateRouteCapabilities(profile Profile, req GenerateRequest, checkWindow bool) error {
	caps := profile.Capabilities()
	if checkWindow && caps.ContextWindowTokens > 0 && EstimateRequestTokens(req)+req.MaxOutputTokens >= caps.ContextWindowTokens {
		return fmt.Errorf("%w: request exceeds selected model context window", ErrBadRequest)
	}
	if len(req.Tools) > 0 && !caps.ToolCalls {
		return fmt.Errorf("%w: selected model does not support tools", ErrBadRequest)
	}
	for _, message := range req.Messages {
		for _, part := range message.Content {
			if part.Type == "image" && !caps.Vision {
				return fmt.Errorf("%w: selected model does not support images", ErrBadRequest)
			}
		}
	}
	return nil
}

// Used for independent operations which must resolve a fresh route each time.
type SideProvider struct {
	Registry *Registry
	Purpose  string
}

func (p SideProvider) Capture() (Provider, error) {
	profile, err := p.Registry.RoutedProfile(p.Purpose, "")
	return profile.Adapter(), err
}
func (p SideProvider) Name() string  { return "side-road" }
func (p SideProvider) Model() string { return p.Purpose }
func (p SideProvider) Capabilities() Capabilities {
	profile, err := p.Registry.RoutedProfile(p.Purpose, "")
	if err != nil {
		return Capabilities{}
	}
	return profile.Capabilities()
}
func (p SideProvider) Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error) {
	provider, err := p.Capture()
	if err != nil {
		return GenerateResponse{}, err
	}
	return provider.Generate(ctx, req)
}
