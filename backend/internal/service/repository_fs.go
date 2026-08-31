package service

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxRepositoryEntries  = 64
	maxRepositoryFileSize = 64 << 10
)

var repositoryIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func validRepositoryID(id string) bool {
	return repositoryIDPattern.MatchString(id)
}

func readRepositoryFile(root, id, filename string, maxBytes int64) ([]byte, string, error) {
	if !validRepositoryID(id) || filepath.IsAbs(id) || strings.ContainsAny(id, `/\`) {
		return nil, "", ErrInvalidRepositoryID
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, "", err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, "", ErrUnsafeRepositoryPath
	}
	dir := filepath.Join(root, id)
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return nil, "", err
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		return nil, "", ErrUnsafeRepositoryPath
	}
	path := filepath.Join(dir, filename)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, "", ErrUnsafeRepositoryPath
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil, "", ErrRepositoryFileTooLarge
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, "", err
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, "", ErrUnsafeRepositoryPath
	}
	raw, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, "", err
	}
	if maxBytes > 0 && int64(len(raw)) > maxBytes {
		return nil, "", ErrRepositoryFileTooLarge
	}
	return raw, resolvedPath, nil
}

func deleteRepositoryDirectory(root, id string) error {
	if !validRepositoryID(id) || filepath.IsAbs(id) || strings.ContainsAny(id, `/\`) {
		return ErrInvalidRepositoryID
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return ErrUnsafeRepositoryPath
	}
	dir := filepath.Join(root, id)
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		return ErrUnsafeRepositoryPath
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedDir)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return ErrUnsafeRepositoryPath
	}
	return os.RemoveAll(resolvedDir)
}

func repositoryIDs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, min(len(entries), maxRepositoryEntries))
	for _, entry := range entries {
		if len(ids) >= maxRepositoryEntries {
			break
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !validRepositoryID(entry.Name()) {
			continue
		}
		ids = append(ids, entry.Name())
	}
	return ids, nil
}

func repositoryOpenURL(path string) string {
	return (&url.URL{Scheme: "vscode", Host: "file", Path: path}).String()
}

var (
	ErrInvalidRepositoryID    = errors.New("invalid repository id")
	ErrUnsafeRepositoryPath   = errors.New("unsafe repository path")
	ErrRepositoryFileTooLarge = errors.New("repository file exceeds 64KB")
	ErrRepositoryCorrupt      = errors.New("repository metadata is invalid")
)

func repositoryReadError(kind, id string, err error) error {
	return fmt.Errorf("%s %q: %w", kind, id, err)
}
