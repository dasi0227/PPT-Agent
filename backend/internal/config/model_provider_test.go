package config

import "testing"

func TestInferModelProvider(t *testing.T) {
	for _, tc := range []struct{ model, provider string }{
		{"deepseek-chat", "deepseek"},
		{" DeepSeek/DeepSeek-R1 ", "deepseek"},
		{"deepseek-r1-distill-qwen-32b", "deepseek"},
		{"kimi-k3", "kimi"},
		{"moonshot-v1-128k", "kimi"},
		{"xiaomi/MiMo-V2-Flash", "mimo"},
		{"openai/gpt-5", "openai"},
		{"chatgpt-4o-latest", "openai"},
		{"o3-mini", "openai"},
		{"claude-sonnet-4-5", "anthropic"},
		{"google/gemini-2.5-pro", "gemini"},
		{"Qwen/Qwen3-32B", "qwen"},
		{"MiniMax-M2", "minimax"},
		{"z-ai/glm-4.5", "zai"},
		{"", "custom"},
		{"my-gpt-proxy", "custom"},
		{"gptastic", "custom"},
		{"openai/private-model", "custom"},
		{"deepseek/private-model", "custom"},
		{"future-model", "custom"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			if got := InferModelProvider(tc.model); got != tc.provider {
				t.Fatalf("provider = %q, want %q", got, tc.provider)
			}
		})
	}
}
