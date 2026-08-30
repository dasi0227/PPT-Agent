package service

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/gitcommit"
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
