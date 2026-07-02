package asset

import (
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

// AC-ASSET-001 / DS-SEED-002：全部 seed manifest MUST 通过 asset-manifest schema 校验。
func TestSeedManifestsValid(t *testing.T) {
	entries, err := WalkSeedAssets()
	if err != nil {
		t.Fatalf("walk seed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no seed assets found")
	}
	for _, e := range entries {
		if err := ValidateManifestRaw(e.RawJSON); err != nil {
			t.Errorf("%s/%s manifest invalid: %v", e.KindDir, e.Name, err)
		}
		// 目录名 MUST 与 manifest.name 一致（seeding 依赖此同构）。
		if string(e.Manifest.Name) != e.Name {
			t.Errorf("%s/%s: dir name != manifest.name (%q)", e.KindDir, e.Name, e.Manifest.Name)
		}
		// kind MUST 与所在子目录一致。
		if kindDirs[e.Manifest.Kind] != e.KindDir {
			t.Errorf("%s/%s: kind %q not in dir %q", e.KindDir, e.Name, e.Manifest.Kind, e.KindDir)
		}
	}
}

// AC-TOKENS-002 / AC-ASSET-002 / DS-THEMES-001：每套 seed theme 提供必需 token 全集。
func TestSeedThemesTokenComplete(t *testing.T) {
	entries, err := WalkSeedAssets()
	if err != nil {
		t.Fatalf("walk seed: %v", err)
	}
	count := 0
	for _, e := range entries {
		if e.Manifest.Kind != KindTheme {
			continue
		}
		count++
		css, err := ReadSeedFile(path.Join(e.Dir, e.Manifest.Assets.Tokens))
		if err != nil {
			t.Fatalf("read %s tokens: %v", e.Name, err)
		}
		if miss := ValidateThemeTokens(css); len(miss) != 0 {
			t.Errorf("theme %s missing required tokens: %v", e.Name, miss)
		}
	}
	if count < 5 {
		t.Errorf("expected >=5 seed themes, got %d", count)
	}
}

// AC-HTML-001：seed layout 的整页 template MUST 通过 lint-slide。
func TestSeedLayoutsLintPass(t *testing.T) {
	entries, err := WalkSeedAssets()
	if err != nil {
		t.Fatalf("walk seed: %v", err)
	}
	count := 0
	for _, e := range entries {
		if e.Manifest.Kind != KindLayout {
			continue
		}
		count++
		html, err := ReadSeedFile(path.Join(e.Dir, e.Manifest.Assets.HTML))
		if err != nil {
			t.Fatalf("read %s template: %v", e.Name, err)
		}
		if ok, reason := designsystem.LintSlideResult(html); !ok {
			t.Errorf("layout %s template failed lint-slide: %s", e.Name, reason)
		}
	}
	if count < 13 {
		t.Errorf("expected >=13 seed layouts, got %d", count)
	}
}

// AC-CHARTS-001（静态部分）：不同 seed theme 的 --color-primary 取值不同，
// 证明图表配色（走 var(--color-*)）随主题换肤而变；组件本身无硬编码色由上一测试保证。
func TestSeedThemesPrimaryColorDiffers(t *testing.T) {
	entries, err := WalkSeedAssets()
	if err != nil {
		t.Fatalf("walk seed: %v", err)
	}
	seen := map[string]string{} // primary 取值 -> theme 名
	re := regexp.MustCompile(`--color-primary\s*:\s*([^;]+);`)
	distinct := 0
	for _, e := range entries {
		if e.Manifest.Kind != KindTheme {
			continue
		}
		css, err := ReadSeedFile(path.Join(e.Dir, e.Manifest.Assets.Tokens))
		if err != nil {
			t.Fatalf("read %s tokens: %v", e.Name, err)
		}
		m := re.FindSubmatch(css)
		if m == nil {
			t.Errorf("theme %s: --color-primary not found", e.Name)
			continue
		}
		val := strings.TrimSpace(string(m[1]))
		if _, dup := seen[val]; !dup {
			distinct++
		}
		seen[val] = e.Name
	}
	if distinct < 2 {
		t.Errorf("expected distinct primary colors across themes, got %d distinct", distinct)
	}
}

// DS-COMP-002：component css MUST 不含硬编码主题色（配色走 token）。
func TestSeedComponentsNoHardcodedColor(t *testing.T) {
	entries, err := WalkSeedAssets()
	if err != nil {
		t.Fatalf("walk seed: %v", err)
	}
	for _, e := range entries {
		if e.Manifest.Kind != KindComponent || e.Manifest.Assets.CSS == "" {
			continue
		}
		css, err := ReadSeedFile(path.Join(e.Dir, e.Manifest.Assets.CSS))
		if err != nil {
			t.Fatalf("read %s css: %v", e.Name, err)
		}
		for _, c := range designsystem.LintSlide(css) {
			if c.ID == "no-hardcoded-theme" && !c.OK {
				t.Errorf("component %s css has hardcoded theme value: %s", e.Name, c.Reason)
			}
		}
	}
}
