package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

type previewContentStore struct {
	store.Store
	project model.Project
}

func (s previewContentStore) GetSlide(context.Context, string) (model.Slide, error) {
	return model.Slide{ID: "sli_one", ProjectID: s.project.ID}, nil
}

func (s previewContentStore) GetProject(context.Context, string) (model.Project, error) {
	return s.project, nil
}

func TestReadHTMLBindsPreviewToSourceHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, model.SlideHTMLPath("sli_one"))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	original := []byte("<h1>original</h1>")
	write := func(path string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(path, original)
	write(filepath.Join(dir, "design.json"), []byte(`{"theme":"swiss-modern"}`))
	svc := NewSlideService(previewContentStore{project: model.Project{ID: "pro_one", WorkDir: dir}})
	hash := spec.ContentHash(original)
	preview, err := svc.ReadHTML(context.Background(), "sli_one", hash)
	if err != nil || !strings.Contains(string(preview), "<h1>original</h1>") || !strings.Contains(string(preview), "/themes/swiss-modern/css") {
		t.Fatalf("matching source must return normalized preview: %s, %v", preview, err)
	}
	changed := []byte("<h1>changed during load</h1>")
	write(path, changed)
	if preview, err = svc.ReadHTML(context.Background(), "sli_one", hash); !errors.Is(err, ErrSlideHTMLChanged) || preview != nil {
		t.Fatalf("stale snapshot received a different artifact: %s, %v", preview, err)
	}
	if preview, err = svc.ReadHTML(context.Background(), "sli_one", spec.ContentHash(changed)); err != nil || !strings.Contains(string(preview), "changed during load") {
		t.Fatalf("refreshed snapshot cannot load current content: %s, %v", preview, err)
	}
}
