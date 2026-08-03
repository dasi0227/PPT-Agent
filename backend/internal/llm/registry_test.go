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
		{Name: "Kimi Vision", Provider: "kimi", URL: "https://api.moonshot.cn/v1", Model: "kimi-k3", Key: "one"},
		{Name: "Kimi Text", Provider: "kimi", URL: "https://api.moonshot.cn/v1", Model: "kimi-k2", Key: "two"},
	})
	if err != nil {
		t.Fatal(err)
	}
	vision, err := registry.Resolve("")
	if err != nil || !vision.Capabilities().Vision || vision.Name() != "Kimi Vision" {
		t.Fatalf("default profile resolution failed: err=%v", err)
	}
	text, err := registry.Resolve("Kimi Text")
	if err != nil || text.Capabilities().Vision || !text.Capabilities().ToolCalls {
		t.Fatalf("same-provider model capabilities are wrong: err=%v", err)
	}
}

func TestRegistryUnknownModelFailsClosed(t *testing.T) {
	registry, err := NewRegistry("Unknown", []ProfileConfig{{
		Name: "Unknown", Provider: "openai", URL: "https://api.openai.com/v1",
		Model: "future-unregistered-model", Key: "secret",
	}})
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.Resolve("Unknown")
	caps := profile.Capabilities()
	if caps.Vision || caps.ToolCalls || caps.MultipleToolCalls || caps.Reasoning {
		t.Fatalf("unknown model capabilities were invented: %+v", caps)
	}
}

func TestPublicRegistryProjectionContainsOnlySafeFields(t *testing.T) {
	provider := registryFakeProvider{
		name: "kimi", model: "kimi-k3",
		caps: Capabilities{
			Vision: true, ToolCalls: true, MultipleToolCalls: true,
			Reasoning: true, RequiresReasoningReplay: true,
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
		!got.Capabilities.MultipleToolCalls {
		t.Fatalf("safe projection is wrong: %+v", got)
	}
}
