package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func writeRepositoryFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func completeThemeCSS() string {
	var css strings.Builder
	css.WriteString(":root {\n")
	for _, token := range designsystem.RequiredTokens() {
		css.WriteString("  " + token + ": 1;\n")
	}
	css.WriteString("}\n")
	return css.String()
}

func themeFile(name, description, css string) string {
	return "/*\n---\nname: " + name + "\ndescription: " + description + "\n---\n*/\n" + css
}

func componentFile(name, description, html string) string {
	return "<!--\n---\nname: " + name + "\ndescription: " + description + "\n---\n-->\n" + html
}

type failingTagMetadataStore struct {
	*memoryRepositoryMetadataStore
}

func (s *failingTagMetadataStore) ReplaceResourceTagKeys(context.Context, string, string, []string) error {
	return errors.New("tag update failed")
}

func TestRepositoryServicesParseFrontmatterProtocols(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, filepath.Join(root, "assets/themes/t1/theme.css"), themeFile("Theme One", "Clear theme", completeThemeCSS()))
	writeRepositoryFile(t, filepath.Join(root, "assets/components/c1/index.html"), componentFile("Metric", "One metric", `<style>.metric{color:var(--color-primary)}</style><div class="metric">42%</div>`))
	writeRepositoryFile(t, filepath.Join(root, "assets/skills/s1/SKILL.md"), "---\nname: Story\ndescription: Shape a story.\n---\nLead with the conclusion.")

	theme, err := NewThemeService(WorkRoot(root)).Get("t1")
	if err != nil || theme.ID != "t1" || theme.Name != "Theme One" || theme.Description != "Clear theme" || !strings.Contains(theme.CSS, "--color-bg") {
		t.Fatalf("theme=%+v err=%v", theme, err)
	}
	component, err := NewComponentService(WorkRoot(root)).Get("c1")
	if err != nil || component.Name != "Metric" || !strings.Contains(component.HTML, "42%") {
		t.Fatalf("component=%+v err=%v", component, err)
	}
	skill, err := NewSkillService(WorkRoot(root)).Get("s1")
	if err != nil || skill.Name != "Story" || skill.Disabled || skill.Content != "Lead with the conclusion." {
		t.Fatalf("skill=%+v err=%v", skill, err)
	}
}

func TestRepositoryMetadataUpdateRestoresFileWhenTagWriteFails(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "assets/components/c1/index.html")
	original := componentFile("Card", "Original description", "<div>Card</div>\n")
	writeRepositoryFile(t, path, original)
	store := &failingTagMetadataStore{memoryRepositoryMetadataStore: newMemoryRepositoryMetadataStore()}
	service := NewComponentService(WorkRoot(root), store)

	if _, err := service.UpdateMetadata("c1", "Edited", "Edited description", []model.ComponentTag{"card"}); err == nil {
		t.Fatal("metadata update succeeded despite tag failure")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != original {
		t.Fatalf("component file was not restored:\n%s", raw)
	}
}

func TestThemeServiceAcceptsThinAndThickThemes(t *testing.T) {
	root := t.TempDir()
	for id, css := range map[string]string{
		"thin":  completeThemeCSS(),
		"thick": completeThemeCSS() + "\n.slide-stage { background: var(--color-bg); }\n",
	} {
		writeRepositoryFile(t, filepath.Join(root, "assets/themes", id, "theme.css"), themeFile("Theme", "Valid theme", css))
	}
	service := NewThemeService(WorkRoot(root))
	for _, id := range []string{"thin", "thick"} {
		if _, err := service.Get(id); err != nil {
			t.Fatalf("valid %s theme rejected: %v", id, err)
		}
	}
}

func TestThemeServiceRejectsEveryMissingRequiredToken(t *testing.T) {
	for _, omitted := range designsystem.RequiredTokens() {
		t.Run(omitted, func(t *testing.T) {
			root := t.TempDir()
			var css strings.Builder
			css.WriteString(":root {\n")
			for _, token := range designsystem.RequiredTokens() {
				if token != omitted {
					css.WriteString("  " + token + ": 1;\n")
				}
			}
			css.WriteString("}\n")
			writeRepositoryFile(t, filepath.Join(root, "assets/themes/incomplete/theme.css"), themeFile("Incomplete", "Missing one token", css.String()))

			service := NewThemeService(WorkRoot(root))
			if _, err := service.Get("incomplete"); !errors.Is(err, ErrRepositoryCorrupt) || !strings.Contains(err.Error(), omitted) {
				t.Fatalf("Get error = %v, want repository corruption naming %s", err, omitted)
			}
			if _, err := service.CSS("incomplete"); !errors.Is(err, ErrRepositoryCorrupt) || !strings.Contains(err.Error(), omitted) {
				t.Fatalf("CSS error = %v, want repository corruption naming %s", err, omitted)
			}
		})
	}
}

func TestThemeListSkipsThemesWithIncompleteTokens(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, filepath.Join(root, "assets/themes/valid/theme.css"), themeFile("Valid", "Complete tokens", completeThemeCSS()))
	writeRepositoryFile(t, filepath.Join(root, "assets/themes/invalid/theme.css"), themeFile("Invalid", "Incomplete tokens", `:root { --color-bg: #fff; }`))

	themes, err := NewThemeService(WorkRoot(root)).List()
	if err != nil || len(themes) != 1 || themes[0].ID != "valid" {
		t.Fatalf("themes=%+v err=%v", themes, err)
	}
}

func TestFactoryThemesFollowTokenAndSelectorContracts(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "seed", "assets", "themes", "*", "theme.css"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("factory theme paths=%v err=%v", paths, err)
	}
	legacySelector := regexp.MustCompile(`(?m)(^|[,{]\s*)\.(slide|title)(?:\s|[,>{:#.\[])`)
	for _, path := range paths {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if missing := designsystem.LintTokens(raw); len(missing) > 0 {
			t.Errorf("factory theme %s missing tokens: %v", path, missing)
		}
		metadata, _, parseErr := parseRepositoryFrontmatter(raw, cssFrontmatterStyle)
		if parseErr != nil || metadata.Name == "" || metadata.Description == "" {
			t.Errorf("factory theme %s metadata=%+v err=%v", path, metadata, parseErr)
		}
		if match := legacySelector.Find(raw); match != nil {
			t.Errorf("factory theme %s uses legacy selector %q", path, match)
		}
	}
	manifests, err := filepath.Glob(filepath.Join("..", "..", "..", "seed", "assets", "themes", "*", "manifest.json"))
	if err != nil || len(manifests) != 0 {
		t.Fatalf("legacy theme manifests=%v err=%v", manifests, err)
	}
}

func TestComponentMetadataRejectsLegacyJSONScript(t *testing.T) {
	raw := []byte(`<script type="application/json" id="meta">{"name":"Card","description":"Card reference"}</script><div>Card</div>`)
	if _, err := parseComponentMeta(raw); !errors.Is(err, ErrRepositoryCorrupt) {
		t.Fatalf("legacy JSON metadata err=%v", err)
	}
}

func TestComponentMetadataRejectsFileTags(t *testing.T) {
	raw := []byte("<!--\n---\nname: Card\ndescription: Card reference\ntags: [card]\n---\n-->\n<div>Card</div>")
	if _, err := parseComponentMeta(raw); !errors.Is(err, ErrRepositoryCorrupt) {
		t.Fatalf("file tags err=%v", err)
	}
}

func TestComponentMetadataRejectsLegacyKind(t *testing.T) {
	raw := []byte("<!--\n---\nname: Card\ndescription: Card reference\nkind: content\n---\n-->\n<div>Card</div>")
	if _, err := parseComponentMeta(raw); !errors.Is(err, ErrRepositoryCorrupt) {
		t.Fatalf("legacy kind err=%v", err)
	}
}

func TestComponentMetadataAcceptsContentFieldsOnly(t *testing.T) {
	raw := []byte(componentFile("Catalog", "Component metadata", "<div>Catalog</div>"))
	meta, err := parseComponentMeta(raw)
	if err != nil || meta.Name != "Catalog" {
		t.Fatalf("meta=%+v err=%v", meta, err)
	}
}

func TestFactoryComponentsFollowTheComponentContract(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "seed", "assets", "components", "*", "index.html"))
	if err != nil || len(paths) != 5 {
		t.Fatalf("factory component paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		meta, parseErr := parseComponentMeta(raw)
		if parseErr != nil || meta.Name == "" {
			t.Fatalf("factory component %s meta=%+v err=%v", path, meta, parseErr)
		}
	}
}

func TestFactorySkillsFollowTheSkillContract(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "seed", "assets", "skills", "*", "SKILL.md"))
	if err != nil || len(paths) != 3 {
		t.Fatalf("factory skill paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		meta, body, parseErr := parseSkillMarkdown(raw)
		if parseErr != nil || meta.Name == "" || meta.Description == "" || body == "" {
			t.Fatalf("factory skill %s meta=%+v err=%v", path, meta, parseErr)
		}
	}
}

func TestRepositoryServicesRejectTraversalSymlinksAndOversizeFiles(t *testing.T) {
	root := t.TempDir()
	themeRoot := filepath.Join(root, "assets/themes")
	writeRepositoryFile(t, filepath.Join(themeRoot, "valid/theme.css"), themeFile("Valid", "Valid theme", completeThemeCSS()))
	service := NewThemeService(WorkRoot(root))
	if _, err := service.Get("../valid"); !errors.Is(err, ErrInvalidRepositoryID) {
		t.Fatalf("traversal err=%v", err)
	}
	if err := service.Delete("../valid"); !errors.Is(err, ErrInvalidRepositoryID) {
		t.Fatalf("delete traversal err=%v", err)
	}
	if err := os.Symlink(filepath.Join(themeRoot, "valid"), filepath.Join(themeRoot, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get("linked"); !errors.Is(err, ErrUnsafeRepositoryPath) {
		t.Fatalf("symlink err=%v", err)
	}
	if err := service.Delete("linked"); !errors.Is(err, ErrUnsafeRepositoryPath) {
		t.Fatalf("delete symlink err=%v", err)
	}
	writeRepositoryFile(t, filepath.Join(themeRoot, "large/theme.css"), themeFile("Large", "Large theme", strings.Repeat("x", maxRepositoryFileSize+1)))
	if _, err := service.Get("large"); !errors.Is(err, ErrRepositoryFileTooLarge) {
		t.Fatalf("size err=%v", err)
	}
}

func TestSkillMetadataStateToggle(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, filepath.Join(root, "assets/skills/s1/SKILL.md"), "---\nname: Story\ndescription: Shape a story.\n---\nLead.")
	service := NewSkillService(WorkRoot(root))
	skills, err := service.List()
	if err != nil || len(skills) != 1 || skills[0].Disabled {
		t.Fatalf("missing registry: skills=%+v err=%v", skills, err)
	}
	updated, err := service.SetDisabled("s1", true)
	if err != nil || !updated.Disabled {
		t.Fatalf("disable: skill=%+v err=%v", updated, err)
	}
	if _, err := service.Resolve([]string{"s1"}); err == nil {
		t.Fatal("disabled skill resolved")
	}
	if err := service.Delete("s1"); err != nil {
		t.Fatalf("delete skill: %v", err)
	}
	skills, err = service.List()
	if err != nil || len(skills) != 0 {
		t.Fatalf("deleted skill remained listed: skills=%+v err=%v", skills, err)
	}
}

func TestComponentRegistryControlsDisabledState(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, filepath.Join(root, "assets/components/c1/index.html"), componentFile("Card", "Card reference", "<div>Card</div>"))
	service := NewComponentService(WorkRoot(root))
	component, err := service.SetDisabled("c1", true)
	if err != nil || !component.Disabled {
		t.Fatalf("disable component: %+v, %v", component, err)
	}
	components, err := service.LoadComponents(context.Background())
	if err != nil || len(components) != 0 {
		t.Fatalf("disabled component exposed to agent: %+v, %v", components, err)
	}
}
