package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func TestPresentationRollbackCreatesNewCanonicalVersion(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "slide.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	store, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.NewProjectService(store, service.WorkRoot(root)).CreateProject(context.Background(), service.CreateProjectParams{Topic: "Deck"})
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"common/base.css", "common/tokens.css"} {
		if _, err := os.Stat(filepath.Join(project.WorkDir, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("project runtime CSS missing %s: %v", relative, err)
		}
	}
	svc := service.NewSlideService(store)
	slide, err := svc.AddSlide(context.Background(), project.ID, "", "content")
	if err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideHTMLPath(slide.ID)))
	if err := os.MkdirAll(filepath.Dir(currentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentPath, []byte("<html>current</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := model.SlideHTMLVersionTarget(project.ID, slide.ID)
	snapshot := model.SlideHTMLVersionSnapshot(slide.ID, 0)
	snapshotPath := filepath.Join(project.WorkDir, filepath.FromSlash(snapshot))
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, []byte("<html>old</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateVersion(context.Background(), model.Version{
		ID: "v0", TargetType: "slide_html", TargetID: target,
		VersionNo: 0, SnapshotPath: snapshot, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	rolled, err := svc.RollbackSlide(context.Background(), slide.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rolled.CurrentVersion != 1 {
		t.Fatalf("version=%d", rolled.CurrentVersion)
	}
	raw, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "<html>old</html>" {
		t.Fatalf("html=%s", raw)
	}
	versions, err := svc.ListVersions(context.Background(), slide.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[1].VersionNo != 1 {
		t.Fatalf("versions=%+v", versions)
	}
}
