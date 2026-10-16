package commandexec

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// Node retains filesystem semantics, including executable bits, empty directories
// and internal symbolic links. Snapshots never traverse symbolic links.
type Node struct {
	Kind    string `json:"kind"`
	Mode    uint32 `json:"mode"`
	Content []byte `json:"content,omitempty"`
}

type Tree map[string]Node

func (n Node) Hash() string { data, _ := json.Marshal(n); return ContentHash(data) }
func (t Tree) Hash() string { data, _ := json.Marshal(t); return ContentHash(data) }

func ReadNode(path string) (*Node, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	n := &Node{Mode: uint32(info.Mode().Perm())}
	switch {
	case info.IsDir():
		n.Kind = "directory"
	case info.Mode().IsRegular():
		n.Kind = "file"
		n.Content, err = os.ReadFile(path)
	case info.Mode()&os.ModeSymlink != 0:
		n.Kind = "symlink"
		var link string
		link, err = os.Readlink(path)
		n.Content = []byte(link)
	default:
		return nil, commandError(CodeNotAllowed, "Special filesystem nodes are not supported: "+path)
	}
	return n, err
}

func Snapshot(root string) (Tree, error) {
	out := Tree{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == ".runtime" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		node, err := ReadNode(path)
		if err != nil {
			return err
		}
		if node == nil {
			return fmt.Errorf("project changed during snapshot: %s", relative)
		}
		out[relative] = *node
		return nil
	})
	return out, err
}

func (t Tree) CopyTo(root, originalRoot string) error {
	paths := make([]string, 0, len(t))
	for path := range t {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		node := t[path]
		if node.Kind == "symlink" && filepath.IsAbs(string(node.Content)) {
			relative, err := filepath.Rel(originalRoot, string(node.Content))
			if err == nil && within(relative) {
				link, _ := filepath.Rel(filepath.Dir(path), relative)
				node.Content = []byte(link)
			}
		}
		if node.Kind == "directory" {
			node.Mode |= 0o700
		}
		if err := ApplyNode(root, path, &node); err != nil {
			return err
		}
	}
	for i := len(paths) - 1; i >= 0; i-- {
		node := t[paths[i]]
		if node.Kind == "directory" {
			if err := os.Chmod(filepath.Join(root, paths[i]), fs.FileMode(node.Mode)); err != nil {
				return err
			}
		}
	}
	return nil
}

func within(path string) bool {
	return !filepath.IsAbs(path) && path != ".." && !strings.HasPrefix(path, "../")
}

func ProtectedPath(path string) bool {
	path = strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	for _, root := range []string{".git", ".runtime", ".run", ".commit-tmp", "versions"} {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return path == ".manifest.json" || path == ".design.json" || path == ".outline.json"
}

func ValidateLink(root, path string, node Node) error {
	if node.Kind != "symlink" {
		return nil
	}
	target := string(node.Content)
	if filepath.IsAbs(target) {
		return commandError(CodeNotAllowed, "New symbolic links must use project-relative targets: "+path)
	}
	relative := filepath.Clean(filepath.Join(filepath.Dir(path), target))
	if !within(relative) || ProtectedPath(relative) {
		return commandError(CodeNotAllowed, "Symbolic link escapes the project or targets protected data: "+path)
	}
	full := filepath.Join(root, relative)
	// Resolve the closest existing ancestor as well as complete link chains.
	for {
		resolved, err := filepath.EvalSymlinks(full)
		if err == nil {
			rel, _ := filepath.Rel(root, resolved)
			if !within(rel) || ProtectedPath(rel) {
				return commandError(CodeNotAllowed, "Symbolic link targets data outside the permitted workspace: "+path)
			}
			return nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(full)
		if parent == full {
			return err
		}
		full = parent
	}
}

// ApplyNode never follows an existing symlink at the destination or in a parent.
// Removal is intentionally nonrecursive; callers order child deletions first.
func ApplyNode(root, relative string, node *Node) error {
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return commandError(CodeNotAllowed, "Workspace root must be absolute")
	}
	relative = filepath.Clean(relative)
	if relative == "." || !within(relative) {
		return commandError(CodeNotAllowed, "Invalid workspace path")
	}
	path := filepath.Join(root, relative)
	for parent := filepath.Dir(path); parent != root; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err == nil && !info.IsDir() {
			return commandError(CodeNotAllowed, "Workspace parent is not a directory: "+parent)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	old, err := ReadNode(path)
	if err != nil {
		return err
	}
	if node == nil {
		if old == nil {
			return nil
		}
		return os.Remove(path)
	}
	if old != nil && old.Kind != node.Kind {
		if err := os.Remove(path); err != nil {
			return err
		}
		old = nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	switch node.Kind {
	case "directory":
		if err := os.MkdirAll(path, fs.FileMode(node.Mode)); err != nil {
			return err
		}
		return os.Chmod(path, fs.FileMode(node.Mode))
	case "symlink":
		if old != nil {
			if string(old.Content) == string(node.Content) {
				return nil
			}
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return os.Symlink(string(node.Content), path)
	case "file":
		file, err := os.CreateTemp(filepath.Dir(path), ".command-write-*")
		if err != nil {
			return err
		}
		temporary := file.Name()
		defer os.Remove(temporary)
		if err = file.Chmod(fs.FileMode(node.Mode)); err == nil {
			_, err = file.Write(node.Content)
		}
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		return os.Rename(temporary, path)
	default:
		return commandError(CodeNotAllowed, "Unsupported workspace node")
	}
}

// OrderedChanges removes old nodes deepest-first, then creates parents before children.
func OrderedChanges(nodes map[string]*Node) []string {
	paths := make([]string, 0, len(nodes))
	for path := range nodes {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		a, b := nodes[paths[i]], nodes[paths[j]]
		if (a == nil) != (b == nil) {
			return a == nil
		}
		if a == nil {
			return paths[i] > paths[j]
		}
		return paths[i] < paths[j]
	})
	return paths
}

// A command may make its own directories read-only. Restore owner traversal
// before cleanup; WalkDir never follows links into the live project.
func RemoveWorkspace(root string) {
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if entry != nil && entry.IsDir() {
			_ = os.Chmod(path, 0700)
		}
		return nil
	})
	_ = os.RemoveAll(root)
}
