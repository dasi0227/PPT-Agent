package artifactfs

import (
	"errors"
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
	abs, err = resolvePath(abs)
	if err != nil {
		return nil, err
	}
	return &Sandbox{root: abs}, nil
}

func (s *Sandbox) Resolve(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes artifact root: absolute path %q not allowed", rel)
	}
	clean := filepath.Clean(filepath.Join(s.root, rel))
	if !within(s.root, clean) {
		return "", fmt.Errorf("path escapes artifact root: %q", rel)
	}
	resolved, err := resolvePath(clean)
	if err != nil {
		return "", err
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
	// Readers must see either the previous complete document or the next one,
	// especially when multiple logical resources share the Spec collection.
	file, err := os.CreateTemp(filepath.Dir(abs), ".artifact-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0o644); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(file.Name(), abs); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(abs))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *Sandbox) Delete(rel string) error {
	abs, err := s.Resolve(rel)
	if err != nil {
		return err
	}
	return os.Remove(abs)
}

func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// Resolve the nearest existing ancestor, including symlinks above missing
// directories. Existing dangling links and other resolution errors fail closed.
func resolvePath(path string) (string, error) {
	ancestor := path
	missing := ""
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(ancestor)
			if err != nil {
				return "", err
			}
			return filepath.Join(resolved, missing), nil
		}
		if !errors.Is(err, os.ErrNotExist) || ancestor == filepath.Dir(ancestor) {
			return "", err
		}
		missing = filepath.Join(filepath.Base(ancestor), missing)
		ancestor = filepath.Dir(ancestor)
	}
}
