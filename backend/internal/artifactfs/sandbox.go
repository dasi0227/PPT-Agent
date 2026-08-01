package artifactfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sandbox confines artifact file operations to one authoritative root.
type Sandbox struct {
	root string
}

func NewSandbox(root string) (*Sandbox, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return &Sandbox{root: abs}, nil
}

func (s *Sandbox) Resolve(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes artifact root: absolute path %q not allowed", rel)
	}
	clean := filepath.Clean(filepath.Join(s.root, rel))
	resolved := clean
	if value, err := filepath.EvalSymlinks(clean); err == nil {
		resolved = value
	} else if parent, err := filepath.EvalSymlinks(filepath.Dir(clean)); err == nil {
		resolved = filepath.Join(parent, filepath.Base(clean))
	}
	if !within(s.root, resolved) {
		return "", fmt.Errorf("path escapes artifact root: %q", rel)
	}
	return clean, nil
}

func (s *Sandbox) Read(rel string) ([]byte, error) {
	abs, err := s.Resolve(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

func (s *Sandbox) Write(rel string, data []byte) error {
	abs, err := s.Resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o644)
}

func (s *Sandbox) Delete(rel string) error {
	abs, err := s.Resolve(rel)
	if err != nil {
		return err
	}
	return os.Remove(abs)
}

func within(root, target string) bool {
	return target == root || strings.HasPrefix(target, root+string(os.PathSeparator))
}
