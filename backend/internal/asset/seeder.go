package asset

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// SeedStore 是 seeding 落库所需的最小持久化能力（消费方定义接口，避免反向依赖）。
type SeedStore interface {
	UpsertAsset(ctx context.Context, a model.Asset) error
	CountAssets(ctx context.Context) (int, error)
}

// Seeder 把内嵌 seed 资产载入 work_root 的 _assets/（同构映射）并 upsert SQLite（DS-SEED-001/002/005）。
type Seeder struct {
	store    SeedStore
	workRoot string
	clock    func() int64
	newID    func() string
}

// NewSeeder 构造 seeder。clock/newID 可注入以便测试确定性；nil 用默认。
func NewSeeder(store SeedStore, workRoot string, clock func() int64, newID func() string) *Seeder {
	if clock == nil {
		clock = func() int64 { return 0 }
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &Seeder{store: store, workRoot: workRoot, clock: clock, newID: newID}
}

// Seed 执行冷启动 seeding：
//  1. 拷贝 common/ 基座（base.css）到 work_root/_assets/common/（供生成期取用）。
//  2. 逐资产：schema 校验（不过则跳过并报错）→ 拷贝载荷到 _assets/<kind_dir>/<id>/ → upsert SQLite。
//
// 幂等：目录与 SQLite 均按 (name,kind) 覆盖，可重复执行（DS-SEED-005）。
func (s *Seeder) Seed(ctx context.Context) error {
	if err := s.seedCommon(); err != nil {
		return err
	}
	entries, err := WalkSeedAssets()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ValidateManifestRaw(e.RawJSON); err != nil {
			return fmt.Errorf("seed %s/%s manifest invalid: %w", e.KindDir, e.Name, err)
		}
		if e.Manifest.Kind == KindTheme {
			css, err := ReadSeedFile(path.Join(e.Dir, e.Manifest.Assets.Tokens))
			if err != nil {
				return fmt.Errorf("seed theme %s read tokens: %w", e.Name, err)
			}
			if miss := ValidateThemeTokens(css); len(miss) != 0 {
				return fmt.Errorf("seed theme %s missing tokens: %v", e.Name, miss)
			}
		}

		// _assets/<kind_dir>/<name>（资产 id 用 name，保证幂等与同构，DS-SEED 直接映射）。
		relDir := path.Join("_assets", e.KindDir, e.Name)
		destDir := filepath.Join(s.workRoot, filepath.FromSlash(relDir))
		if err := s.copyAssetDir(e.Dir, destDir); err != nil {
			return fmt.Errorf("seed copy %s: %w", e.Name, err)
		}

		now := s.clock()
		a := model.Asset{
			ID:           s.newID(),
			Name:         e.Manifest.Name,
			Kind:         string(e.Manifest.Kind),
			Version:      e.Manifest.Version,
			Source:       string(SourcePreset),
			Description:  e.Manifest.Description,
			Tags:         e.Manifest.Tags,
			ManifestPath: path.Join(relDir, "manifest.json"),
			Dir:          relDir,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := s.store.UpsertAsset(ctx, a); err != nil {
			return fmt.Errorf("seed upsert %s: %w", e.Name, err)
		}
	}
	return nil
}

// seedCommon 拷贝 common/ 基座到 _assets/common/（base.css 主题无关，生成期取用）。
func (s *Seeder) seedCommon() error {
	root := SeedFS()
	return fs.WalkDir(root, "common", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(root, p)
		if err != nil {
			return err
		}
		dest := filepath.Join(s.workRoot, "_assets", filepath.FromSlash(p))
		return writeFile(dest, data)
	})
}

// copyAssetDir 递归拷贝内嵌 seed 资产目录到 destDir。
func (s *Seeder) copyAssetDir(seedDir, destDir string) error {
	root := SeedFS()
	return fs.WalkDir(root, seedDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(root, p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.FromSlash(seedDir), filepath.FromSlash(p))
		if err != nil {
			return err
		}
		return writeFile(filepath.Join(destDir, rel), data)
	})
}

func writeFile(dest string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}
