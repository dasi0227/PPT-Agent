package commandexec

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCandidateMetadataRejectsUnrepresentableAttributes(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes")
	if err := os.WriteFile(file, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCandidateMetadata(root); err != nil {
		t.Fatalf("ordinary filesystem provenance rejected: %v", err)
	}
	for _, path := range []string{root, file} {
		if err := unix.Setxattr(path, "user.ppt-test", []byte("uncommittable"), 0); err != nil {
			t.Fatal(err)
		}
		if err := ValidateCandidateMetadata(root); ErrorCode(err) != CodeNotAllowed {
			t.Fatalf("custom metadata was accepted for %s: %v", path, err)
		}
		if err := unix.Removexattr(path, "user.ppt-test"); err != nil {
			t.Fatal(err)
		}
	}
}
