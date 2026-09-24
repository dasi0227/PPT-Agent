package llm

import (
	"context"
	"testing"
)

type registryFakeProvider struct {
	name  string
	model string
	caps  Capabilities
}

func (p registryFakeProvider) Name() string               { return p.name }
func (p registryFakeProvider) Model() string              { return p.model }
func (p registryFakeProvider) Capabilities() Capabilities { return p.caps }
func (p registryFakeProvider) Generate(context.Context, GenerateRequest) (GenerateResponse, error) {
	return GenerateResponse{}, nil
}

func TestRegistrySupportsMultipleProfilesForOneProvider(t *testing.T) {
	registry, err := NewRegistry("Kimi Vision", []ProfileConfig{
		{Name: "Kimi Vision", Protocol: "anthropic", BaseURL: "https://api.moonshot.cn/anthropic/v1", Model: "kimi-k3", Key: "one"},
		{Name: "Kimi Text", Protocol: "anthropic", BaseURL: "https://api.moonshot.cn/anthropic/v1", Model: "kimi-future-model", Key: "two"},
	})
	if err != nil {
		t.Fatal(err)
	}
	vision, err := registry.Resolve("")
	if err != nil || !vision.Capabilities().Vision || vision.Name() != "Kimi Vision" {
		t.Fatalf("default profile resolution failed: err=%v", err)
	}
	text, err := registry.Resolve("Kimi Text")
	if err != nil || !text.Capabilities().Vision || !text.Capabilities().ToolCalls {
		t.Fatalf("same-provider model capabilities are wrong: err=%v", err)
	}
}

func TestRegistrySelectsProtocolIndependentlyOfBrand(t *testing.T) {
	r, err := NewRegistry("Responses", []ProfileConfig{
		{Name: "Responses", Protocol: ProtocolResponses, BaseURL: "https://gateway.example/one/v1/", Model: "moonshot/kimi-k3", Key: "one"},
		{Name: "Messages", Protocol: ProtocolAnthropic, BaseURL: "https://gateway.example/two/v1", Model: "moonshot/kimi-k3", Key: "two"},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := r.Resolve("Responses")
	second, _ := r.Resolve("Messages")
	if _, ok := first.Adapter().(*ResponsesAdapter); !ok {
		t.Fatal("brand overrode Responses selection")
	}
	if _, ok := second.Adapter().(*AnthropicAdapter); !ok {
		t.Fatal("brand overrode Anthropic selection")
	}
	if first.URL() != "https://gateway.example/one/v1" || first.Adapter().Name() != "kimi" || second.Adapter().Name() != "kimi" {
		t.Fatal("endpoint or branding was lost")
	}
	public := r.Public()
	if public.Profiles[0].Provider != "kimi" || public.Profiles[0].Protocol != ProtocolResponses || public.Profiles[1].Protocol != ProtocolAnthropic {
		t.Fatal("public model identity does not match the configuration")
	}
}

func TestRegistryUsesProductCapabilitiesForEveryConfiguredModel(t *testing.T) {
	registry, err := NewRegistry("Unknown", []ProfileConfig{{
		Name: "Unknown", Protocol: "responses", BaseURL: "https://api.openai.com/v1",
		Model: "future-unregistered-model", Key: "secret",
	}})
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.Resolve("Unknown")
	caps := profile.Capabilities()
	if !caps.Vision || !caps.ToolCalls || !caps.MultipleToolCalls || caps.ContextWindowTokens != 200_000 {
		t.Fatalf("configured model did not receive the product contract: %+v", caps)
	}
}

func TestPublicRegistryProjectionContainsOnlySafeFields(t *testing.T) {
	provider := registryFakeProvider{
		name: "kimi", model: "kimi-k3",
		caps: Capabilities{
			Vision: true, ToolCalls: true, MultipleToolCalls: true,
			ImageInputMIMEs: []string{"image/png"}, MaxImageBytes: 123,
		},
	}
	registry, err := NewRegistryWithProfiles("Safe Name", []Profile{
		NewTestProfile("Safe Name", "https://private-gateway.invalid/v1", provider),
	})
	if err != nil {
		t.Fatal(err)
	}
	public := registry.Public()
	if public.Default != "Safe Name" || len(public.Profiles) != 1 {
		t.Fatalf("safe projection is incomplete: %+v", public)
	}
	got := public.Profiles[0]
	if got.Name != "Safe Name" || got.Model != "kimi-k3" ||
		!got.Capabilities.Vision || !got.Capabilities.ToolCalls ||
		!got.Capabilities.MultipleToolCalls || got.Capabilities.ContextWindowTokens != 0 {
		t.Fatalf("safe projection is wrong: %+v", got)
	}
}

func TestConfiguredProvidersUseFixedContextWindow(t *testing.T) {
	cases := []struct {
		provider string
		model    string
	}{
		{ProviderDeepSeek, "deepseek-chat"},
		{ProviderKimi, "kimi-k3"},
		{ProviderOpenAI, "gpt-5"},
	}
	for _, tc := range cases {
		registry, err := NewRegistry("Profile", []ProfileConfig{{
			Name: "Profile", Protocol: ProtocolResponses, BaseURL: "https://gateway.example/v1", Model: tc.model, Key: "secret",
		}})
		if err != nil {
			t.Fatal(err)
		}
		profile, _ := registry.Resolve("")
		if got := profile.Capabilities().ContextWindowTokens; got != 200_000 {
			t.Fatalf("%s/%s context window=%d", tc.provider, tc.model, got)
		}
	}
}
