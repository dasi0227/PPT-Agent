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
	if strings.TrimSpace(os.Getenv("LLM_CONFIG_PATH")) == "" {
		t.Setenv("LLM_CONFIG_PATH", findRepoConfig(t))
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	profile, ok := deepSeekProfile(cfg.LLM.Profiles)
	if !ok {
		t.Skip("no DeepSeek profile configured")
	}
	adapter := llm.NewDeepSeekAdapter(llm.DeepSeekConfig{
		APIKey: profile.Key, BaseURL: profile.URL, Model: profile.Model,
		Timeout: 90 * time.Second,
	})
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
				model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck},
				nil,
			),
		},
		{
			name: "execute-ppt",
			tools: disclosedToolsForLiveTest(
				PhaseExecuting, model.ModeExecute,
				model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck},
				&Plan{ID: "plan_live", Revision: 1, Status: PlanActive},
			),
		},
		{
			name: "execute-spec",
			tools: disclosedToolsForLiveTest(
				PhaseExecuting, model.ModeExecute,
				model.RunScope{Artifact: model.ArtifactSpec, Level: model.ScopeDeck},
				&Plan{ID: "plan_live", Revision: 1, Status: PlanActive},
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
				Reasoning:       llm.ReasoningDisabled,
				MaxOutputTokens: 32,
			})
			if err != nil {
				t.Fatalf("DeepSeek rejected %s tool schemas: %v", test.name, err)
			}
		})
	}
}

func findRepoConfig(t *testing.T) string {
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
		if profile.Provider != llm.ProviderDeepSeek {
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
	pack := testPack(mode, scope.Artifact, scope.Level, false, "live schema validation")
	pack.Command.Scope = scope
	registry := NewToolRegistry()
	_ = (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry)
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
