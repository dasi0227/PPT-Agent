package artifactfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandboxRejectsSymlinkEscapeAboveMissingDirectories(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "redirect")); err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"redirect/probe.txt", "redirect/new/inside/probe.txt", "../outside.txt"} {
		if err := sandbox.Write(path, []byte("dummy")); err == nil {
			t.Fatalf("accepted escaping write: %s", path)
		}
		if _, err := sandbox.Read(path); err == nil {
			t.Fatalf("accepted escaping read: %s", path)
		}
		if err := sandbox.Delete(path); err == nil {
			t.Fatalf("accepted escaping delete: %s", path)
		}
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside directory changed: entries=%v err=%v", entries, err)
	}
}

func TestSandboxResolvesNewRootsAndInternalLinks(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "new", "artifacts")
	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := sandbox.Write("nested/inside/probe.txt", []byte("dummy")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("nested", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if raw, err := sandbox.Read("alias/inside/probe.txt"); err != nil || string(raw) != "dummy" {
		t.Fatalf("internal link read: raw=%q err=%v", raw, err)
	}
	for _, name := range []string{"dangling", "loop"} {
		target := "missing"
		if name == "loop" {
			target = name
		}
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
		if _, err := sandbox.Resolve(name + "/new/probe.txt"); err == nil {
			t.Fatalf("accepted unresolved link: %s", name)
		}
	}
}
