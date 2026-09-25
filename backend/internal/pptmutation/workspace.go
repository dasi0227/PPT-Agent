package pptmutation

import (
	"errors"
	"io/fs"
	"sort"
)

// Buffer makes every domain operation atomic even when it changes multiple
// resources. The adapter decides whether Commit targets run staging or the
// committed project filesystem.
type Buffer struct {
	base     Workspace
	writes   map[string][]byte
	deletes  map[string]bool
	rollback func() error
}

func NewBuffer(base Workspace) *Buffer {
	return &Buffer{base: base, writes: map[string][]byte{}, deletes: map[string]bool{}}
}
func (b *Buffer) Read(path string) ([]byte, error) {
	if b.deletes[path] {
		return nil, fs.ErrNotExist
	}
	if raw, ok := b.writes[path]; ok {
		return append([]byte(nil), raw...), nil
	}
	return b.base.Read(path)
}
func (b *Buffer) Write(path string, raw []byte) error {
	delete(b.deletes, path)
	b.writes[path] = append([]byte(nil), raw...)
	return nil
}
func (b *Buffer) Delete(path string) error {
	delete(b.writes, path)
	b.deletes[path] = true
	return nil
}
func (b *Buffer) HasChanges() bool {
	return len(b.writes) > 0 || len(b.deletes) > 0
}

func (b *Buffer) Commit() error {
	paths := make([]string, 0, len(b.writes))
	for path := range b.writes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	backups := map[string][]byte{}
	missing := map[string]bool{}
	for _, path := range paths {
		raw, err := b.base.Read(path)
		if errors.Is(err, fs.ErrNotExist) {
			missing[path] = true
		} else if err != nil {
			return err
		} else {
			backups[path] = raw
		}
	}
	for path := range b.deletes {
		raw, err := b.base.Read(path)
		if errors.Is(err, fs.ErrNotExist) {
			missing[path] = true
		} else if err != nil {
			return err
		} else {
			backups[path] = raw
		}
	}
	applied := []string{}
	b.rollback = func() error {
		var failures []error
		for i := len(applied) - 1; i >= 0; i-- {
			path := applied[i]
			var err error
			if missing[path] {
				err = b.base.Delete(path)
				if errors.Is(err, fs.ErrNotExist) {
					err = nil
				}
			} else {
				err = b.base.Write(path, backups[path])
			}
			if err != nil {
				failures = append(failures, err)
			}
		}
		return errors.Join(failures...)
	}
	for _, path := range paths {
		// A writer can replace the file before reporting a durability error.
		applied = append(applied, path)
		if err := b.base.Write(path, b.writes[path]); err != nil {
			return errors.Join(err, b.Rollback())
		}
	}
	deletePaths := make([]string, 0, len(b.deletes))
	for path := range b.deletes {
		deletePaths = append(deletePaths, path)
	}
	sort.Strings(deletePaths)
	for _, path := range deletePaths {
		applied = append(applied, path)
		if err := b.base.Delete(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return errors.Join(err, b.Rollback())
		}
	}
	return nil
}

// Rollback restores the preimages if a later metadata transaction fails. The
// caller must still hold the same project write lock used during Commit.
func (b *Buffer) Rollback() error {
	if b.rollback == nil {
		return nil
	}
	err := b.rollback()
	if err == nil {
		b.rollback = nil
	}
	return err
}
