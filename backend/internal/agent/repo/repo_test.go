package repo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/edit"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/overview"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// ── test doubles ──────────────────────────────────────

type memStore struct {
	assets map[string]model.Asset
}

func newMemStore() *memStore { return &memStore{assets: map[string]model.Asset{}} }

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
func (m *memStore) UpsertAsset(_ context.Context, a model.Asset) error {
	m.assets[a.ID] = a
	return nil
}

type nopLLM struct{}

func (nopLLM) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}
func (nopLLM) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}
func (nopLLM) CallTool(context.Context, llm.ToolCallRequest) (llm.ToolCallResponse, error) {
	return llm.ToolCallResponse{ToolCall: &llm.ToolCall{Name: "finish", Args: map[string]any{"summary": "ok"}}}, nil
}

// ── helpers ──────────────────────────────────────

const neonManifest = `{
  "name": "neon-card",
  "version": "1.0.0",
  "kind": "component",
  "description": "霓虹卡片",
  "tags": ["card","neon"],
  "mount": {"position": "append"},
  "assets": {"html": "component.html", "css": "component.css"}
}`

const neonCSS = `.neon-card{ border-radius: 8px; box-shadow: 0 0 12px var(--color-primary); }`
const neonHTML = `<div class="neon-card">{{content}}</div>`

// setupRepo 建一个 work_root，落一个 neon-card 组件资产 + 一个 PPT 项目页与公共层（用于隔离断言）。
func setupRepo(t *testing.T) (string, *memStore) {
	t.Helper()
	root := t.TempDir()
	// 资产：_assets/components/neon-card/
	dir := "_assets/components/neon-card"
	writeAt(t, root, dir+"/manifest.json", neonManifest)
	writeAt(t, root, dir+"/component.css", neonCSS)
	writeAt(t, root, dir+"/component.html", neonHTML)

	// 一个 PPT 项目（隔离对照）：projects/p1/slides + common
	writeAt(t, root, "projects/p1/common/tokens.css", ":root{--color-primary:#f00;}")
	writeAt(t, root, "projects/p1/slides/000/index.html", "<html>page0</html>")
	writeAt(t, root, "projects/p1/slides/001/index.html", "<html>page1</html>")

	store := newMemStore()
	store.assets["a-neon"] = model.Asset{
		ID: "a-neon", Name: "neon-card", Kind: "component", Source: "user",
		Description: "霓虹卡片", Tags: []string{"card", "neon"},
		ManifestPath: dir + "/manifest.json", Dir: dir,
	}
	return root, store
}

func writeAt(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hashTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		data, _ := os.ReadFile(p)
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	return out
}

func mustSandbox(t *testing.T, root string) *tools.Sandbox {
	t.Helper()
	sb, err := tools.NewSandbox(root)
	if err != nil {
		t.Fatalf("sandbox: %v", err)
	}
	return sb
}

// SPEC-CMD-REPO-001/004：scope=repo 的门控只含资产工具，绝不含任何页/公共层写工具。
func TestRepoScopeAssetOnly(t *testing.T) {
	root, store := setupRepo(t)
	// 故意把页/公共层写工具也塞进候选，验证 Gate 机制级排除它们。
	sb := mustSandbox(t, root)
	all := []tools.Tool{
		NewSearchAssetsTool(store),
		NewReadAssetTool(store, sb),
		NewPatchAssetTool(store, sb, func() int64 { return 1 }),
		NewValidateAssetTool(store, sb),
		tools.NewFinishTool(),
		// 越权：页/公共层写工具（属其它 scope）
		edit.NewPatchSlideTool(nil, sb, "p1", "r1", 0, func() int64 { return 1 }, func() string { return "v" }),
		overview.NewPatchDesignTool(nil, sb, "p1", "r1", func() int64 { return 1 }, func() string { return "v" }),
	}
	loop := harness.New(nopLLM{}, harness.Config{
		RunID: "r1", Kind: model.KindEdit, Scope: model.ScopeRepo, Mode: model.ModeNormal, Tools: all,
	})
	names := map[string]bool{}
	for _, tl := range loop.GatedTools() {
		names[tl.Name()] = true
	}
	// 页/公共层写工具 MUST NOT 出现。
	for _, forbidden := range []string{"patch_slide", "patch_design", "write_slide", "fanout_page_patch"} {
		if names[forbidden] {
			t.Errorf("repo scope MUST NOT register %q", forbidden)
		}
	}
	// 资产工具 MUST 全在。
	for _, want := range []string{"search_assets", "read_asset", "patch_asset", "validate_asset", "finish"} {
		if !names[want] {
			t.Errorf("repo scope should register %q", want)
		}
	}
}

// AC-CMD-REPO-001：patch_asset 把 neon-card 圆角调大 → 仅该资产载荷变，任何 PPT 页/公共层 hash 不变。
func TestPatchAssetOnlyChangesAsset(t *testing.T) {
	root, store := setupRepo(t)
	before := hashTree(t, root)
	sb := mustSandbox(t, root)

	tool := NewPatchAssetTool(store, sb, func() int64 { return 2 })
	res, err := tool.Execute(context.Background(), map[string]any{
		"asset_id": "a-neon", "file": "css",
		"edits": []any{map[string]any{"old_text": "border-radius: 8px", "new_text": "border-radius: 24px"}},
	})
	if err != nil || !res.OK {
		t.Fatalf("patch_asset: %v %s", err, res.Observation)
	}

	after := hashTree(t, root)
	// 资产 css 变了。
	cssKey := "_assets/components/neon-card/component.css"
	if before[cssKey] == after[cssKey] {
		t.Error("asset css must change")
	}
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(cssKey)))
	if !strings.Contains(string(raw), "24px") {
		t.Errorf("radius not updated: %s", raw)
	}
	// 任何 PPT 页/公共层 hash 不变（SPEC-CMD-REPO-004）。
	for _, ppt := range []string{
		"projects/p1/common/tokens.css",
		"projects/p1/slides/000/index.html",
		"projects/p1/slides/001/index.html",
	} {
		if before[ppt] != after[ppt] {
			t.Errorf("PPT file changed (must not): %s", ppt)
		}
	}
	// 资产其它载荷（html）未动。
	htmlKey := "_assets/components/neon-card/component.html"
	if before[htmlKey] != after[htmlKey] {
		t.Error("untouched payload (html) must not change")
	}
}

// ARCH-TOOLS-003：锚点不唯一 → 整体失败，文件不变。
func TestPatchAssetAnchorNotUnique(t *testing.T) {
	root, store := setupRepo(t)
	// 让 css 里出现两个相同片段。
	writeAt(t, root, "_assets/components/neon-card/component.css", ".a{x:1px}.b{x:1px}")
	sb := mustSandbox(t, root)
	before := hashTree(t, root)

	tool := NewPatchAssetTool(store, sb, func() int64 { return 2 })
	res, _ := tool.Execute(context.Background(), map[string]any{
		"asset_id": "a-neon", "file": "css",
		"edits": []any{map[string]any{"old_text": "x:1px", "new_text": "x:2px"}},
	})
	if res.OK {
		t.Fatal("expected failure: anchor not unique")
	}
	after := hashTree(t, root)
	if before["_assets/components/neon-card/component.css"] != after["_assets/components/neon-card/component.css"] {
		t.Error("file must be unchanged on anchor failure")
	}
}

// 越界路径护栏（ARCH-TOOLS-006）：资产 Dir 指向 work_root 外时，写入被 Sandbox 拒绝。
func TestPatchAssetPathBoundary(t *testing.T) {
	root, store := setupRepo(t)
	// 篡改资产 Dir 指向上级目录逃逸。
	a := store.assets["a-neon"]
	a.Dir = "../escape"
	a.ManifestPath = "../escape/manifest.json"
	store.assets["a-neon"] = a
	sb := mustSandbox(t, root)

	tool := NewPatchAssetTool(store, sb, func() int64 { return 2 })
	res, _ := tool.Execute(context.Background(), map[string]any{
		"asset_id": "a-neon", "file": "css",
		"edits": []any{map[string]any{"old_text": "x", "new_text": "y"}},
	})
	if res.OK {
		t.Fatal("expected failure: path escapes work_root")
	}
}

// runner 装配：scope=repo 的 runner 跑通（nop LLM 直接 finish），无 panic。
func TestRepoRunnerFinishes(t *testing.T) {
	root, store := setupRepo(t)
	r := NewRunner(nopLLM{}, store, Params{RunID: "r1", WorkRoot: root, Instruction: "把 neon-card 圆角调大"},
		func() int64 { return 1 }, func() string { return "v" })
	out := r.Run(context.Background(), nopEmitter{}, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("expected finished, got %s: %s", out.Status, out.Message)
	}
}

type nopEmitter struct{}

func (nopEmitter) Emit(model.EventType, any) {}
