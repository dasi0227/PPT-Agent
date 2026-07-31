package generate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
)

func newDesignTool(t *testing.T) (*SubmitDesignSpecTool, *memStore, string) {
	t.Helper()
	dir := t.TempDir()
	sandbox, err := tools.NewSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := newMemStore()
	seq := 0
	newID := func() string { seq++; return "v-" + itoa(seq) }
	return NewSubmitDesignSpecTool(store, sandbox, "p1", "r1", func() int64 { return 1 }, newID), store, dir
}

func validSpecArgs() map[string]any {
	return map[string]any{
		"subject": map[string]any{"topic": "云原生可观测性", "audience": "工程决策者", "job": "说服采用"},
		"palette": []any{
			map[string]any{"name": "ink", "hex": "#12161C", "role": "背景/正文"},
			map[string]any{"name": "signal", "hex": "#3BA7A0", "role": "主强调"},
			map[string]any{"name": "amber", "hex": "#E0A340", "role": "次强调"},
		},
		"type": map[string]any{
			"display": map[string]any{"family": "Space Grotesk", "weights": []any{500, 700}, "usage": "大标题"},
			"body":    map[string]any{"family": "Inter", "weights": []any{400, 600}, "usage": "正文"},
		},
		"signature": "每页右下角一条细的链路脉冲 SVG",
	}
}

// AC-V2-PIPE-002：合法 design_spec → 落盘 + 产 design 版本 + emit design_spec artifact。
func TestDesignSpecSubmitPersistsAndVersions(t *testing.T) {
	tool, store, dir := newDesignTool(t)
	res, err := tool.Execute(context.Background(), validSpecArgs())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("valid spec rejected: %s", res.Observation)
	}
	if res.Artifact == nil || res.Artifact.Type != "design_spec" || res.Artifact.Ref != "design/design-spec.json" {
		t.Fatalf("bad artifact: %+v", res.Artifact)
	}
	// 落盘。
	raw, err := os.ReadFile(filepath.Join(dir, "design/design-spec.json"))
	if err != nil {
		t.Fatalf("design-spec.json not written: %v", err)
	}
	var spec blueprint.DesignSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("design-spec.json invalid: %v", err)
	}
	if len(spec.Palette) != 3 || spec.Signature == "" || spec.SchemaVersion != blueprint.SchemaVersion {
		t.Errorf("persisted spec incomplete: %+v", spec)
	}
	// 产 design 版本。
	if len(store.versions) != 1 || store.versions[0].TargetType != "design" {
		t.Errorf("want 1 design version, got %+v", store.versions)
	}
	// 读回。
	if tool.Spec() == nil || tool.Spec().Subject.Topic != "云原生可观测性" {
		t.Errorf("Spec() not readable back")
	}
}

// design_spec 校验：palette 不足 / 字体缺失 / signature 空 → 可操作错误，不落盘。
func TestDesignSpecValidationRejects(t *testing.T) {
	cases := map[string]func(map[string]any){
		"palette<3":    func(a map[string]any) { a["palette"] = []any{a["palette"].([]any)[0]} },
		"no signature": func(a map[string]any) { a["signature"] = "" },
		"body font missing family": func(a map[string]any) {
			a["type"].(map[string]any)["body"] = map[string]any{"weights": []any{400}, "usage": "x"}
		},
		"bad hex": func(a map[string]any) {
			a["palette"].([]any)[1].(map[string]any)["hex"] = "teal"
		},
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			tool, _, dir := newDesignTool(t)
			args := validSpecArgs()
			mut(args)
			res, err := tool.Execute(context.Background(), args)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if res.OK {
				t.Fatalf("invalid spec (%s) should be rejected", name)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "design/design-spec.json")); statErr == nil {
				t.Errorf("rejected spec must not be persisted")
			}
		})
	}
}

// tokensFromSpec 覆盖必需 token 全集，可通过 LintTokens。
func TestTokensFromSpecCoversRequired(t *testing.T) {
	spec := DesignSpec{
		Palette: []DesignColor{
			{Name: "paper", Hex: "#F5F3EC", Role: "浅底"},
			{Name: "signal", Hex: "#3BA7A0", Role: "主强调"},
			{Name: "ink", Hex: "#12161C", Role: "正文"},
		},
		Type: DesignType{
			Display: DesignFont{Family: "Space Grotesk", Weights: []int{700}, Usage: "标题"},
			Body:    DesignFont{Family: "Inter", Weights: []int{400}, Usage: "正文"},
		},
		Signature: "链路脉冲",
	}
	css := tokensFromSpec(spec)
	for _, tk := range []string{"--color-bg", "--color-fg", "--color-primary", "--color-accent", "--color-muted", "--font-sans", "--text-title", "--text-body", "--space-4", "--radius-md", "--shadow-card", "--stage-w", "--stage-h"} {
		if !containsToken(css, tk) {
			t.Errorf("generated tokens.css missing %s", tk)
		}
	}
}

func containsToken(css []byte, tk string) bool {
	return indexOf(string(css), tk+":") >= 0
}
