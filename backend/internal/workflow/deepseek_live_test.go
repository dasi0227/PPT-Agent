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
				model.NewRunScope(model.ScopeObjectPresentation, model.ScopeAllPages),
				nil,
			),
		},
		{
			name: "execute-ppt",
			tools: disclosedToolsForLiveTest(
				PhaseExecuting, model.ModeExecute,
				model.NewRunScope(model.ScopeObjectPresentation, model.ScopeAllPages),
				&Plan{ID: "plan_live", Revision: 1, Status: PlanActive},
			),
		},
		{
			name: "execute-spec",
			tools: disclosedToolsForLiveTest(
				PhaseExecuting, model.ModeExecute,
				model.NewRunScope(model.ScopeObjectSpec, model.ScopeAllPages),
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

func TestDeepSeekLiveProducesBoundedNextInputSuggestions(t *testing.T) {
	if os.Getenv("RUN_DEEPSEEK_LIVE") != "1" {
		t.Skip("set RUN_DEEPSEEK_LIVE=1 to observe next-input suggestions")
	}
	adapter := configuredDeepSeekAdapter(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	for _, mode := range []model.RunMode{model.ModeChat, model.ModeGrill, model.ModeExecute} {
		t.Run(string(mode), func(t *testing.T) {
			phase := PhaseChat
			if mode == model.ModeExecute {
				phase = PhaseExecuting
			}
			var finish ToolSchema
			for _, schema := range controlSchemas(phase, mode, nil) {
				if schema.Name == "finish" {
					finish = schema
					break
				}
			}
			if finish.Name == "" {
				t.Fatal("finish is not disclosed")
			}
			response, err := adapter.Generate(ctx, llm.GenerateRequest{
				Messages: []llm.Message{
					{Role: llm.RoleSystem, Content: llm.TextContent(
						"You are completing a PPT creation task in " + string(mode) + " mode.\n" +
							loadPromptModule("runtime.completion").Body + "\n" + loadPromptModule("runtime.next-input-suggestions").Body,
					)},
					{Role: llm.RoleUser, Content: llm.TextContent("The Chinese user asked to improve the narrative of a product launch deck. The work is complete: the opening now states the audience problem and slide 2 has a clearer evidence hierarchy. Call finish exactly once with a concise Chinese final response and zero to three useful next-input suggestions.")},
				},
				Tools:           toLLMToolSchemas([]ToolSchema{finish}),
				Reasoning:       llm.ReasoningDisabled,
				MaxOutputTokens: 320,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "finish" {
				t.Fatalf("expected one finish call, got text=%q calls=%+v", response.Text(), response.ToolCalls)
			}
			call := response.ToolCalls[0]
			message, ok := call.Args["message"].(string)
			if !ok || strings.TrimSpace(message) == "" {
				t.Fatalf("finish message is empty: %+v", call.Args)
			}
			suggestions := NormalizeSuggestedNextInputs(call.Args["suggested_next_inputs"])
			t.Logf("mode=%s suggestions=%d values=%q", mode, len(suggestions), suggestions)
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
	return llm.NewDeepSeekAdapter(llm.DeepSeekConfig{
		APIKey: profile.Key, BaseURL: profile.URL, Model: profile.Model,
		Timeout: 90 * time.Second,
	})
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
	pack := testPack(mode, scope.Object, scope.Source.Kind, false, "live schema validation")
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
