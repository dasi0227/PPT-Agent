package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/seed"
)

type Initializer struct {
	WorkRoot string
}

func (i Initializer) Initialize() error {
	if strings.TrimSpace(i.WorkRoot) == "" {
		return errors.New("work root is required")
	}
	for _, module := range []string{"themes", "components"} {
		sourceRoot := path.Join("assets", module)
		if err := fs.WalkDir(seed.FS(), sourceRoot, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative := strings.TrimPrefix(strings.TrimPrefix(sourcePath, sourceRoot), "/")
			if relative == "" {
				return nil
			}
			destination := filepath.Join(i.WorkRoot, "assets", module, filepath.FromSlash(relative))
			if entry.IsDir() {
				return os.MkdirAll(destination, 0o755)
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("seed symlink is not allowed: %s", sourcePath)
			}
			if _, err := os.Lstat(destination); err == nil {
				return nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			raw, err := fs.ReadFile(seed.FS(), sourcePath)
			if err != nil {
				return err
			}
			return writeMissingFile(destination, raw)
		}); err != nil {
			return err
		}
	}
	return os.MkdirAll(filepath.Join(i.WorkRoot, "skills"), 0o755)
}

func writeMissingFile(destination string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(destination)
	}
	return err
}
