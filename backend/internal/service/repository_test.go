package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestRepositoryServicesParseIndependentProtocols(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, filepath.Join(root, "assets/themes/t1/manifest.json"), `{"name":"Theme One","description":"分类：Clear theme"}`)
	writeRepositoryFile(t, filepath.Join(root, "assets/themes/t1/theme.css"), `:root{--color-bg:#fff}`)
	writeRepositoryFile(t, filepath.Join(root, "assets/components/c1/index.html"), `<script type="application/json" id="meta">{"name":"Metric","description":"One metric","tags":["metric"],"kind":"data"}</script><style>.metric{color:var(--color-primary)}</style><div class="metric">42%</div>`)
	writeRepositoryFile(t, filepath.Join(root, "assets/skills/s1/SKILL.md"), "---\nname: Story\ndescription: Shape a story.\n---\nLead with the conclusion.")

	theme, err := NewThemeService(WorkRoot(root)).Get("t1")
	if err != nil || theme.ID != "t1" || theme.Name != "Theme One" || theme.Description != "Clear theme" || !strings.Contains(theme.CSS, "--color-bg") {
		t.Fatalf("theme=%+v err=%v", theme, err)
	}
	component, err := NewComponentService(WorkRoot(root)).Get("c1")
	if err != nil || component.Name != "Metric" || component.Kind != "data" || !strings.Contains(component.HTML, "42%") {
		t.Fatalf("component=%+v err=%v", component, err)
	}
	skill, err := NewSkillService(WorkRoot(root)).Get("s1")
	if err != nil || skill.Name != "Story" || skill.Disabled || skill.Content != "Lead with the conclusion." {
		t.Fatalf("skill=%+v err=%v", skill, err)
	}
}

func TestComponentTagsRejectUnknownAndDuplicateValues(t *testing.T) {
	for name, tags := range map[string]string{
		"unknown":   `["badge"]`,
		"duplicate": `["card","card"]`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := []byte(`<script type="application/json" id="meta">{"name":"Card","description":"Card reference","tags":` + tags + `}</script><div>Card</div>`)
			if _, err := parseComponentMeta(raw); !errors.Is(err, ErrRepositoryCorrupt) {
				t.Fatalf("tags %s err=%v", tags, err)
			}
		})
	}
}

func TestComponentTagsAcceptTheCompleteEnum(t *testing.T) {
	raw := []byte(`<script type="application/json" id="meta">{"name":"Catalog","description":"All supported tags","tags":["card","metric","comparison","quote","list","chart","process","timeline","other"]}</script><div>Catalog</div>`)
	meta, err := parseComponentMeta(raw)
	if err != nil || len(meta.Tags) != 9 {
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
		if parseErr != nil || len(meta.Tags) == 0 || meta.Kind == "" {
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
	writeRepositoryFile(t, filepath.Join(themeRoot, "valid/manifest.json"), `{"name":"Valid","description":"Valid theme"}`)
	writeRepositoryFile(t, filepath.Join(themeRoot, "valid/theme.css"), `:root{}`)
	service := NewThemeService(WorkRoot(root))
	if _, err := service.Get("../valid"); !errors.Is(err, ErrInvalidRepositoryID) {
		t.Fatalf("traversal err=%v", err)
	}
	if err := os.Symlink(filepath.Join(themeRoot, "valid"), filepath.Join(themeRoot, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get("linked"); !errors.Is(err, ErrUnsafeRepositoryPath) {
		t.Fatalf("symlink err=%v", err)
	}
	writeRepositoryFile(t, filepath.Join(themeRoot, "large/manifest.json"), `{"name":"Large","description":"Large theme"}`)
	writeRepositoryFile(t, filepath.Join(themeRoot, "large/theme.css"), strings.Repeat("x", maxRepositoryFileSize+1))
	if _, err := service.Get("large"); !errors.Is(err, ErrRepositoryFileTooLarge) {
		t.Fatalf("size err=%v", err)
	}
}

func TestSkillRegistryMissingCorruptAndAtomicToggle(t *testing.T) {
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
	writeRepositoryFile(t, filepath.Join(root, "assets/skills/registry.json"), `{`)
	if _, err := service.List(); !errors.Is(err, ErrRepositoryCorrupt) {
		t.Fatalf("corrupt registry err=%v", err)
	}
}

func TestComponentContractHasNoDisabledState(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, filepath.Join(root, "assets/components/c1/index.html"), `<script type="application/json" id="meta">{"name":"Card","description":"Card reference","tags":[]}</script><div>Card</div>`)
	component, err := NewComponentService(WorkRoot(root)).Get("c1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(component.HTML), `"disabled"`) {
		t.Fatal("component unexpectedly contains state")
	}
}
