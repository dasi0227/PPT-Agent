package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

func TestInitializerOnlyWritesMissingRepositoryFiles(t *testing.T) {
	root := t.TempDir()
	custom := filepath.Join(root, "assets", "themes", "swiss-modern", "theme.css")
	if err := os.MkdirAll(filepath.Dir(custom), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(custom, []byte("/* user theme */"), 0o644); err != nil {
		t.Fatal(err)
	}
	initializer := Initializer{WorkRoot: root}
	if err := initializer.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := initializer.Initialize(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(custom)
	if err != nil || string(raw) != "/* user theme */" {
		t.Fatalf("user file was overwritten: %q err=%v", raw, err)
	}
	component, err := os.ReadFile(filepath.Join(root, "assets", "components", "feature-card", "index.html"))
	if err != nil || !strings.Contains(string(component), `id="meta"`) {
		t.Fatalf("missing component seed: %v", err)
	}
	for _, themeID := range []string{"blueprint", "corporate-clean", "xiaohongshu-white"} {
		themeCSS, readErr := os.ReadFile(filepath.Join(root, "assets", "themes", themeID, "theme.css"))
		if readErr != nil {
			t.Fatalf("missing system theme %s: %v", themeID, readErr)
		}
		if missing := designsystem.LintTokens(themeCSS); len(missing) > 0 {
			t.Fatalf("system theme %s has incomplete tokens: %v", themeID, missing)
		}
	}
	for _, themeID := range []string{
		"blueprint", "bold-signal", "corporate-clean", "editorial-serif",
		"swiss-modern", "tokyo-night", "warm-pastel", "xiaohongshu-white",
	} {
		manifest, manifestErr := os.ReadFile(filepath.Join(root, "assets", "themes", themeID, "manifest.json"))
		if manifestErr != nil || strings.Contains(string(manifest), `"tags"`) {
			t.Fatalf("system theme %s manifest must not contain tags: %v", themeID, manifestErr)
		}
	}
}
