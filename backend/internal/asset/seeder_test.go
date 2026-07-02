package asset

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// memSeedStore 是 SeedStore 的内存实现（不打真实 SQLite）。
type memSeedStore struct {
	assets map[string]model.Asset // key: name|kind
}

func newMemSeedStore() *memSeedStore { return &memSeedStore{assets: map[string]model.Asset{}} }

func (m *memSeedStore) UpsertAsset(_ context.Context, a model.Asset) error {
	m.assets[a.Name+"|"+a.Kind] = a
	return nil
}
func (m *memSeedStore) CountAssets(_ context.Context) (int, error) { return len(m.assets), nil }

// AC-SEED-001：全新环境首次 seeding → _assets 含全部 seed 资产，SQLite 记录 source=preset。
func TestSeedLoad(t *testing.T) {
	workRoot := t.TempDir()
	store := newMemSeedStore()
	seeder := NewSeeder(store, workRoot, func() int64 { return 100 }, seqIDs())

	if err := seeder.Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	entries, err := WalkSeedAssets()
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(store.assets) != len(entries) {
		t.Fatalf("store has %d assets, seed source has %d", len(store.assets), len(entries))
	}

	for _, e := range entries {
		a, ok := store.assets[string(e.Manifest.Name)+"|"+string(e.Manifest.Kind)]
		if !ok {
			t.Errorf("asset %s/%s not upserted", e.KindDir, e.Name)
			continue
		}
		if a.Source != "preset" {
			t.Errorf("asset %s source=%q, want preset", e.Name, a.Source)
		}
		// 载荷落盘到 _assets/<kind_dir>/<name>/manifest.json。
		mp := filepath.Join(workRoot, filepath.FromSlash(a.ManifestPath))
		if _, err := os.Stat(mp); err != nil {
			t.Errorf("manifest not written for %s: %v", e.Name, err)
		}
	}

	// common/base.css 拷入 _assets/common/。
	if _, err := os.Stat(filepath.Join(workRoot, "_assets", "common", "base.css")); err != nil {
		t.Errorf("common/base.css not seeded: %v", err)
	}
}

// 幂等：重复 seeding 不重复膨胀（DS-SEED-005）。
func TestSeedIdempotent(t *testing.T) {
	workRoot := t.TempDir()
	store := newMemSeedStore()
	seeder := NewSeeder(store, workRoot, func() int64 { return 100 }, seqIDs())

	if err := seeder.Seed(context.Background()); err != nil {
		t.Fatalf("seed 1: %v", err)
	}
	n1 := len(store.assets)
	if err := seeder.Seed(context.Background()); err != nil {
		t.Fatalf("seed 2: %v", err)
	}
	if len(store.assets) != n1 {
		t.Errorf("idempotent seeding grew store: %d -> %d", n1, len(store.assets))
	}
}

func seqIDs() func() string {
	n := 0
	return func() string { n++; return "seed-id-" + itoa(n) }
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
