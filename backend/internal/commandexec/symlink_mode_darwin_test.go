package commandexec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyNodePreservesSymlinkModeWithoutChangingTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []uint32{0o777, 0o711} {
		if err := ApplyNode(root, "link", &Node{Kind: "symlink", Mode: mode, Content: []byte("target")}); err != nil {
			t.Fatal(err)
		}
		link, err := ReadNode(filepath.Join(root, "link"))
		if err != nil || link == nil || link.Kind != "symlink" || link.Mode != mode || string(link.Content) != "target" {
			t.Fatalf("symbolic-link metadata changed: %+v %v", link, err)
		}
		info, err := os.Stat(target)
		if err != nil || info.Mode().Perm() != 0o640 {
			t.Fatalf("link chmod changed its target: %v %v", info, err)
		}
	}
}
