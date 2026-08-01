package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

var errInjected = errors.New("injected failure")

type failingAssetStore struct {
	*sqlitestore.Store
	failCreateVersion bool
	failDeleteAsset   bool
	createAssetErr    error
	upsertAssetErr    error
}

func (s *failingAssetStore) CreateAsset(ctx context.Context, a model.Asset) error {
	if s.createAssetErr != nil {
		return s.createAssetErr
	}
	return s.Store.CreateAsset(ctx, a)
}

func (s *failingAssetStore) CreateVersion(ctx context.Context, v model.Version) error {
	if s.failCreateVersion {
		return errInjected
	}
	return s.Store.CreateVersion(ctx, v)
}

func (s *failingAssetStore) UpsertAsset(ctx context.Context, a model.Asset) error {
	if s.upsertAssetErr != nil {
		return s.upsertAssetErr
	}
	return s.Store.UpsertAsset(ctx, a)
}

func (s *failingAssetStore) DeleteAsset(ctx context.Context, id string) error {
	if s.failDeleteAsset {
		return errInjected
	}
	return s.Store.DeleteAsset(ctx, id)
}

func newAssetServiceForTest(t *testing.T) (*AssetService, *sqlitestore.Store, string) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{DBPath: filepath.Join(root, "assets.db"), WorkRoot: root}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	svc := NewAssetService(st, root)
	svc.clock = func() int64 { return 123 }
	seq := 0
	svc.newID = func() string {
		seq++
		return "id-" + string(rune('0'+seq))
	}
	return svc, st, root
}

func componentManifest(name string) asset.Manifest {
	return asset.Manifest{
		Name:        name,
		Version:     "1.0.0",
		Kind:        asset.KindComponent,
		Description: "卡片组件",
		Tags:        []string{"card"},
		Mount:       &asset.Mount{Target: ".slide-content", Position: "append"},
		Params: map[string]asset.Param{
			"title": {Type: "string", Default: "标题"},
		},
		Assets: asset.Payload{HTML: "template.html", CSS: "style.css"},
	}
}

func TestAssetServiceCreatePatchDeleteVersions(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)

	a, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("neon-card"),
		Payload: map[string]string{
			"template.html": `<div class="neon-card">标题</div>`,
			"style.css":     `.neon-card{border-radius: 8px;color:var(--color-primary);}`,
		},
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if a.Source != string(asset.SourceUser) {
		t.Fatalf("create must force source=user, got %q", a.Source)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(a.ManifestPath))); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	versions, err := st.ListVersions(ctx, "asset", "asset-"+a.ID)
	if err != nil {
		t.Fatalf("list versions after create: %v", err)
	}
	if len(versions) != 1 || versions[0].VersionNo != 0 {
		t.Fatalf("create should produce asset v0, got %+v", versions)
	}

	patched, err := svc.PatchAsset(ctx, a.ID, PatchAssetParams{Edits: []AssetPatchEdit{{
		File: "style.css", OldText: "border-radius: 8px", NewText: "border-radius: 24px",
	}}})
	if err != nil {
		t.Fatalf("patch asset: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(patched.Dir), "style.css"))
	if !strings.Contains(string(raw), "24px") {
		t.Fatalf("payload not patched: %s", raw)
	}
	versions, _ = st.ListVersions(ctx, "asset", "asset-"+a.ID)
	if len(versions) != 2 || versions[1].VersionNo != 1 {
		t.Fatalf("patch should append asset v1, got %+v", versions)
	}

	if err := svc.DeleteAsset(ctx, a.ID); err != nil {
		t.Fatalf("delete user asset: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(a.Dir))); !os.IsNotExist(err) {
		t.Fatalf("asset dir should be removed, stat err=%v", err)
	}
	versions, _ = st.ListVersions(ctx, "asset", "asset-"+a.ID)
	if len(versions) != 3 || versions[2].VersionNo != 2 {
		t.Fatalf("delete should append asset v2 snapshot, got %+v", versions)
	}
}

func TestAssetServiceCreateSnapshotFailureRollsBackVisibleAsset(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)
	svc.store = &failingAssetStore{Store: st, failCreateVersion: true}

	_, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("snapshot-fails"),
		Payload: map[string]string{
			"template.html": `<div class="snapshot-fails">标题</div>`,
			"style.css":     `.snapshot-fails{color:var(--color-primary);}`,
		},
	})
	if !errors.Is(err, errInjected) {
		t.Fatalf("expected injected snapshot error, got %v", err)
	}
	assets, err := st.ListAssets(ctx, "component")
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	for _, a := range assets {
		if a.Name == "snapshot-fails" {
			t.Fatalf("asset row must be rolled back on snapshot failure: %+v", a)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "_assets/components/snapshot-fails")); !os.IsNotExist(err) {
		t.Fatalf("asset dir must be removed on snapshot failure, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "versions/asset-id-1/v0")); !os.IsNotExist(err) {
		t.Fatalf("snapshot dir must be removed on snapshot failure, stat err=%v", err)
	}
}

func TestAssetServiceCreateInvalidExtraPayloadPathCleansDirectory(t *testing.T) {
	ctx := context.Background()
	svc, _, root := newAssetServiceForTest(t)

	_, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("extra-path-fails"),
		Payload: map[string]string{
			"template.html": `<div class="extra-path-fails">标题</div>`,
			"style.css":     `.extra-path-fails{color:var(--color-primary);}`,
			"../evil.css":   `body{color:red;}`,
		},
	})
	if err == nil {
		t.Fatal("expected invalid extra payload path to fail")
	}
	if _, err := os.Stat(filepath.Join(root, "_assets/components/extra-path-fails")); !os.IsNotExist(err) {
		t.Fatalf("asset dir must be removed when payload write validation fails, stat err=%v", err)
	}
}

func TestAssetServiceDeleteDBFailureKeepsDirectoryForRetry(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)
	a, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("delete-db-fails"),
		Payload: map[string]string{
			"template.html": `<div class="delete-db-fails">标题</div>`,
			"style.css":     `.delete-db-fails{color:var(--color-primary);}`,
		},
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	svc.store = &failingAssetStore{Store: st, failDeleteAsset: true}

	if err := svc.DeleteAsset(ctx, a.ID); !errors.Is(err, errInjected) {
		t.Fatalf("expected injected delete error, got %v", err)
	}
	if _, err := st.GetAsset(ctx, a.ID); err != nil {
		t.Fatalf("asset row must remain when DB delete fails: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(a.Dir))); err != nil {
		t.Fatalf("asset dir must remain when DB delete fails: %v", err)
	}
}

func TestAssetServicePatchSnapshotFailureRestoresPayload(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)
	a, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("patch-snapshot-fails"),
		Payload: map[string]string{
			"template.html": `<div class="patch-snapshot-fails">标题</div>`,
			"style.css":     `.patch-snapshot-fails{border-radius: 8px;color:var(--color-primary);}`,
		},
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	svc.store = &failingAssetStore{Store: st, failCreateVersion: true}

	_, err = svc.PatchAsset(ctx, a.ID, PatchAssetParams{Edits: []AssetPatchEdit{{
		File: "style.css", OldText: "border-radius: 8px", NewText: "border-radius: 24px",
	}}})
	if !errors.Is(err, errInjected) {
		t.Fatalf("expected injected snapshot error, got %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Dir), "style.css"))
	if strings.Contains(string(raw), "24px") || !strings.Contains(string(raw), "8px") {
		t.Fatalf("payload must be restored on snapshot failure: %s", raw)
	}
}

func TestAssetServicePatchUpsertFailureRestoresPayload(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)
	a, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("patch-upsert-fails"),
		Payload: map[string]string{
			"template.html": `<div class="patch-upsert-fails">标题</div>`,
			"style.css":     `.patch-upsert-fails{border-radius: 8px;color:var(--color-primary);}`,
		},
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	svc.store = &failingAssetStore{Store: st, upsertAssetErr: errInjected}

	_, err = svc.PatchAsset(ctx, a.ID, PatchAssetParams{Edits: []AssetPatchEdit{{
		File: "style.css", OldText: "border-radius: 8px", NewText: "border-radius: 24px",
	}}})
	if !errors.Is(err, errInjected) {
		t.Fatalf("expected injected upsert error, got %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Dir), "style.css"))
	if strings.Contains(string(raw), "24px") || !strings.Contains(string(raw), "8px") {
		t.Fatalf("payload must be restored on upsert failure: %s", raw)
	}
}

func TestAssetServiceDuplicateCreateReturnsConflict(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAssetServiceForTest(t)
	params := CreateAssetParams{
		Manifest: componentManifest("dupe-card"),
		Payload: map[string]string{
			"template.html": `<div class="dupe-card">标题</div>`,
			"style.css":     `.dupe-card{color:var(--color-primary);}`,
		},
	}
	if _, err := svc.CreateAsset(ctx, params); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := svc.CreateAsset(ctx, params); !errors.Is(err, ErrAssetConflict) {
		t.Fatalf("second create should be conflict, got %v", err)
	}
}

func TestAssetServiceUniqueConstraintCreateMapsConflictAndCleansDirectory(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)
	svc.store = &failingAssetStore{
		Store:          st,
		createAssetErr: errors.New("constraint failed: UNIQUE constraint failed: assets.name, assets.kind"),
	}
	_, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("race-card"),
		Payload: map[string]string{
			"template.html": `<div class="race-card">标题</div>`,
			"style.css":     `.race-card{color:var(--color-primary);}`,
		},
	})
	if !errors.Is(err, ErrAssetConflict) {
		t.Fatalf("unique constraint should map to ErrAssetConflict, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "_assets/components/race-card")); !os.IsNotExist(err) {
		t.Fatalf("asset dir must be removed after unique constraint failure, stat err=%v", err)
	}
}

func TestAssetServiceRejectsEscapingPayloadPaths(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAssetServiceForTest(t)
	m := componentManifest("escape-card")
	m.Assets.HTML = "../template.html"
	if _, err := svc.CreateAsset(ctx, CreateAssetParams{Manifest: m, Payload: map[string]string{"../template.html": "x", "style.css": "y"}}); err == nil {
		t.Fatal("expected escaping payload path to be rejected")
	} else if !IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAssetDirectoryHelpersStayInsideSandbox(t *testing.T) {
	root := t.TempDir()
	sb, err := artifactfs.NewSandbox(root)
	if err != nil {
		t.Fatalf("sandbox: %v", err)
	}
	outside := filepath.Join(filepath.Dir(root), "outside-service-asset")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write outside marker: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	writeAssetFile(t, root, "src/manifest.json", "{}")

	if err := copyDir(sb, "../src", "dst"); err == nil {
		t.Fatal("copyDir should reject escaping source")
	}
	if err := copyDir(sb, "src", "../dst"); err == nil {
		t.Fatal("copyDir should reject escaping destination")
	}
	if err := copyDir(sb, "missing", "dst"); err == nil {
		t.Fatal("copyDir should fail for missing source")
	}
	if err := copyDir(sb, "src", "deep/parent/dst"); err != nil {
		t.Fatalf("copyDir should create missing target parents: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "deep/parent/dst/manifest.json")); err != nil {
		t.Fatalf("copied file missing: %v", err)
	}
	if err := removeDir(sb, "../outside-service-asset"); err == nil {
		t.Fatal("removeDir should reject escaping relative path")
	}
	if err := removeDir(sb, outside); err == nil {
		t.Fatal("removeDir should reject absolute path")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside marker must not be removed: %v", err)
	}
}

func TestAssetServiceValidationAndPresetProtection(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)

	bad := componentManifest("bad-card")
	bad.Assets.HTML = ""
	if _, err := svc.CreateAsset(ctx, CreateAssetParams{Manifest: bad, Payload: map[string]string{}}); err == nil {
		t.Fatal("expected manifest validation failure")
	} else if !IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}

	presetDir := "_assets/components/preset-card"
	writeAssetFile(t, root, presetDir+"/manifest.json", `{
  "name":"preset-card","version":"1.0.0","kind":"component","source":"preset",
  "description":"preset","mount":{"position":"append"},"assets":{"html":"template.html","css":"style.css"}
}`)
	writeAssetFile(t, root, presetDir+"/template.html", `<div class="preset-card">出厂态</div>`)
	writeAssetFile(t, root, presetDir+"/style.css", `.preset-card{border-radius: 8px;color:var(--color-primary);}`)
	preset := model.Asset{
		ID: "preset-id", Name: "preset-card", Kind: "component", Source: "preset",
		Description: "preset", ManifestPath: presetDir + "/manifest.json", Dir: presetDir,
		CreatedAt: 1, UpdatedAt: 1,
	}
	if err := st.UpsertAsset(ctx, preset); err != nil {
		t.Fatalf("seed preset: %v", err)
	}
	if err := svc.DeleteAsset(ctx, preset.ID); err == nil {
		t.Fatal("expected preset delete to be rejected")
	} else if !IsPresetDeleteError(err) {
		t.Fatalf("expected preset delete error, got %v", err)
	}
	if _, err := st.GetAsset(ctx, preset.ID); err != nil {
		t.Fatalf("preset row should remain after rejected delete: %v", err)
	}
}

func TestAssetServiceRollbackPresetToFactorySnapshot(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)
	presetDir := "_assets/components/preset-card"
	writeAssetFile(t, root, presetDir+"/manifest.json", `{
  "name":"preset-card","version":"1.0.0","kind":"component","source":"preset",
  "description":"preset","mount":{"position":"append"},"assets":{"html":"template.html","css":"style.css"}
}`)
	writeAssetFile(t, root, presetDir+"/template.html", `<div class="preset-card">出厂态</div>`)
	writeAssetFile(t, root, presetDir+"/style.css", `.preset-card{border-radius: 8px;color:var(--color-primary);}`)
	preset := model.Asset{
		ID: "preset-id", Name: "preset-card", Kind: "component", Source: "preset",
		Description: "preset", ManifestPath: presetDir + "/manifest.json", Dir: presetDir,
		CreatedAt: 1, UpdatedAt: 1,
	}
	if err := st.UpsertAsset(ctx, preset); err != nil {
		t.Fatalf("seed preset: %v", err)
	}

	if _, err := svc.PatchAsset(ctx, preset.ID, PatchAssetParams{Edits: []AssetPatchEdit{{
		File: "style.css", OldText: "border-radius: 8px", NewText: "border-radius: 24px",
	}}}); err != nil {
		t.Fatalf("patch preset: %v", err)
	}
	versions, _ := st.ListVersions(ctx, "asset", "asset-"+preset.ID)
	if len(versions) != 2 || versions[0].VersionNo != 0 || versions[1].VersionNo != 1 {
		t.Fatalf("first preset patch should preserve factory v0 and patched v1, got %+v", versions)
	}

	if _, err := svc.RollbackAsset(ctx, preset.ID, 0); err != nil {
		t.Fatalf("rollback preset: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(presetDir), "style.css"))
	if !strings.Contains(string(raw), "border-radius: 8px") {
		t.Fatalf("preset should be restored to factory snapshot: %s", raw)
	}
	versions, _ = st.ListVersions(ctx, "asset", "asset-"+preset.ID)
	if len(versions) != 3 || versions[2].VersionNo != 2 {
		t.Fatalf("rollback should append v2, got %+v", versions)
	}
}

func TestAssetServiceRollbackSnapshotFailureRestoresCurrentPayload(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)
	a, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("rollback-snapshot-fails"),
		Payload: map[string]string{
			"template.html": `<div class="rollback-snapshot-fails">标题</div>`,
			"style.css":     `.rollback-snapshot-fails{border-radius: 8px;color:var(--color-primary);}`,
		},
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if _, err := svc.PatchAsset(ctx, a.ID, PatchAssetParams{Edits: []AssetPatchEdit{{
		File: "style.css", OldText: "border-radius: 8px", NewText: "border-radius: 24px",
	}}}); err != nil {
		t.Fatalf("patch asset: %v", err)
	}
	svc.store = &failingAssetStore{Store: st, failCreateVersion: true}

	_, err = svc.RollbackAsset(ctx, a.ID, 0)
	if !errors.Is(err, errInjected) {
		t.Fatalf("expected injected snapshot error, got %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Dir), "style.css"))
	if strings.Contains(string(raw), "8px") || !strings.Contains(string(raw), "24px") {
		t.Fatalf("payload must be restored when rollback snapshot fails: %s", raw)
	}
	versions, _ := st.ListVersions(ctx, "asset", "asset-"+a.ID)
	if len(versions) != 2 {
		t.Fatalf("failed rollback must not append version rows, got %+v", versions)
	}
}

func TestAssetServiceRollbackUpsertFailureRestoresCurrentPayload(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)
	a, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("rollback-upsert-fails"),
		Payload: map[string]string{
			"template.html": `<div class="rollback-upsert-fails">标题</div>`,
			"style.css":     `.rollback-upsert-fails{border-radius: 8px;color:var(--color-primary);}`,
		},
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if _, err := svc.PatchAsset(ctx, a.ID, PatchAssetParams{Edits: []AssetPatchEdit{{
		File: "style.css", OldText: "border-radius: 8px", NewText: "border-radius: 24px",
	}}}); err != nil {
		t.Fatalf("patch asset: %v", err)
	}
	svc.store = &failingAssetStore{Store: st, upsertAssetErr: errInjected}

	_, err = svc.RollbackAsset(ctx, a.ID, 0)
	if !errors.Is(err, errInjected) {
		t.Fatalf("expected injected upsert error, got %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Dir), "style.css"))
	if strings.Contains(string(raw), "8px") || !strings.Contains(string(raw), "24px") {
		t.Fatalf("payload must be restored when rollback upsert fails: %s", raw)
	}
}

func TestAssetServiceConcurrentPatchSameAssetIsSerialized(t *testing.T) {
	ctx := context.Background()
	svc, st, root := newAssetServiceForTest(t)
	var slots []string
	for i := 0; i < 20; i++ {
		slots = append(slots, fmt.Sprintf("slot-%02d", i))
	}
	a, err := svc.CreateAsset(ctx, CreateAssetParams{
		Manifest: componentManifest("concurrent-card"),
		Payload: map[string]string{
			"template.html": `<div class="concurrent-card">` + strings.Join(slots, " ") + `</div>`,
			"style.css":     `.concurrent-card{color:var(--color-primary);}`,
		},
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}

	start := make(chan struct{})
	errs := make(chan error, len(slots))
	var wg sync.WaitGroup
	for i := range slots {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.PatchAsset(ctx, a.ID, PatchAssetParams{Edits: []AssetPatchEdit{{
				File: "template.html", OldText: fmt.Sprintf("slot-%02d", i), NewText: fmt.Sprintf("done-%02d", i),
			}}})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent patch should be serialized, got %v", err)
		}
	}

	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Dir), "template.html"))
	html := string(raw)
	for i := range slots {
		if !strings.Contains(html, fmt.Sprintf("done-%02d", i)) || strings.Contains(html, fmt.Sprintf("slot-%02d", i)) {
			t.Fatalf("final payload lost patch %02d: %s", i, html)
		}
	}
	versions, err := st.ListVersions(ctx, "asset", "asset-"+a.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != len(slots)+1 {
		t.Fatalf("create + 20 patches should produce 21 versions, got %d: %+v", len(versions), versions)
	}
	for i, v := range versions {
		if v.VersionNo != i {
			t.Fatalf("versions must be contiguous, got %+v", versions)
		}
	}
}

func writeAssetFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
