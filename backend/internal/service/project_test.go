package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
	st, err := sqlitestore.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}

	project, err := NewProjectService(st, WorkRoot(root)).CreateProject(ctx, CreateProjectParams{
		Topic: "Initialization baseline",
	})
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Join(root, "projects", project.ID)
	if project.WorkDir != filepath.Join(projectRoot, "artifacts") {
		t.Fatalf("project work dir = %q", project.WorkDir)
	}
	for _, path := range []string{
		project.WorkDir,
		filepath.Join(projectRoot, "threads"),
		filepath.Join(projectRoot, "checkpoints"),
	} {
		if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
			t.Fatalf("project directory missing: %s (%v)", path, statErr)
		}
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
	snapshot, err := NewPPTMutationService(st).Snapshot(ctx, project.ID)
	if err != nil || snapshot.Outline.Sections == nil || len(snapshot.Outline.Sections) != 0 {
		t.Fatalf("initialized project snapshot: %+v %v", snapshot, err)
	}
	if !reflect.DeepEqual(snapshot.Manifest, &spec.Manifest{Requirements: []string{}, Prohibitions: []string{}}) ||
		!reflect.DeepEqual(snapshot.Design, &spec.Design{Requirements: []string{}, Prohibitions: []string{}}) {
		t.Fatalf("new project must expose empty resources: %+v", snapshot)
	}
	for _, part := range []string{"manifest", "design", "outline"} {
		if snapshot.Hashes[part] == "" {
			t.Fatalf("initialized %s has no source hash", part)
		}
		raw, err := os.ReadFile(filepath.Join(project.WorkDir, "."+part+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := spec.ParseStrictSourceJSON(raw, part); err != nil {
			t.Fatalf("initialized %s is invalid: %v", part, err)
		}
	}
	if _, err := os.Stat(filepath.Join(project.WorkDir, "state.json")); !os.IsNotExist(err) {
		t.Fatalf("project must not create obsolete state.json: %v", err)
	}

	changed, cleanup, err := gitcommit.NewExecutor().StageAll(ctx, project.WorkDir, "verify-empty")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if changed.FilesChanged != 0 {
		t.Fatalf("initial scaffold remains uncommitted: %+v", changed)
	}

	thread, err := NewThreadService(st).CreateThread(ctx, project.ID, CreateThreadParams{Title: "First task"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(projectRoot, filepath.FromSlash(model.ThreadJournalPath(thread.ID))),
	} {
		if info, statErr := os.Stat(path); statErr != nil || !info.Mode().IsRegular() {
			t.Fatalf("thread history file missing: %s (%v)", path, statErr)
		}
	}
	if _, statErr := os.Stat(filepath.Join(projectRoot, "threads", thread.ID, "memory.json")); !os.IsNotExist(statErr) {
		t.Fatalf("memory.json must not be created: %v", statErr)
	}
}

func TestSetThemePersistsOnlyProjectTheme(t *testing.T) {
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
	st, err := sqlitestore.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	registerFixture(t, root, st, "theme", "tokyo-night", "Tokyo Night", "Dark presentation", completeThemeCSS())
	svc := NewProjectServiceWithRepositories(st, WorkRoot(root), nil, NewThemeService(WorkRoot(root), st))
	project, err := svc.CreateProject(ctx, CreateProjectParams{Topic: "Theme persistence"})
	if err != nil {
		t.Fatal(err)
	}
	designBefore, err := os.ReadFile(filepath.Join(project.WorkDir, ".design.json"))
	if err != nil {
		t.Fatal(err)
	}

	updated, err := svc.SetTheme(ctx, project.ID, "tokyo-night")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Theme != "tokyo-night" || updated.Theme == "Tokyo Night" {
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
	if refreshed.Theme != "tokyo-night" {
		t.Fatalf("refreshed project=%+v", refreshed)
	}
	designAfter, err := os.ReadFile(filepath.Join(project.WorkDir, ".design.json"))
	if err != nil || string(designAfter) != string(designBefore) {
		t.Fatalf("theme change must preserve design file: %v", err)
	}
	disabled := true
	if _, err := svc.themes.resources.Patch(ctx, "theme", "tokyo-night", ResourcePatch{Disabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetTheme(ctx, project.ID, "tokyo-night"); !errors.Is(err, ErrThemeDisabled) {
		t.Fatalf("disabled theme accepted: %v", err)
	}
	unchanged, err := svc.GetProject(ctx, project.ID)
	if err != nil || unchanged.Theme != "tokyo-night" {
		t.Fatalf("existing theme changed: %+v, %v", unchanged, err)
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
