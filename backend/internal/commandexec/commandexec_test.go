package commandexec

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

func TestShellSyntaxHasNoReadingAllowlist(t *testing.T) {
	for _, source := range []string{"cat ~/.env", "find .. -type f", "python3 -c 'print(1)'", "x=hello; printf '%s' \"$x\" | cat > output; cat output", "false || echo recovered", "jq -n '1'"} {
		if d := Evaluate(source); d.Outcome == Deny {
			t.Fatalf("%s: %+v", source, d)
		}
	}
	if d := Evaluate("echo '"); d.ReasonCode != CodeInvalid {
		t.Fatalf("syntax=%+v", d)
	}
	for _, source := range []string{"git commit -m change", "git push origin main"} {
		if d := Evaluate(source); d.ReasonCode != CodeNotAllowed {
			t.Fatalf("write boundary=%+v", d)
		}
	}
}

func TestWorkspacePreservesNodesWithoutFollowingExternalLinks(t *testing.T) {
	root, copy := t.TempDir(), t.TempDir()
	_ = os.Mkdir(filepath.Join(root, "empty"), 0755)
	_ = os.WriteFile(filepath.Join(root, "script"), []byte("echo hello"), 0755)
	_ = os.Symlink("script", filepath.Join(root, "local"))
	_ = os.Symlink("/unreadable/external", filepath.Join(root, "external"))
	before, err := Snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := before.CopyTo(copy, root); err != nil {
		t.Fatal(err)
	}
	after, err := Snapshot(copy)
	if err != nil {
		t.Fatal(err)
	}
	if before.Hash() != after.Hash() {
		t.Fatal("copy lost filesystem metadata")
	}
	if err := ValidateLink(copy, "escape", Node{Kind: "symlink", Content: []byte("../outside")}); err == nil {
		t.Fatal("escaping link accepted")
	}
}

func TestRuntimeRestrictsExternalAccessAndReadOnlyMode(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires macOS sandbox-runtime")
	}
	root, outside := t.TempDir(), filepath.Join(t.TempDir(), "outside")
	_ = os.WriteFile(outside, []byte("readable"), 0600)
	executor := NewRunner(config.DefaultSandboxConfig())
	original := t.TempDir()
	for _, source := range []string{"x=hello; printf '%s' \"$x\" > local; cat local", "printf test > \"$TMPDIR/scratch\"; cat \"$TMPDIR/scratch\""} {
		result, err := executor.Execute(context.Background(), root, original, true, Evaluate(source))
		if err != nil {
			t.Fatalf("%s: %v, stderr=%s", source, err, result.Stderr)
		}
	}
	for _, source := range []string{"cat '" + outside + "'", "printf changed > '" + outside + "'", "ln '" + outside + "' alias && printf changed > alias", "mkdir .git; printf changed > .git/config", "curl --max-time 2 http://127.0.0.1:8787/api/v1/healthz"} {
		if _, err := executor.Execute(context.Background(), root, original, true, Evaluate(source)); err == nil {
			t.Fatalf("prohibited access succeeded: %s", source)
		}
	}
	if _, err := executor.Execute(context.Background(), root, original, false, Evaluate("printf changed > local")); err == nil {
		t.Fatal("read-only command wrote to workspace")
	}
	allowed := config.DefaultSandboxConfig()
	allowed.ReadRoots = []string{filepath.Dir(outside)}
	if _, err := NewRunner(allowed).Execute(context.Background(), root, original, false, Evaluate("cat '"+outside+"'")); err != nil {
		t.Fatalf("configured read root failed: %v", err)
	}
	raw, _ := os.ReadFile(outside)
	if string(raw) != "readable" {
		t.Fatalf("host file changed: %q", raw)
	}
}

func TestRuntimeStopsExcessiveOutputAndScrubsHostEnvironment(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires macOS sandbox-runtime")
	}
	t.Setenv("PPT_TEST_SECRET", "must-not-leak")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("BASH_ENV", "/host/startup-must-not-run")
	cfg := config.DefaultSandboxConfig()
	cfg.Execution.MaxOutputBytes = 1024
	runner := NewRunner(cfg)
	root, original := t.TempDir(), t.TempDir()
	result, err := runner.Execute(context.Background(), root, original, false, Evaluate("printf '%s|%s' \"${PPT_TEST_SECRET-unset}\" \"${BASH_ENV-unset}\""))
	if err != nil || result.Stdout != "unset|unset" {
		t.Fatalf("environment: %+v %v", result, err)
	}
	result, err = runner.Execute(context.Background(), root, original, true, Evaluate("yes output"))
	if err == nil || ErrorCode(err) != CodeOutputLimit || len(result.Stdout)+len(result.Stderr) > 1024 {
		t.Fatalf("output limit: %+v %v", result, err)
	}
}
