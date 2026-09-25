package service

import (
	"errors"
	"os"
	"path/filepath"
)

func atomicRewriteRepositoryFile(path string, raw []byte) error {
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	mode := os.FileMode(0644)
	if info != nil {
		mode = info.Mode().Perm()
	}
	if info != nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return ErrUnsafeRepositoryPath
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".repository-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err = tmp.Chmod(mode); err == nil {
		_, err = tmp.Write(raw)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
