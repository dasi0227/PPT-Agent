package migrations

import (
	"embed"
	"io/fs"
	"sort"
)

//go:embed *.sql
var FS embed.FS

// Files 返回按文件名字典序排列的迁移文件名（保证执行顺序）。
func Files() ([]string, error) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}
