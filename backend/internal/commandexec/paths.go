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
	lower := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	base := filepath.Base(lower)
	for _, pattern := range sensitiveFilePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	parts := strings.Split(lower, "/")
	for index, part := range parts {
		for _, directory := range sensitiveDirectories {
			if part == directory {
				return true
			}
		}
		if part == ".git" && index+1 < len(parts) && parts[index+1] == "config" {
			return true
		}
	}
	return false
}

var sensitiveFilePatterns = []string{
	".env", ".env.*", ".netrc", ".npmrc", ".pypirc", "*credential*", "*secret*",
	"*.pem", "*.key", "*.p12", "*.pfx",
}

var sensitiveDirectories = []string{".ssh", ".aws", ".kube"}

// Git can expose deleted or staged files that are absent from the working tree.
// Default/directory diffs exclude those paths using the same sensitivity rules.
func sensitiveGitExclusions() []string {
	paths := make([]string, 0, len(sensitiveFilePatterns)+len(sensitiveDirectories)+1)
	for _, pattern := range sensitiveFilePatterns {
		paths = append(paths, ":(exclude,icase,glob)**/"+pattern)
	}
	for _, directory := range sensitiveDirectories {
		paths = append(paths, ":(exclude,icase,glob)**/"+directory+"/**")
	}
	return append(paths, ":(exclude,icase,glob)**/.git/config")
}
