package commandexec

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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

func TestShellSandboxAllowsReadingButPreventsHostWrites(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires macOS Seatbelt")
	}
	root, outside := t.TempDir(), filepath.Join(t.TempDir(), "outside")
	_ = os.WriteFile(outside, []byte("readable"), 0600)
	executor, err := NewExecutor(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"cat '" + outside + "'", "x=hello; printf '%s' \"$x\" > local; cat local"} {
		result, err := executor.Execute(context.Background(), Evaluate(source), nil)
		if err != nil {
			t.Fatalf("%s: %v, stderr=%s", source, err, result.Stderr)
		}
	}
	for _, source := range []string{"printf changed > '" + outside + "'", "ln '" + outside + "' alias && printf changed > alias", "printf changed > .outline.json"} {
		if _, err := executor.Execute(context.Background(), Evaluate(source), nil); err == nil {
			t.Fatalf("prohibited write succeeded: %s", source)
		}
	}
	raw, _ := os.ReadFile(outside)
	if string(raw) != "readable" {
		t.Fatalf("host file changed: %q", raw)
	}
}
