package service

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func TestCreateProjectCommitsInitialScaffold(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, cleanupDB, err := sqlitestore.Open(&config.Config{
		DBPath:   filepath.Join(root, "project.db"),
		WorkRoot: root,
	}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanupDB)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	project, err := NewProjectService(st, WorkRoot(root)).CreateProject(ctx, CreateProjectParams{
		Topic: "Initialization baseline", Language: "zh-CN",
	})
	if err != nil {
		t.Fatal(err)
	}
	if subject := projectGitOutput(t, project.WorkDir, "log", "-1", "--format=%s"); subject != initialProjectCommitTitle {
		t.Fatalf("initial commit subject = %q", subject)
	}
	if count := projectGitOutput(t, project.WorkDir, "rev-list", "--count", "HEAD"); count != "1" {
		t.Fatalf("initial commit count = %q", count)
	}
	if status := projectGitOutput(t, project.WorkDir, "status", "--porcelain=v1"); status != "" {
		t.Fatalf("new project repository is dirty: %q", status)
	}

	changed, cleanup, err := gitcommit.NewExecutor().StageAll(ctx, project.WorkDir, "verify-empty")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if changed.FilesChanged != 0 {
		t.Fatalf("initial scaffold remains uncommitted: %+v", changed)
	}
}

func TestSetThemePersistsThemeIDToProjectAndDesign(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, cleanupDB, err := sqlitestore.Open(&config.Config{
		DBPath:   filepath.Join(root, "project.db"),
		WorkRoot: root,
	}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanupDB)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	writeRepositoryFile(t, filepath.Join(root, "assets/themes/tokyo-night/manifest.json"), `{"name":"Tokyo Night","description":"Dark presentation"}`)
	writeRepositoryFile(t, filepath.Join(root, "assets/themes/tokyo-night/theme.css"), completeThemeCSS())
	svc := NewProjectServiceWithRepositories(st, WorkRoot(root), nil, NewThemeService(WorkRoot(root)))
	project, err := svc.CreateProject(ctx, CreateProjectParams{Topic: "Theme persistence", Language: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := svc.SetTheme(ctx, project.ID, "tokyo-night")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Theme != "tokyo-night" || updated.Theme == "Tokyo Night" || updated.DesignRevision != 2 {
		t.Fatalf("updated project=%+v", updated)
	}
	stored, err := st.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Theme != "tokyo-night" {
		t.Fatalf("stored project=%+v", stored)
	}
	refreshed, err := svc.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Theme != "tokyo-night" || refreshed.DesignRevision != 2 {
		t.Fatalf("refreshed project=%+v", refreshed)
	}
	raw, err := os.ReadFile(filepath.Join(project.WorkDir, "design.json"))
	if err != nil {
		t.Fatal(err)
	}
	var design spec.Design
	if err := json.Unmarshal(raw, &design); err != nil {
		t.Fatal(err)
	}
	if design.Theme != "tokyo-night" || design.Revision != 2 {
		t.Fatalf("persisted design=%+v", design)
	}
}

func projectGitOutput(t *testing.T, workDir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = workDir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
