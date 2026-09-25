package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

func writeRepositoryFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func completeThemeCSS() string {
	var css strings.Builder
	css.WriteString(":root {\n")
	for _, token := range designsystem.RequiredTokens() {
		css.WriteString(token + ": 1;\n")
	}
	css.WriteString("}\n")
	return css.String()
}
func registerFixture(t *testing.T, root string, st ResourceStore, kind, id, name, description, body string) {
	t.Helper()
	folder, file, _, err := resourceFile(kind)
	if err != nil {
		t.Fatal(err)
	}
	writeRepositoryFile(t, filepath.Join(root, "assets", folder, id, file), body)
	if _, err = NewResourceService(WorkRoot(root), st).Register(context.Background(), model.Resource{Type: kind, ID: id, Name: name, Description: description}); err != nil {
		t.Fatal(err)
	}
}
func TestResourceMetadataNeverRewritesPayload(t *testing.T) {
	root := t.TempDir()
	st := newMemoryResourceStore()
	svc := NewResourceService(WorkRoot(root), st)
	for kind, body := range map[string]string{"theme": completeThemeCSS(), "component": "<article>Card</article>", "skill": "# Instructions\n\n---\nname: literal example\n---\n", "snippet": "A short instruction"} {
		registerFixture(t, root, st, kind, "sample", "Original", "Description", body)
		before, _, path, _, err := svc.Inspect(context.Background(), kind, "sample")
		if err != nil {
			t.Fatal(err)
		}
		var hash string
		if kind == "theme" {
			theme, _ := NewThemeService(WorkRoot(root), st).Get("sample")
			hash = theme.StyleHash
		}
		name, desc, disabled := "Renamed", "New description", true
		if _, err = svc.Patch(context.Background(), kind, "sample", ResourcePatch{Name: &name, Description: &desc, Disabled: &disabled}); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		if string(raw) != body {
			t.Fatal("metadata rewrote payload")
		}
		after, _, _, _, _ := svc.Inspect(context.Background(), kind, "sample")
		if after.Name == before.Name || !after.Disabled {
			t.Fatal("metadata was not stored")
		}
		if kind == "theme" {
			theme, _ := NewThemeService(WorkRoot(root), st).Get("sample")
			if theme.StyleHash != hash {
				t.Fatal("metadata changed theme hash")
			}
		}
	}
}
func TestResourcesRemainManageableWhenPayloadIsMissingOrInvalid(t *testing.T) {
	root := t.TempDir()
	st := newMemoryResourceStore()
	svc := NewResourceService(WorkRoot(root), st)
	registerFixture(t, root, st, "component", "card", "Card", "Description", "<div>card</div>")
	path, _ := svc.payloadPath("component", "card")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	components := NewComponentService(WorkRoot(root), st)
	list, err := components.List()
	if err != nil || len(list) != 1 || list[0].ContentState != "missing" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	loaded, err := components.LoadComponents(context.Background())
	if err != nil || len(loaded) != 0 {
		t.Fatal("missing component exposed")
	}
	name := "Repair later"
	if _, err = svc.Patch(context.Background(), "component", "card", ResourcePatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	writeRepositoryFile(t, path, "")
	list, err = components.List()
	if err != nil || list[0].ContentState != "invalid" {
		t.Fatalf("invalid list=%+v err=%v", list, err)
	}
	if err = svc.Delete(context.Background(), "component", "card"); err != nil {
		t.Fatal(err)
	}
	if _, err = st.GetResource(context.Background(), "component", "card"); !errors.Is(err, store.ErrResourceNotFound) {
		t.Fatal(err)
	}
}
func TestResourceRegistrationRequiresIdentityValidContentAndTags(t *testing.T) {
	root := t.TempDir()
	st := newMemoryResourceStore()
	svc := NewResourceService(WorkRoot(root), st)
	registerFixture(t, root, st, "component", "card", "Card", "Description", "<div>card</div>")
	r := model.Resource{Type: "component", ID: "card", Name: "Card", Description: "Description"}
	if _, err := svc.Register(context.Background(), r); !errors.Is(err, store.ErrResourceConflict) {
		t.Fatal(err)
	}
	r.ID = "../card"
	if _, err := svc.Register(context.Background(), r); !errors.Is(err, ErrInvalidRepositoryID) {
		t.Fatal(err)
	}
	r.ID = "other"
	r.Tags = []string{"workflow"}
	writeRepositoryFile(t, filepath.Join(root, "assets/components/other/index.html"), "<div>other</div>")
	if _, err := svc.Register(context.Background(), r); !errors.Is(err, store.ErrTagNotFound) {
		t.Fatal(err)
	}
	writeRepositoryFile(t, filepath.Join(root, "assets/components/unregistered/index.html"), "<div>unregistered</div>")
	rows, _ := st.ListResources(context.Background(), "component")
	if len(rows) != 1 {
		t.Fatal("unregistered resource appeared")
	}
	if err := os.Symlink(filepath.Join(root, "assets/components/card"), filepath.Join(root, "assets/components/linked")); err != nil {
		t.Fatal(err)
	}
	r.ID = "linked"
	r.Tags = nil
	if _, err := svc.Register(context.Background(), r); !errors.Is(err, ErrUnsafeRepositoryPath) {
		t.Fatal(err)
	}
}
func TestResourceInitializationIsExplicitAndIdempotent(t *testing.T) {
	root := t.TempDir()
	st := newMemoryResourceStore()
	svc := NewResourceService(WorkRoot(root), st)
	ctx := context.Background()
	if err := svc.InitializeResources(ctx, filepath.Join("..", "..", "..", "seed")); err != nil {
		t.Fatal(err)
	}
	snippets, err := svc.Snippets(ctx)
	if err != nil || len(snippets) != 6 {
		t.Fatalf("snippets=%d err=%v", len(snippets), err)
	}
	chosen := snippets[0]
	name := "User name"
	if _, err = svc.Patch(ctx, "snippet", chosen.ID, ResourcePatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.WriteSnippet(ctx, chosen.ID, "User body"); err != nil {
		t.Fatal(err)
	}
	if err = svc.InitializeResources(ctx, filepath.Join("..", "..", "..", "seed")); err != nil {
		t.Fatal(err)
	}
	current, _ := svc.Snippet(ctx, chosen.ID)
	if current.Name != name || current.Content != "User body" {
		t.Fatal("initialization overwrote user edits")
	}
	for _, id := range []string{"frontend-design", "design-taste-frontend"} {
		v, err := NewSkillService(WorkRoot(root), st).ResolveDynamic([]string{id})
		if err != nil || len(v) != 1 || strings.HasPrefix(v[0].Content, "---\n") {
			t.Fatalf("seed skill: %v", err)
		}
	}
	if err = svc.Delete(ctx, "snippet", chosen.ID); err != nil {
		t.Fatal(err)
	}
	if err = NewResourceService(WorkRoot(root), st).RecoverDeletes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Snippet(ctx, chosen.ID); !errors.Is(err, store.ErrResourceNotFound) {
		t.Fatal("startup recreated resource")
	}
}
