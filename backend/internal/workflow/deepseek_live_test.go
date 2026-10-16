package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestDeepSeekLiveAcceptsDisclosedToolSchemas(t *testing.T) {
	if os.Getenv("RUN_DEEPSEEK_LIVE") != "1" {
		t.Skip("set RUN_DEEPSEEK_LIVE=1 to validate tool schemas against DeepSeek")
	}
	adapter := configuredDeepSeekAdapter(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	for _, test := range []struct {
		name  string
		tools []ToolSchema
	}{
		{
			name: "plan",
			tools: disclosedToolsForLiveTest(
				PhasePlanning, model.ModePlan,
				model.NewRunScope(model.ScopeAllPages),
				nil,
			),
		},
		{
			name: "execute",
			tools: disclosedToolsForLiveTest(
				PhaseExecuting, model.ModeExecute,
				model.NewRunScope(model.ScopeAllPages),
				&Plan{ApprovalID: "approval-test", ID: "plan_live", Status: PlanActive},
			),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := adapter.Generate(ctx, llm.GenerateRequest{
				Messages: []llm.Message{
					{Role: llm.RoleSystem, Content: llm.TextContent("Validate the supplied function schemas. Do not call any tools.")},
					{Role: llm.RoleUser, Content: llm.TextContent("Reply with OK only.")},
				},
				Tools:           toLLMToolSchemas(test.tools),
				MaxOutputTokens: 32,
			})
			if err != nil {
				t.Fatalf("DeepSeek rejected %s tool schemas: %v", test.name, err)
			}
		})
	}
}

func configuredDeepSeekAdapter(t *testing.T) llm.Provider {
	t.Helper()
	backendConfig := findBackendConfig(t)
	oldWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Dir(backendConfig)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWorkingDir) })
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	profile, ok := deepSeekProfile(cfg.LLM.Profiles)
	if !ok {
		t.Skip("no DeepSeek profile configured")
	}
	registry, err := llm.NewRegistry(profile.Name, []llm.ProfileConfig{{
		Name: profile.Name, Protocol: profile.Protocol,
		BaseURL: profile.BaseURL, Key: profile.Key, Model: profile.Model, Timeout: 90 * time.Second,
	}})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := registry.Resolve(profile.Name)
	if err != nil {
		t.Fatal(err)
	}
	return resolved.Adapter()
}

func findBackendConfig(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("config.yaml was not found")
		}
		dir = parent
	}
}

func deepSeekProfile(profiles []config.LLMProfile) (config.LLMProfile, bool) {
	var fallback config.LLMProfile
	for _, profile := range profiles {
		if config.InferModelProvider(profile.Model) != "deepseek" {
			continue
		}
		if fallback.Name == "" {
			fallback = profile
		}
		if strings.HasPrefix(strings.ToLower(profile.Model), "deepseek-v4-pro") {
			return profile, true
		}
	}
	return fallback, fallback.Name != ""
}

func disclosedToolsForLiveTest(phase RunPhase, mode model.RunMode, scope model.RunScope, plan *Plan) []ToolSchema {
	registry := NewToolRegistry()
	_ = (DefaultDomainToolProvider{}).RegisterDomainTools(registry)
	return append(registry.Disclose(phase, mode, scope), controlSchemas(phase, mode, plan)...)
}

func toLLMToolSchemas(schemas []ToolSchema) []llm.ToolSchema {
	out := make([]llm.ToolSchema, 0, len(schemas))
	for _, schema := range schemas {
		out = append(out, llm.ToolSchema{
			Name: schema.Name, Description: schema.Description, Parameters: schema.Parameters,
		})
	}
	return out
}
