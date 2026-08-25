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
	base    Workspace
	writes  map[string][]byte
	deletes map[string]bool
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
	rollback := func() {
		for i := len(applied) - 1; i >= 0; i-- {
			path := applied[i]
			if missing[path] {
				_ = b.base.Delete(path)
			} else {
				_ = b.base.Write(path, backups[path])
			}
		}
	}
	for _, path := range paths {
		if err := b.base.Write(path, b.writes[path]); err != nil {
			rollback()
			return err
		}
		applied = append(applied, path)
	}
	deletePaths := make([]string, 0, len(b.deletes))
	for path := range b.deletes {
		deletePaths = append(deletePaths, path)
	}
	sort.Strings(deletePaths)
	for _, path := range deletePaths {
		if err := b.base.Delete(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			rollback()
			return err
		}
		applied = append(applied, path)
	}
	return nil
}
