package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"go.uber.org/zap"
)

func sourceFixture(t *testing.T) (*SlideSourceService, model.Project) {
	t.Helper()
	root := t.TempDir()
	db, closeDB, err := sqlitestore.Open(&config.Config{WorkRoot: root, DBPath: filepath.Join(root, "db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDB)
	store, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project := model.Project{ID: "pro_source", WorkDir: filepath.Join(root, "projects", "pro_source", "artifacts"), Title: "Source", Theme: "default", CreatedAt: 1, UpdatedAt: 1}
	ctx := context.Background()
	if err := store.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceSlides(ctx, project.ID, []model.Slide{{ID: "sli_source", ProjectID: project.ID}}); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		path := filepath.Join(project.WorkDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(".outline.json", `{"sections":[{"id":"sec_source","slides":[{"slide_id":"sli_source","title":"Source","role":"content"}]}]}`)
	write(model.SpecCollectionPath, `{"sli_source":{"key_message":"原始","elements":[]}}`)
	write(model.SlideHTMLPath("sli_source"), `<html><body><section class="slide-stage">原始</section></body></html>`)
	return NewSlideSourceService(projecthistory.New(store, run.NewLockManager(), root)), project
}

func sourceCode(err error) string {
	var typed *SlideSourceError
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

func TestHTMLSourceIsRawAndRejectsJSON(t *testing.T) {
	svc, project := sourceFixture(t)
	doc, err := svc.Read(context.Background(), project.ID, "sli_source", "html")
	if err != nil || doc.Path != "sli_source.html" || doc.Content != `<html><body><section class="slide-stage">原始</section></body></html>` {
		t.Fatalf("raw HTML: %+v %v", doc, err)
	}
	for _, kind := range []string{"spec", "design", "manifest", "outline"} {
		if _, err := svc.Read(context.Background(), project.ID, "sli_source", kind); sourceCode(err) != "SOURCE_REQUEST_INVALID" {
			t.Fatalf("JSON source accepted: %s %v", kind, err)
		}
	}
}
