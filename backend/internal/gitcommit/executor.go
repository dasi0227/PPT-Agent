package gitcommit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxDiffBytes = 96 * 1024
	ignoreBlock  = "threads/\n.run/\n.commit-tmp/\n*.tmp\n"
)

type ChangeSet struct {
	IndexPath    string
	NameStatus   string
	NumStat      string
	Diff         string
	FilesChanged int
	Insertions   int
	Deletions    int
}

type Message struct {
	Title string
	Items []string
}

type Result struct {
	Branch      string
	Hash        string
	CommittedAt string
}

type Executor struct{}

func NewExecutor() *Executor { return &Executor{} }

func (e *Executor) Bootstrap(ctx context.Context, workDir string) error {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	gitDir := filepath.Join(workDir, ".git")
	if _, err := os.Stat(gitDir); errors.Is(err, os.ErrNotExist) {
		if _, runErr := e.run(ctx, workDir, nil, "init", "--initial-branch=main"); runErr != nil {
			return fmt.Errorf("initialize Git repository: %w", runErr)
		}
	} else if err != nil {
		return err
	}
	root, err := e.run(ctx, workDir, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("resolve Git repository: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(strings.TrimSpace(root))
	if err != nil {
		return err
	}
	resolvedWorkDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		return err
	}
	if resolvedRoot != resolvedWorkDir {
		return errors.New("Git repository root does not match project work directory")
	}
	if err := ensureIgnore(filepath.Join(workDir, ".gitignore")); err != nil {
		return err
	}
	for _, marker := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(gitDir, marker)); err == nil {
			return errors.New("Git repository has an unfinished operation")
		}
	}
	return nil
}

func (e *Executor) StageAll(ctx context.Context, workDir, operationID string) (ChangeSet, func(), error) {
	gitDir := filepath.Join(workDir, ".git")
	tempDir := filepath.Join(gitDir, "ppt-agent")
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return ChangeSet{}, nil, err
	}
	indexPath := filepath.Join(tempDir, "index-"+operationID)
	_ = os.Remove(indexPath)
	cleanup := func() { _ = os.Remove(indexPath) }
	env := []string{"GIT_INDEX_FILE=" + indexPath}
	if _, err := e.run(ctx, workDir, env, "rev-parse", "--verify", "HEAD"); err == nil {
		if _, err := e.run(ctx, workDir, env, "read-tree", "HEAD"); err != nil {
			cleanup()
			return ChangeSet{}, nil, err
		}
	} else if _, err := e.run(ctx, workDir, env, "read-tree", "--empty"); err != nil {
		cleanup()
		return ChangeSet{}, nil, err
	}
	if _, err := e.run(ctx, workDir, env, "add", "-A", "--", "."); err != nil {
		cleanup()
		return ChangeSet{}, nil, err
	}
	nameStatus, err := e.run(ctx, workDir, env, "diff", "--cached", "--name-status", "--no-renames")
	if err != nil {
		cleanup()
		return ChangeSet{}, nil, err
	}
	numStat, err := e.run(ctx, workDir, env, "diff", "--cached", "--numstat", "--no-renames")
	if err != nil {
		cleanup()
		return ChangeSet{}, nil, err
	}
	diff, err := e.run(ctx, workDir, env, "diff", "--cached", "--no-color", "--no-ext-diff", "--unified=2")
	if err != nil {
		cleanup()
		return ChangeSet{}, nil, err
	}
	if len(diff) > maxDiffBytes {
		diff = diff[:maxDiffBytes] + "\n[diff truncated]\n"
	}
	files, insertions, deletions := parseNumStat(numStat)
	return ChangeSet{
		IndexPath: indexPath, NameStatus: nameStatus, NumStat: numStat, Diff: diff,
		FilesChanged: files, Insertions: insertions, Deletions: deletions,
	}, cleanup, nil
}

func (e *Executor) Commit(ctx context.Context, workDir string, changes ChangeSet, message Message) (Result, error) {
	body := make([]string, 0, len(message.Items))
	for _, item := range message.Items {
		body = append(body, "- "+item)
	}
	env := []string{
		"GIT_INDEX_FILE=" + changes.IndexPath,
		"GIT_AUTHOR_NAME=PPT Agent",
		"GIT_AUTHOR_EMAIL=ppt-agent@local",
		"GIT_COMMITTER_NAME=PPT Agent",
		"GIT_COMMITTER_EMAIL=ppt-agent@local",
	}
	args := []string{"commit", "-m", message.Title}
	if len(body) > 0 {
		args = append(args, "-m", strings.Join(body, "\n"))
	}
	if _, err := e.run(ctx, workDir, env, args...); err != nil {
		return Result{}, err
	}
	realIndex := filepath.Join(workDir, ".git", "index")
	if err := os.Rename(changes.IndexPath, realIndex); err != nil {
		return Result{}, fmt.Errorf("align Git index: %w", err)
	}
	branch, err := e.run(ctx, workDir, nil, "branch", "--show-current")
	if err != nil || strings.TrimSpace(branch) == "" {
		return Result{}, errors.New("read committed branch")
	}
	hash, err := e.run(ctx, workDir, nil, "rev-parse", "--short=7", "HEAD")
	if err != nil {
		return Result{}, err
	}
	committedAt, err := e.run(ctx, workDir, nil, "show", "-s", "--format=%cI", "HEAD")
	if err != nil {
		return Result{}, err
	}
	return Result{
		Branch: strings.TrimSpace(branch), Hash: strings.TrimSpace(hash),
		CommittedAt: strings.TrimSpace(committedAt),
	}, nil
}

func (e *Executor) run(ctx context.Context, workDir string, extraEnv []string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = workDir
	command.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}
	return stdout.String(), nil
}

func parseNumStat(value string) (files, insertions, deletions int) {
	for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		files++
		if fields[0] != "-" {
			if n, err := strconv.Atoi(fields[0]); err == nil {
				insertions += n
			}
		}
		if fields[1] != "-" {
			if n, err := strconv.Atoi(fields[1]); err == nil {
				deletions += n
			}
		}
	}
	return files, insertions, deletions
}

func ensureIgnore(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	existing := string(raw)
	var missing []string
	for _, entry := range strings.Split(strings.TrimSpace(ignoreBlock), "\n") {
		found := false
		for _, line := range strings.Split(existing, "\n") {
			if strings.TrimSpace(line) == entry {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, entry)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	existing += strings.Join(missing, "\n") + "\n"
	return os.WriteFile(path, []byte(existing), 0o644)
}
