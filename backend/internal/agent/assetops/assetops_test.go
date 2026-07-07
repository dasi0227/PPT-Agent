package assetops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var errInjected = errors.New("injected failure")

type memStore struct {
	assets      map[string]model.Asset
	versions    []model.Version
	nextVerByTg map[string]int
	slideVer    map[string]int
	failCreate  bool
}

func newMemStore() *memStore {
	return &memStore{
		assets:      map[string]model.Asset{},
		nextVerByTg: map[string]int{},
		slideVer:    map[string]int{},
	}
}

func (m *memStore) ListSlides(_ context.Context, _ string) ([]model.Slide, error) {
	return []model.Slide{{ID: "000", ProjectID: "p1", Idx: 0, Order: 0, Layout: "cover", Title: "T",
		JSONPath: model.SlideJSONPath("000"), HTMLPath: model.SlideHTMLPath("000")}}, nil
}

func (m *memStore) ListAssets(_ context.Context, kind string) ([]model.Asset, error) {
	var out []model.Asset
	for _, a := range m.assets {
		if kind == "" || a.Kind == kind {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *memStore) GetAsset(_ context.Context, id string) (model.Asset, error) {
	a, ok := m.assets[id]
	if !ok {
		return model.Asset{}, os.ErrNotExist
	}
	return a, nil
}

func (m *memStore) NextVersionNo(_ context.Context, tt, tid string) (int, error) {
	return m.nextVerByTg[tt+"|"+tid], nil
}

func (m *memStore) CreateVersion(_ context.Context, v model.Version) error {
	if m.failCreate {
		return errInjected
	}
	m.versions = append(m.versions, v)
	m.nextVerByTg[v.TargetType+"|"+v.TargetID] = v.VersionNo + 1
	return nil
}

func (m *memStore) DeleteVersion(_ context.Context, tt, tid string, no int) error {
	for i, v := range m.versions {
		if v.TargetType == tt && v.TargetID == tid && v.VersionNo == no {
			m.versions = append(m.versions[:i], m.versions[i+1:]...)
			break
		}
	}
	return nil
}

func (m *memStore) SetSlideVersion(_ context.Context, slideID string, no int) error {
	m.slideVer[slideID] = no
	return nil
}

func TestMountComponentInjectsTokenizedCSSAndVersions(t *testing.T) {
	workRoot := t.TempDir()
	projectRoot := filepath.Join(workRoot, "p1")
	writeFile(t, projectRoot, "slides/000/index.html", validSlide())
	writeFile(t, projectRoot, "common/tokens.css", ":root{}")
	writeFile(t, projectRoot, "common/base.css", ".slide-stage{}")

	assetDir := "_assets/components/stat-badge"
	writeFile(t, workRoot, assetDir+"/manifest.json", `{
  "name":"stat-badge","version":"1.0.0","kind":"component","source":"preset",
  "description":"stat","mount":{"target":".slide-content","position":"append"},
  "params":{"value":{"type":"string","default":"42"},"label":{"type":"string","default":"指标"}},
  "assets":{"html":"template.html","css":"style.css"}
}`)
	writeFile(t, workRoot, assetDir+"/template.html", `<div class="stat-badge"><span>{{value}}</span><span>{{label}}</span></div>`)
	writeFile(t, workRoot, assetDir+"/style.css", `.stat-badge{color:var(--color-primary);font-size:var(--text-body);}`)

	store := newMemStore()
	store.assets["a-stat"] = model.Asset{
		ID: "a-stat", Name: "stat-badge", Kind: "component", Source: "preset",
		ManifestPath: assetDir + "/manifest.json", Dir: assetDir,
	}
	tool := NewMountAssetTool(store, projectRoot, workRoot, "p1", "r1", nil, func() int64 { return 1 }, seqID())
	res, err := tool.Execute(context.Background(), map[string]any{
		"slide_idx": 0,
		"asset_id":  "a-stat",
		"params": map[string]any{
			"value": "99%",
			"label": "覆盖率",
		},
	})
	if err != nil || !res.OK {
		t.Fatalf("mount_asset: %v %s", err, res.Observation)
	}
	raw, _ := os.ReadFile(filepath.Join(projectRoot, "slides/000/index.html"))
	html := string(raw)
	if !strings.Contains(html, "99%") || !strings.Contains(html, "覆盖率") {
		t.Fatalf("params not applied: %s", html)
	}
	if ok, reason := designsystem.LintSlideResult(raw); !ok {
		t.Fatalf("mounted slide should pass lint: %s", reason)
	}
	if len(store.versions) != 1 || store.versions[0].TargetType != "slide" || store.slideVer["000"] != 0 {
		t.Fatalf("mount should create slide version v0, versions=%+v slideVer=%+v", store.versions, store.slideVer)
	}
}

func TestApplyParamsOnlyReplacesNamedPlaceholders(t *testing.T) {
	template := `<div class="card-default-title">default-title {{title}}</div><p>default-title remains</p>`
	out := applyParams(template, map[string]asset.Param{
		"title": {Type: "string", Default: "default-title"},
	}, map[string]any{"title": `<b>新标题</b>`})
	if !strings.Contains(out, "&lt;b&gt;新标题&lt;/b&gt;") {
		t.Fatalf("placeholder should be HTML-escaped and replaced: %s", out)
	}
	if !strings.Contains(out, "card-default-title") || !strings.Contains(out, "default-title remains") {
		t.Fatalf("default literal text/class must not be globally replaced: %s", out)
	}
	if strings.Contains(out, "{{title}}") {
		t.Fatalf("placeholder should be gone: %s", out)
	}
}

func TestRepeatedMountKeepsCoreHTMLStructure(t *testing.T) {
	workRoot := t.TempDir()
	projectRoot := filepath.Join(workRoot, "p1")
	writeFile(t, projectRoot, "slides/000/index.html", validSlide())
	writeFile(t, projectRoot, "common/tokens.css", ":root{}")
	writeFile(t, projectRoot, "common/base.css", ".slide-stage{}")

	assetDir := "_assets/components/badge"
	writeFile(t, workRoot, assetDir+"/manifest.json", `{
  "name":"badge","version":"1.0.0","kind":"component","source":"preset",
  "description":"badge","mount":{"target":".slide-content","position":"append"},
  "params":{"value":{"type":"string","default":"42"}},
  "assets":{"html":"template.html","css":"style.css"}
}`)
	writeFile(t, workRoot, assetDir+"/template.html", `<div class="badge">{{value}}</div>`)
	writeFile(t, workRoot, assetDir+"/style.css", `.badge{color:var(--color-primary);font-size:var(--text-body);}`)
	store := newMemStore()
	store.assets["a-badge"] = model.Asset{ID: "a-badge", Name: "badge", Kind: "component", Source: "preset", ManifestPath: assetDir + "/manifest.json", Dir: assetDir}
	tool := NewMountAssetTool(store, projectRoot, workRoot, "p1", "r1", nil, func() int64 { return 1 }, seqID())

	for i := 0; i < 3; i++ {
		res, err := tool.Execute(context.Background(), map[string]any{
			"slide_idx": 0,
			"asset_id":  "a-badge",
			"params":    map[string]any{"value": "v" + string(rune('0'+i))},
		})
		if err != nil || !res.OK {
			t.Fatalf("mount #%d: %v %s", i+1, err, res.Observation)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(projectRoot, "slides/000/index.html"))
	html := strings.ToLower(string(raw))
	for _, want := range []string{"<!doctype html>", "<html", "<head", "<body", "tokens.css", "base.css", "slide-stage"} {
		if !strings.Contains(html, want) {
			t.Fatalf("mounted html lost core structure %q: %s", want, raw)
		}
	}
	if ok, reason := designsystem.LintSlideResult(raw); !ok {
		t.Fatalf("repeatedly mounted slide should pass lint: %s", reason)
	}
	if len(store.versions) != 3 {
		t.Fatalf("each successful mount should create a version, got %+v", store.versions)
	}
}

func TestMountRejectsInvalidSlideWithoutWriting(t *testing.T) {
	workRoot := t.TempDir()
	projectRoot := filepath.Join(workRoot, "p1")
	writeFile(t, projectRoot, "slides/000/index.html", validSlide())
	before, _ := os.ReadFile(filepath.Join(projectRoot, "slides/000/index.html"))
	assetDir := "_assets/components/bad-card"
	writeFile(t, workRoot, assetDir+"/manifest.json", `{
  "name":"bad-card","version":"1.0.0","kind":"component","source":"user",
  "description":"bad","mount":{"target":".slide-content","position":"append"},
  "assets":{"html":"template.html","css":"style.css"}
}`)
	writeFile(t, workRoot, assetDir+"/template.html", `<div class="bad-card">bad</div>`)
	writeFile(t, workRoot, assetDir+"/style.css", `.bad-card{color:#ffffff;}`)
	store := newMemStore()
	store.assets["a-bad"] = model.Asset{ID: "a-bad", Name: "bad-card", Kind: "component", Source: "user", ManifestPath: assetDir + "/manifest.json", Dir: assetDir}

	tool := NewMountAssetTool(store, projectRoot, workRoot, "p1", "r1", nil, func() int64 { return 1 }, seqID())
	res, err := tool.Execute(context.Background(), map[string]any{"slide_idx": 0, "asset_id": "a-bad"})
	if err != nil {
		t.Fatalf("mount_asset unexpected error: %v", err)
	}
	if res.OK {
		t.Fatal("mount should fail when lint-slide would fail")
	}
	after, _ := os.ReadFile(filepath.Join(projectRoot, "slides/000/index.html"))
	if string(before) != string(after) {
		t.Fatal("slide must remain unchanged on failed mount")
	}
	if len(store.versions) != 0 {
		t.Fatalf("failed mount must not create version, got %+v", store.versions)
	}
}

func TestApplyThemeWritesFullTokensAndVersions(t *testing.T) {
	workRoot := t.TempDir()
	projectRoot := filepath.Join(workRoot, "p1")
	tokens, err := asset.ReadSeedFile("assets/themes/tokyo-night/tokens.css")
	if err != nil {
		t.Fatalf("read seed tokens: %v", err)
	}
	assetDir := "_assets/themes/tokyo-night"
	writeFile(t, workRoot, assetDir+"/manifest.json", `{
  "name":"tokyo-night","version":"1.0.0","kind":"theme","source":"preset",
  "description":"theme","assets":{"tokens":"tokens.css"}
}`)
	writeFile(t, workRoot, assetDir+"/tokens.css", string(tokens))
	writeFile(t, projectRoot, "common/tokens.css", ":root{}")
	store := newMemStore()
	store.assets["theme-1"] = model.Asset{ID: "theme-1", Name: "tokyo-night", Kind: "theme", Source: "preset", ManifestPath: assetDir + "/manifest.json", Dir: assetDir}

	tool := NewApplyThemeTool(store, projectRoot, workRoot, "p1", "r1", func() int64 { return 1 }, seqID())
	res, err := tool.Execute(context.Background(), map[string]any{"theme_asset_id": "theme-1"})
	if err != nil || !res.OK {
		t.Fatalf("apply_theme: %v %s", err, res.Observation)
	}
	written, _ := os.ReadFile(filepath.Join(projectRoot, "common/tokens.css"))
	if string(written) != string(tokens) {
		t.Fatal("apply_theme should copy the theme token file exactly")
	}
	if miss := asset.ValidateThemeTokens(written); len(miss) != 0 {
		t.Fatalf("applied theme missing tokens: %v", miss)
	}
	if len(store.versions) != 1 || store.versions[0].TargetType != "design" {
		t.Fatalf("apply_theme should create design version, got %+v", store.versions)
	}
}

func TestApplyThemeRestoresTokensWhenVersionCreateFails(t *testing.T) {
	workRoot := t.TempDir()
	projectRoot := filepath.Join(workRoot, "p1")
	tokens, err := asset.ReadSeedFile("assets/themes/tokyo-night/tokens.css")
	if err != nil {
		t.Fatalf("read seed tokens: %v", err)
	}
	assetDir := "_assets/themes/tokyo-night"
	writeFile(t, workRoot, assetDir+"/manifest.json", `{
  "name":"tokyo-night","version":"1.0.0","kind":"theme","source":"preset",
  "description":"theme","assets":{"tokens":"tokens.css"}
}`)
	writeFile(t, workRoot, assetDir+"/tokens.css", string(tokens))
	writeFile(t, projectRoot, "common/tokens.css", ":root{--color-primary:#000;}")
	store := newMemStore()
	store.failCreate = true
	store.assets["theme-1"] = model.Asset{ID: "theme-1", Name: "tokyo-night", Kind: "theme", Source: "preset", ManifestPath: assetDir + "/manifest.json", Dir: assetDir}

	tool := NewApplyThemeTool(store, projectRoot, workRoot, "p1", "r1", func() int64 { return 1 }, seqID())
	_, err = tool.Execute(context.Background(), map[string]any{"theme_asset_id": "theme-1"})
	if err != errInjected {
		t.Fatalf("expected injected version error, got %v", err)
	}
	written, _ := os.ReadFile(filepath.Join(projectRoot, "common/tokens.css"))
	if string(written) != ":root{--color-primary:#000;}" {
		t.Fatalf("tokens.css must be restored on version failure: %s", written)
	}
	if len(store.versions) != 0 {
		t.Fatalf("failed apply_theme must not record versions, got %+v", store.versions)
	}
}

func validSlide() string {
	return `<!doctype html><html><head>` +
		`<link rel="stylesheet" href="../../common/tokens.css">` +
		`<link rel="stylesheet" href="../../common/base.css"></head>` +
		`<body><div class="slide-scaler"><section class="slide-stage">` +
		`<div class="slide-content"><h1 class="slide-title">标题</h1></div>` +
		`</section></div></body></html>`
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seqID() func() string {
	i := 0
	return func() string {
		i++
		return "v"
	}
}
