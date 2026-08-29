package commandexec

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type PathGuard struct {
	root string
}

func NewPathGuard(root string) (*PathGuard, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return nil, commandError(CodePathInvalid, "project root must be an existing directory")
	}
	return &PathGuard{root: filepath.Clean(canonical)}, nil
}

func (g *PathGuard) Root() string {
	return g.root
}

func (g *PathGuard) Validate(path string, requireRegular bool) (string, error) {
	if strings.ContainsRune(path, 0) || filepath.IsAbs(path) {
		return "", commandError(CodePathOutsideProject, "absolute and NUL-containing paths are not allowed")
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return "", commandError(CodePathOutsideProject, "parent path traversal is not allowed")
		}
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "" {
		clean = "."
	}
	full := filepath.Join(g.root, clean)
	if linkInfo, linkErr := os.Lstat(full); linkErr == nil && linkInfo.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(full)
		if readErr != nil {
			return "", commandError(CodePathInvalid, readErr.Error())
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(full), target)
		}
		if !g.contains(target) {
			return "", commandError(CodePathOutsideProject, "path resolves outside the project")
		}
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", commandError(CodePathInvalid, err.Error())
		}
		parent, base := filepath.Dir(full), filepath.Base(full)
		for {
			resolvedParent, parentErr := filepath.EvalSymlinks(parent)
			if parentErr == nil {
				resolved = filepath.Join(resolvedParent, base)
				break
			}
			if !errors.Is(parentErr, fs.ErrNotExist) || parent == filepath.Dir(parent) {
				return "", commandError(CodePathInvalid, "path parent cannot be resolved")
			}
			base = filepath.Join(filepath.Base(parent), base)
			parent = filepath.Dir(parent)
		}
	}
	if !g.contains(resolved) {
		return "", commandError(CodePathOutsideProject, "path resolves outside the project")
	}
	if info, statErr := os.Stat(resolved); statErr == nil {
		if requireRegular && !info.Mode().IsRegular() {
			return "", commandError(CodePathInvalid, "path must name a regular file")
		}
		if info.Mode()&(os.ModeDevice|os.ModeNamedPipe|os.ModeSocket|os.ModeCharDevice) != 0 {
			return "", commandError(CodePathInvalid, "special files are not supported")
		}
	} else if requireRegular {
		return "", commandError(CodePathInvalid, "path must name an existing regular file")
	}
	relative, err := filepath.Rel(g.root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", commandError(CodePathOutsideProject, "path resolves outside the project")
	}
	return filepath.ToSlash(relative), nil
}

func (g *PathGuard) contains(path string) bool {
	path = filepath.Clean(path)
	return path == g.root || strings.HasPrefix(path, g.root+string(filepath.Separator))
}

func IsSensitivePath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	if base == ".env" || strings.HasPrefix(base, ".env.") ||
		base == ".netrc" || base == ".npmrc" || base == ".pypirc" ||
		strings.Contains(base, "credential") || strings.Contains(base, "secret") ||
		strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") ||
		strings.HasSuffix(base, ".p12") || strings.HasSuffix(base, ".pfx") {
		return true
	}
	return strings.HasPrefix(lower, ".ssh/") ||
		strings.HasPrefix(lower, ".aws/") ||
		strings.HasPrefix(lower, ".kube/") ||
		lower == ".git/config"
}
