package asset

import (
	"io/fs"
	"path"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/seed"
)

// kindDirs 是 4 类资产在 seed/assets 与 _assets 下的子目录名（与 kind 对齐，同构）。
var kindDirs = map[Kind]string{
	KindTheme:     "themes",
	KindLayout:    "layouts",
	KindComponent: "components",
	KindFx:        "fx",
}

// SeedFS 返回内嵌 seed 文件系统（供 seeding 与测试遍历）。根下含 assets/ 与 common/。
func SeedFS() fs.FS { return seed.FS() }

// SeedEntry 是一个 seed 资产条目（解析自内嵌 FS）。
type SeedEntry struct {
	Kind     Kind
	KindDir  string // themes|layouts|components|fx
	Name     string // 目录名（== manifest.name）
	Dir      string // 相对 seed 根：assets/<kindDir>/<name>
	Manifest Manifest
	RawJSON  []byte
}

// WalkSeedAssets 遍历内嵌 seed 的全部资产条目（不校验，仅解析 manifest）。
func WalkSeedAssets() ([]SeedEntry, error) {
	root := SeedFS()
	var out []SeedEntry
	for kind, dir := range kindDirs {
		base := path.Join("assets", dir)
		entries, err := fs.ReadDir(root, base)
		if err != nil {
			continue // 该 kind 目录不存在则跳过
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			assetDir := path.Join(base, name)
			raw, err := fs.ReadFile(root, path.Join(assetDir, "manifest.json"))
			if err != nil {
				continue // 无 manifest（如残留 .gitkeep）跳过
			}
			m, err := Parse(raw)
			if err != nil {
				return nil, err
			}
			out = append(out, SeedEntry{
				Kind: kind, KindDir: dir, Name: name, Dir: assetDir, Manifest: m, RawJSON: raw,
			})
		}
	}
	return out, nil
}

// ReadSeedFile 读取 seed 根下的相对文件（如资产载荷或 common/base.css）。
func ReadSeedFile(rel string) ([]byte, error) {
	return fs.ReadFile(SeedFS(), strings.TrimPrefix(rel, "/"))
}
