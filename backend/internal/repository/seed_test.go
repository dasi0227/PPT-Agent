package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
}
