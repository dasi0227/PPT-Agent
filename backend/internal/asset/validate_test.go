package asset

import (
	"os"
	"path/filepath"
	"testing"
)

// 内嵌 schema MUST 与 docs 权威 schema 保持一致（DEV-RULES R1）。
func TestSchemaMatchesDocs(t *testing.T) {
	docsPath := filepath.Join("..", "..", "..", "docs", "v1", "60-design-system", "asset-manifest.schema.json")
	docs, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read docs schema %s: %v", docsPath, err)
	}
	if string(docs) != string(schemaBytes) {
		t.Errorf("embedded asset-manifest.schema.json diverged from docs (source of truth); re-copy it")
	}
}

// AC-ASSET-001：各 kind 的合法 manifest 通过 schema 校验。
func TestValidateManifestAcceptsEachKind(t *testing.T) {
	cases := map[string]Manifest{
		"theme": {
			Name: "tokyo-night", Version: "1.0.0", Kind: KindTheme, Source: SourcePreset,
			Description: "深色霓虹主题", Assets: Payload{Tokens: "tokens.css"},
		},
		"layout": {
			Name: "cover", Version: "1.0.0", Kind: KindLayout, Source: SourcePreset,
			Description: "封面版式", Mount: &Mount{Position: "replace"},
			Assets: Payload{HTML: "template.html", CSS: "style.css"},
		},
		"component": {
			Name: "stat-badge", Version: "1.0.0", Kind: KindComponent, Source: SourcePreset,
			Description: "关键数字徽标", Mount: &Mount{Position: "append"},
			Params: map[string]Param{"value": {Type: "string", Default: "42"}},
			Assets: Payload{HTML: "template.html"},
		},
		"fx": {
			Name: "fade-in", Version: "1.0.0", Kind: KindFx, Source: SourcePreset,
			Description: "淡入动效", Mount: &Mount{Position: "wrap"}, Cleanup: true,
			Assets: Payload{JS: "effect.js"},
		},
	}
	for name, m := range cases {
		if err := ValidateManifest(m); err != nil {
			t.Errorf("%s manifest rejected: %v", name, err)
		}
	}
}

// AC-ASSET-001（反例）：fx 缺 mount → 拒绝（schema allOf 条件）。
func TestValidateManifestRejectsFxWithoutMount(t *testing.T) {
	m := Manifest{
		Name: "fade-in", Version: "1.0.0", Kind: KindFx,
		Description: "淡入", Assets: Payload{JS: "effect.js"},
	}
	if err := ValidateManifest(m); err == nil {
		t.Fatal("expected rejection: fx requires mount")
	}
}

// 反例：theme 缺 tokens 载荷 → 拒绝。
func TestValidateManifestRejectsThemeWithoutTokens(t *testing.T) {
	m := Manifest{
		Name: "x", Version: "1.0.0", Kind: KindTheme, Description: "缺 tokens",
	}
	if err := ValidateManifest(m); err == nil {
		t.Fatal("expected rejection: theme requires assets.tokens")
	}
}

// 反例：name 非法（大写）→ 拒绝（pattern）。
func TestValidateManifestRejectsBadName(t *testing.T) {
	m := Manifest{
		Name: "BadName", Version: "1.0.0", Kind: KindTheme, Description: "x",
		Assets: Payload{Tokens: "tokens.css"},
	}
	if err := ValidateManifest(m); err == nil {
		t.Fatal("expected rejection: name pattern")
	}
}

// AC-ASSET-002：theme tokens 全集齐备 → 无缺失；残缺 → 报缺失。
func TestValidateThemeTokens(t *testing.T) {
	var full []byte
	for _, tk := range []string{
		"--color-bg", "--color-fg", "--color-primary", "--color-accent", "--color-muted",
		"--font-sans", "--text-title", "--text-body", "--space-4", "--radius-md",
		"--shadow-card", "--stage-w", "--stage-h",
	} {
		full = append(full, []byte(tk+": x;\n")...)
	}
	if miss := ValidateThemeTokens(full); len(miss) != 0 {
		t.Fatalf("full token set reported missing: %v", miss)
	}
	if miss := ValidateThemeTokens([]byte("--color-bg: #000;")); len(miss) == 0 {
		t.Fatal("expected missing tokens for incomplete theme")
	}
}
