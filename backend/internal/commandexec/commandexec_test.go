package commandexec

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestParseAcceptedSubset(t *testing.T) {
	graph, err := Parse(`rg "deck field" backend | head -n 40 && git status --short`)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Groups) != 2 || len(graph.Groups[0].Commands) != 2 {
		t.Fatalf("unexpected graph: %#v", graph)
	}
	if got := graph.Groups[0].Commands[0].Args[1]; got != "deck field" {
		t.Fatalf("quote removal mismatch: %q", got)
	}
}

func TestParseDeniedSyntax(t *testing.T) {
	cases := []string{
		`cat $(pwd)`, "cat `pwd`", `cat $HOME`, `cat *.go`, `cat < file`,
		`rg x > result`, `rg x >> result`, `rg x || true`, `rg x; pwd`,
		"rg x\npwd", `rg x &`, `(pwd)`, `FOO=x pwd`, `rg x |& head`,
	}
	for _, source := range cases {
		t.Run(source, func(t *testing.T) {
			if _, err := Parse(source); err == nil {
				t.Fatalf("expected %q to be denied", source)
			}
		})
	}
}

func TestPolicyAllowsDocumentedReadCommands(t *testing.T) {
	root := testProject(t)
	policy, err := NewPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"pwd",
		"ls -la .",
		"cat -- notes.txt",
		"head -n 10 notes.txt",
		"tail -n 10 notes.txt",
		`find . -maxdepth 2 -type f -name "*.txt"`,
		"grep -n hello notes.txt",
		"rg hello .",
		"jq . data.json",
		"stat notes.txt",
		"sed -n 1,2p notes.txt",
		"wc -l notes.txt",
		"git status --short",
		"git diff -- notes.txt",
		"git log",
		"rg hello . | head -n 5",
		"pwd && git status --short",
	}
	for _, source := range cases {
		t.Run(source, func(t *testing.T) {
			decision := policy.Evaluate(source, true)
			if decision.Outcome != Allow {
				t.Fatalf("expected allow, got %#v", decision)
			}
		})
	}
}

func TestPolicyDeniesDangerousFlagsAndPrograms(t *testing.T) {
	root := testProject(t)
	policy, _ := NewPolicy(root)
	cases := []string{
		"curl https://example.com",
		"python script.py",
		"npm install",
		"git checkout main",
		"git -c core.pager=cat status",
		"git diff --ext-diff",
		"git diff --no-index notes.txt ../outside.txt",
		"git diff HEAD",
		"git log -p",
		"git log --pretty=raw",
		"rg --pre cat hello .",
		"jq --rawfile secret notes.txt . data.json",
		"find . -exec cat {} +",
		"find . -delete",
		"tail -f notes.txt",
		"sed -i .bak s/a/b/g notes.txt",
		"sed -i '' s/a/b/g notes.txt | cat",
	}
	for _, source := range cases {
		t.Run(source, func(t *testing.T) {
			if decision := policy.Evaluate(source, true); decision.Outcome != Deny {
				t.Fatalf("expected deny, got %#v", decision)
			}
		})
	}
}

func TestPolicyConfirmsSensitiveReadAndEdit(t *testing.T) {
	root := testProject(t)
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("TOKEN=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, _ := NewPolicy(root)
	read := policy.Evaluate("cat .env", true)
	if read.Outcome != Confirm || read.Mutates {
		t.Fatalf("unexpected sensitive decision: %#v", read)
	}
	edit := policy.Evaluate(`sed -i '' 's/hello/goodbye/g' notes.txt`, true)
	if edit.Outcome != Confirm || !edit.Mutates || edit.PreimageHash == "" {
		t.Fatalf("unexpected edit decision: %#v", edit)
	}
	if decision := policy.Evaluate(`sed -i '' 's/hello/goodbye/g' notes.txt`, false); decision.Outcome != Deny {
		t.Fatalf("write must be denied outside execute mode: %#v", decision)
	}
	diff := policy.Evaluate("git diff -- .env", true)
	if diff.Outcome != Confirm || diff.Mutates {
		t.Fatalf("unexpected sensitive git diff decision: %#v", diff)
	}
}

func TestPolicyRejectsGitRepositoryOutsideProjectRoot(t *testing.T) {
	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	policy, err := NewPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	if decision := policy.Evaluate("git status --short", true); decision.Outcome != Deny {
		t.Fatalf("parent repository was accepted: %#v", decision)
	}
}

func TestPathGuardRejectsEscapesAndSpecialFiles(t *testing.T) {
	root := testProject(t)
	guard, _ := NewPathGuard(root)
	outside := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "outside"), filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../outside", "/etc/passwd", "escape"} {
		if _, err := guard.Validate(path, false); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
	if runtime.GOOS != "windows" {
		fifo := filepath.Join(root, "pipe")
		if err := syscallMkfifo(fifo); err == nil {
			if _, err := guard.Validate("pipe", false); err == nil {
				t.Fatal("expected FIFO to be rejected")
			}
		}
	}
}

func TestExecutorPreservesPipelineAndAndSemantics(t *testing.T) {
	root := testProject(t)
	policy, _ := NewPolicy(root)
	executor, err := NewExecutor(root)
	if err != nil {
		t.Fatal(err)
	}
	decision := policy.Evaluate(`cat notes.txt | head -n 1 && wc -l notes.txt`, true)
	result, err := executor.Execute(context.Background(), decision.Graph)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stdout, "hello") || !strings.Contains(result.Stdout, "2") {
		t.Fatalf("unexpected output: %q", result.Stdout)
	}
}

func TestExecutorPipelineStopsUpstreamAfterDownstreamExits(t *testing.T) {
	if _, err := os.Stat("/usr/bin/yes"); err != nil {
		t.Skip("/usr/bin/yes is unavailable")
	}
	root := testProject(t)
	executor, _ := NewExecutor(root)
	executor.inventory["cat"] = "/usr/bin/yes"
	executor.timeout = time.Second
	result, err := executor.Execute(context.Background(), Graph{
		Groups: []Pipeline{{Commands: []Command{
			{Args: []string{"cat"}},
			{Args: []string{"head", "-n", "1"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "y\n" {
		t.Fatalf("stdout=%q", result.Stdout)
	}
}

func TestExecutorCancellation(t *testing.T) {
	root := testProject(t)
	executor, _ := NewExecutor(root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := executor.Execute(ctx, Graph{Groups: []Pipeline{{Commands: []Command{{Args: []string{"find", ".", "-maxdepth", "8", "-print"}}}}}})
	if err == nil {
		t.Fatal("expected cancellation")
	}
}

func TestExecutorStopsAtOutputLimit(t *testing.T) {
	if _, err := os.Stat("/usr/bin/yes"); err != nil {
		t.Skip("/usr/bin/yes is unavailable")
	}
	root := testProject(t)
	executor, _ := NewExecutor(root)
	executor.inventory["cat"] = "/usr/bin/yes"
	started := time.Now()
	result, err := executor.Execute(context.Background(), Graph{
		Groups: []Pipeline{{Commands: []Command{{Args: []string{"cat"}}}}},
	})
	if ErrorCode(err) != CodeOutputLimit {
		t.Fatalf("error=%v result=%+v", err, result)
	}
	if !result.OutputTruncated || len(result.Stdout) != stdoutLimit {
		t.Fatalf("output was not capped: bytes=%d result=%+v", len(result.Stdout), result)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("output limit did not stop the process promptly: %s", elapsed)
	}
}

func TestExecutorTimeoutKillsProcess(t *testing.T) {
	root := testProject(t)
	executor, _ := NewExecutor(root)
	tail := executor.inventory["tail"]
	if tail == "" {
		t.Skip("tail is unavailable")
	}
	executor.inventory["cat"] = tail
	executor.timeout = 20 * time.Millisecond
	result, err := executor.Execute(context.Background(), Graph{
		Groups: []Pipeline{{Commands: []Command{{Args: []string{"cat", "-f", "notes.txt"}}}}},
	})
	if ErrorCode(err) != CodeTimeout || result.ExitCode != -1 {
		t.Fatalf("error=%v result=%+v", err, result)
	}
}

func TestExecutorRejectsMissingBinary(t *testing.T) {
	root := testProject(t)
	executor, _ := NewExecutor(root)
	delete(executor.inventory, "cat")
	_, err := executor.Execute(context.Background(), Graph{
		Groups: []Pipeline{{Commands: []Command{{Args: []string{"cat", "notes.txt"}}}}},
	})
	if ErrorCode(err) != CodeNotAvailable {
		t.Fatalf("error=%v", err)
	}
}

func TestExecutorUsesMinimalEnvironmentAndAbsoluteInventory(t *testing.T) {
	root := testProject(t)
	executor, _ := NewExecutor(root)
	home := t.TempDir()
	env := executor.environment(home, "git")
	for _, required := range []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + home,
		"RIPGREP_CONFIG_PATH=",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_CONFIG_NOSYSTEM=1",
	} {
		if !slices.Contains(env, required) {
			t.Fatalf("environment missing %q: %v", required, env)
		}
	}
	for _, forbidden := range []string{"SSH_AUTH_SOCK=", "HTTP_PROXY=", "HTTPS_PROXY=", "GIT_CONFIG_COUNT="} {
		for _, item := range env {
			if strings.HasPrefix(item, forbidden) {
				t.Fatalf("environment leaked %q in %v", forbidden, env)
			}
		}
	}

	shadow := filepath.Join(root, "cat")
	if err := os.WriteFile(shadow, []byte("#!/bin/sh\nprintf shadowed\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := executor.Execute(context.Background(), Graph{
		Groups: []Pipeline{{Commands: []Command{{Args: []string{"cat", "notes.txt"}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Stdout, "shadowed") || !strings.Contains(result.Stdout, "hello") {
		t.Fatalf("project-local binary shadowed inventory: %q", result.Stdout)
	}
	if !filepath.IsAbs(executor.inventory["cat"]) {
		t.Fatalf("inventory path is not absolute: %q", executor.inventory["cat"])
	}
	pwd, err := executor.Execute(context.Background(), Graph{
		Groups: []Pipeline{{Commands: []Command{{Args: []string{"pwd"}}}}},
	})
	if err != nil || pwd.Stdout != ".\n" {
		t.Fatalf("project root leaked from pwd: stdout=%q err=%v", pwd.Stdout, err)
	}
}

func TestExecuteSedBytesNeverTouchesBaseline(t *testing.T) {
	root := testProject(t)
	executor, _ := NewExecutor(root)
	before, _ := os.ReadFile(filepath.Join(root, "notes.txt"))
	command := Command{Args: []string{"sed", "-i", "", "s/hello/goodbye/g", "notes.txt"}}
	updated, _, err := executor.ExecuteSedBytes(context.Background(), command, before)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "notes.txt"))
	if string(after) != string(before) {
		t.Fatal("baseline file changed")
	}
	if !strings.Contains(string(updated), "goodbye") {
		t.Fatalf("staged output was not edited: %q", updated)
	}
}

func testProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello\nworld\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func syscallMkfifo(path string) error {
	return syscall.Mkfifo(path, 0o600)
}

func TestExecutorTimeoutConfiguration(t *testing.T) {
	root := testProject(t)
	executor, _ := NewExecutor(root)
	executor.timeout = time.Second
	if executor.timeout != time.Second {
		t.Fatal("timeout override failed")
	}
}
