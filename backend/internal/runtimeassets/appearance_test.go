package runtimeassets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppearanceTracksCSSAndFrozenRuntimeResources(t *testing.T) {
	first := Appearance("editorial-serif", []byte(":root{--color-fg:#111;}"))
	same := Appearance("editorial-serif", []byte(":root{--color-fg:#111;}"))
	changed := Appearance("editorial-serif", []byte(":root{--color-fg:#222;}"))
	if first.Hash != same.Hash || first.Hash == changed.Hash || first.ThemeCSSURL == changed.ThemeCSSURL {
		t.Fatal("appearance identity does not track CSS bytes")
	}
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fonts.css", "base.css", "decorations.js", "theme-bridge.js", "fonts/NotoSansSC-Variable.ttf", "fonts/NotoSerifSC-Variable.ttf", "fonts/JetBrainsMono-Variable.ttf", "fonts/notosanssc-OFL.txt"} {
		frozen, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		embedded, err := Read(name)
		if err != nil {
			t.Fatal(err)
		}
		if Hash(frozen) != Hash(embedded) {
			t.Fatalf("resource changed in snapshot: %s", name)
		}
	}
}
