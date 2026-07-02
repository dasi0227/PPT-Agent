package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

var (
	ErrAssetConflict     = errors.New("service: asset already exists")
	ErrPresetAssetDelete = errors.New("service: preset asset cannot be deleted")
	ErrAssetVersionMiss  = errors.New("service: asset version not found")
)

// ValidationError 表示资产协议或载荷校验失败，应映射为 422 VALIDATION_FAILED。
type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

func validationError(format string, args ...any) error {
	return &ValidationError{Err: fmt.Errorf(format, args...)}
}

func IsValidationError(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}

func IsPresetDeleteError(err error) bool {
	return errors.Is(err, ErrPresetAssetDelete)
}

// CreateAssetParams 是 REST /assets 与 /repo create_asset 共享的新增资产输入。
type CreateAssetParams struct {
	Manifest asset.Manifest
	Payload  map[string]string // key 为 manifest.assets 中声明的相对文件名
}

// AssetPatchEdit 是对某个资产载荷文件的一次锚定替换。
// File 可为实际载荷文件名（如 style.css），也可为逻辑名 html/css/js/tokens。
type AssetPatchEdit struct {
	File    string
	OldText string
	NewText string
}

// PatchAssetParams 是资产锚定修改输入。
type PatchAssetParams struct {
	Edits []AssetPatchEdit
}

// AssetService 收敛个人仓库资产校验、落盘、版本化与回滚逻辑。
// REST handler 与 /repo agent 工具都应调用本服务，避免两套实现分叉。
type AssetService struct {
	store    store.Store
	workRoot string
	clock    func() int64
	newID    func() string
	locks    *AssetLockManager
}

func NewAssetService(s store.Store, workRoot string) *AssetService {
	return NewAssetServiceWithLocks(s, workRoot, defaultAssetLocks)
}

func NewAssetServiceWithLocks(s store.Store, workRoot string, locks *AssetLockManager) *AssetService {
	if locks == nil {
		locks = defaultAssetLocks
	}
	return &AssetService{store: s, workRoot: workRoot, clock: nowUnixAsset, newID: uuid.NewString, locks: locks}
}

// AssetLockManager serializes global asset mutations across REST and agent AssetService instances.
type AssetLockManager struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

var defaultAssetLocks = NewAssetLockManager()

func NewAssetLockManager() *AssetLockManager {
	return &AssetLockManager{locks: map[string]*sync.Mutex{}}
}

func (m *AssetLockManager) Lock(key string) func() {
	m.mu.Lock()
	l, ok := m.locks[key]
	if !ok {
		l = &sync.Mutex{}
		m.locks[key] = l
	}
	m.mu.Unlock()
	l.Lock()
	return l.Unlock
}

func (svc *AssetService) ListAssets(ctx context.Context, kind string) ([]model.Asset, error) {
	return svc.store.ListAssets(ctx, kind)
}

func (svc *AssetService) GetAsset(ctx context.Context, id string) (model.Asset, error) {
	return svc.store.GetAsset(ctx, id)
}

func (svc *AssetService) CreateAsset(ctx context.Context, p CreateAssetParams) (model.Asset, error) {
	m := p.Manifest
	m.Source = asset.SourceUser
	if err := validateNewAsset(m, p.Payload); err != nil {
		return model.Asset{}, err
	}
	unlock := svc.locks.Lock(assetNameLockKey(string(m.Kind), m.Name))
	defer unlock()
	if exists, err := svc.assetNameExists(ctx, string(m.Kind), m.Name); err != nil {
		return model.Asset{}, err
	} else if exists {
		return model.Asset{}, ErrAssetConflict
	}

	kindDir, err := kindDir(m.Kind)
	if err != nil {
		return model.Asset{}, err
	}
	relDir := path.Join("_assets", kindDir, m.Name)
	sandbox, err := tools.NewSandbox(svc.workRoot)
	if err != nil {
		return model.Asset{}, err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return model.Asset{}, err
	}
	if err := sandbox.Write(path.Join(relDir, "manifest.json"), raw); err != nil {
		_ = removeDir(sandbox, relDir)
		return model.Asset{}, err
	}
	for rel, body := range p.Payload {
		if err := validatePayloadPath(rel); err != nil {
			_ = removeDir(sandbox, relDir)
			return model.Asset{}, err
		}
		if err := sandbox.Write(path.Join(relDir, rel), []byte(body)); err != nil {
			_ = removeDir(sandbox, relDir)
			return model.Asset{}, err
		}
	}

	now := svc.clock()
	a := model.Asset{
		ID:           svc.newID(),
		Name:         m.Name,
		Kind:         string(m.Kind),
		Version:      m.Version,
		Source:       string(asset.SourceUser),
		Description:  m.Description,
		Tags:         m.Tags,
		ManifestPath: path.Join(relDir, "manifest.json"),
		Dir:          relDir,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := svc.store.CreateAsset(ctx, a); err != nil {
		_ = removeDir(sandbox, relDir)
		if isUniqueConstraintError(err) {
			return model.Asset{}, ErrAssetConflict
		}
		return model.Asset{}, err
	}
	if _, err := svc.snapshotAsset(ctx, sandbox, a, ""); err != nil {
		_ = svc.store.DeleteAsset(ctx, a.ID)
		_ = removeDir(sandbox, relDir)
		return model.Asset{}, err
	}
	return a, nil
}

func (svc *AssetService) PatchAsset(ctx context.Context, id string, p PatchAssetParams) (model.Asset, error) {
	unlock := svc.locks.Lock(assetIDLockKey(id))
	defer unlock()

	if len(p.Edits) == 0 {
		return model.Asset{}, validationError("edits must not be empty")
	}
	a, err := svc.store.GetAsset(ctx, id)
	if err != nil {
		return model.Asset{}, err
	}
	sandbox, err := tools.NewSandbox(svc.workRoot)
	if err != nil {
		return model.Asset{}, err
	}
	manifestRaw, err := sandbox.Read(a.ManifestPath)
	if err != nil {
		return model.Asset{}, err
	}
	if err := asset.ValidateManifestRaw(manifestRaw); err != nil {
		return model.Asset{}, validationError("%v", err)
	}
	m, err := asset.Parse(manifestRaw)
	if err != nil {
		return model.Asset{}, err
	}
	if err := svc.ensureAssetBaseline(ctx, sandbox, a); err != nil {
		return model.Asset{}, err
	}

	contents := map[string]string{}
	originals := map[string][]byte{}
	for i, e := range p.Edits {
		if strings.TrimSpace(e.File) == "" {
			return model.Asset{}, validationError("edit #%d file is required", i+1)
		}
		if e.OldText == "" {
			return model.Asset{}, validationError("edit #%d old_text is required", i+1)
		}
		payloadRel, err := payloadRelForPatch(m, e.File)
		if err != nil {
			return model.Asset{}, err
		}
		fileRel := path.Join(a.Dir, payloadRel)
		content, ok := contents[fileRel]
		if !ok {
			raw, err := sandbox.Read(fileRel)
			if err != nil {
				return model.Asset{}, err
			}
			originals[fileRel] = raw
			content = string(raw)
		}
		n := strings.Count(content, e.OldText)
		if n == 0 {
			return model.Asset{}, validationError("anchor not found in %s for edit #%d", payloadRel, i+1)
		}
		if n > 1 {
			return model.Asset{}, validationError("anchor not unique in %s for edit #%d: %d matches", payloadRel, i+1, n)
		}
		contents[fileRel] = strings.Replace(content, e.OldText, e.NewText, 1)
	}

	for fileRel, content := range contents {
		if m.Kind == asset.KindTheme && path.Base(fileRel) == path.Base(m.Assets.Tokens) {
			if miss := asset.ValidateThemeTokens([]byte(content)); len(miss) != 0 {
				return model.Asset{}, validationError("theme missing required tokens: %v", miss)
			}
		}
	}
	for fileRel, content := range contents {
		if err := sandbox.Write(fileRel, []byte(content)); err != nil {
			return model.Asset{}, err
		}
	}

	oldAsset := a
	a.UpdatedAt = svc.clock()
	if err := svc.store.UpsertAsset(ctx, a); err != nil {
		restoreFiles(sandbox, originals)
		return model.Asset{}, err
	}
	if _, err := svc.snapshotAsset(ctx, sandbox, a, ""); err != nil {
		restoreFiles(sandbox, originals)
		_ = svc.store.UpsertAsset(ctx, oldAsset)
		return model.Asset{}, err
	}
	return a, nil
}

func (svc *AssetService) DeleteAsset(ctx context.Context, id string) error {
	unlock := svc.locks.Lock(assetIDLockKey(id))
	defer unlock()

	a, err := svc.store.GetAsset(ctx, id)
	if err != nil {
		return err
	}
	unlockName := svc.locks.Lock(assetNameLockKey(a.Kind, a.Name))
	defer unlockName()
	if a.Source == string(asset.SourcePreset) {
		return ErrPresetAssetDelete
	}
	sandbox, err := tools.NewSandbox(svc.workRoot)
	if err != nil {
		return err
	}
	if err := svc.ensureAssetBaseline(ctx, sandbox, a); err != nil {
		return err
	}
	if _, err := svc.snapshotAsset(ctx, sandbox, a, ""); err != nil {
		return err
	}
	if err := svc.store.DeleteAsset(ctx, id); err != nil {
		return err
	}
	return removeDir(sandbox, a.Dir)
}

// RollbackAsset 用历史资产目录快照覆盖当前资产目录，并记录一个新的 asset 版本。
func (svc *AssetService) RollbackAsset(ctx context.Context, id string, versionNo int) (model.Asset, error) {
	unlock := svc.locks.Lock(assetIDLockKey(id))
	defer unlock()

	versions, err := svc.store.ListVersions(ctx, "asset", model.AssetVersionTarget(id))
	if err != nil {
		return model.Asset{}, err
	}
	var snapshot string
	for _, v := range versions {
		if v.VersionNo == versionNo {
			snapshot = v.SnapshotPath
			break
		}
	}
	if snapshot == "" {
		return model.Asset{}, ErrAssetVersionMiss
	}

	sandbox, err := tools.NewSandbox(svc.workRoot)
	if err != nil {
		return model.Asset{}, err
	}
	manifestRaw, err := sandbox.Read(path.Join(snapshot, "manifest.json"))
	if err != nil {
		return model.Asset{}, err
	}
	if err := asset.ValidateManifestRaw(manifestRaw); err != nil {
		return model.Asset{}, validationError("%v", err)
	}
	m, err := asset.Parse(manifestRaw)
	if err != nil {
		return model.Asset{}, err
	}
	kindDir, err := kindDir(m.Kind)
	if err != nil {
		return model.Asset{}, err
	}
	relDir := path.Join("_assets", kindDir, m.Name)
	unlockName := svc.locks.Lock(assetNameLockKey(string(m.Kind), m.Name))
	defer unlockName()

	current, getErr := svc.store.GetAsset(ctx, id)
	createdAt := svc.clock()
	if getErr == nil && current.CreatedAt != 0 {
		createdAt = current.CreatedAt
	}
	hadCurrent := getErr == nil
	backupDir := ""
	if hadCurrent {
		backupDir = path.Join("_tmp", "asset-rollback", id+"-"+svc.newID())
		if err := copyDir(sandbox, current.Dir, backupDir); err != nil {
			return model.Asset{}, err
		}
		defer func() { _ = removeDir(sandbox, backupDir) }()
	}
	restoreCurrent := func() {
		_ = removeDir(sandbox, relDir)
		if hadCurrent {
			_ = copyDir(sandbox, backupDir, current.Dir)
		}
	}
	if err := removeDir(sandbox, relDir); err != nil && !os.IsNotExist(err) {
		return model.Asset{}, err
	}
	if err := copyDir(sandbox, snapshot, relDir); err != nil {
		restoreCurrent()
		return model.Asset{}, err
	}
	now := svc.clock()
	a := model.Asset{
		ID:           id,
		Name:         m.Name,
		Kind:         string(m.Kind),
		Version:      m.Version,
		Source:       string(m.Source),
		Description:  m.Description,
		Tags:         m.Tags,
		ManifestPath: path.Join(relDir, "manifest.json"),
		Dir:          relDir,
		CreatedAt:    createdAt,
		UpdatedAt:    now,
	}
	if a.Source == "" {
		a.Source = string(asset.SourceUser)
	}
	if err := svc.store.UpsertAsset(ctx, a); err != nil {
		restoreCurrent()
		return model.Asset{}, err
	}
	if _, err := svc.snapshotAsset(ctx, sandbox, a, ""); err != nil {
		restoreCurrent()
		if hadCurrent {
			_ = svc.store.UpsertAsset(ctx, current)
		}
		return model.Asset{}, err
	}
	return a, nil
}

func (svc *AssetService) assetNameExists(ctx context.Context, kind, name string) (bool, error) {
	assets, err := svc.store.ListAssets(ctx, kind)
	if err != nil {
		return false, err
	}
	for _, a := range assets {
		if a.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (svc *AssetService) ensureAssetBaseline(ctx context.Context, sandbox *tools.Sandbox, a model.Asset) error {
	versions, err := svc.store.ListVersions(ctx, "asset", model.AssetVersionTarget(a.ID))
	if err != nil {
		return err
	}
	if len(versions) != 0 {
		return nil
	}
	_, err = svc.snapshotAsset(ctx, sandbox, a, "")
	return err
}

func (svc *AssetService) snapshotAsset(ctx context.Context, sandbox *tools.Sandbox, a model.Asset, runID string) (int, error) {
	target := model.AssetVersionTarget(a.ID)
	no, err := svc.store.NextVersionNo(ctx, "asset", target)
	if err != nil {
		return 0, err
	}
	snap := fmt.Sprintf("versions/%s/v%d", target, no)
	if err := copyDir(sandbox, a.Dir, snap); err != nil {
		return 0, err
	}
	if err := svc.store.CreateVersion(ctx, model.Version{
		ID: svc.newID(), TargetType: "asset", TargetID: target, VersionNo: no,
		SnapshotPath: snap, RunID: runID, CreatedAt: svc.clock(),
	}); err != nil {
		_ = removeDir(sandbox, snap)
		return 0, err
	}
	return no, nil
}

func validateNewAsset(m asset.Manifest, payload map[string]string) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := asset.ValidateManifestRaw(raw); err != nil {
		return validationError("%v", err)
	}
	for _, rel := range payloadFiles(m) {
		if err := validatePayloadPath(rel); err != nil {
			return err
		}
		if _, ok := payload[rel]; !ok {
			return validationError("payload missing declared file %q", rel)
		}
	}
	if m.Kind == asset.KindTheme {
		tokens := payload[m.Assets.Tokens]
		if miss := asset.ValidateThemeTokens([]byte(tokens)); len(miss) != 0 {
			return validationError("theme missing required tokens: %v", miss)
		}
	}
	return nil
}

func payloadFiles(m asset.Manifest) []string {
	var out []string
	for _, rel := range []string{m.Assets.HTML, m.Assets.CSS, m.Assets.JS, m.Assets.Tokens} {
		if rel != "" {
			out = append(out, rel)
		}
	}
	return out
}

func payloadRelForPatch(m asset.Manifest, file string) (string, error) {
	var rel string
	switch file {
	case "html":
		rel = m.Assets.HTML
	case "css":
		rel = m.Assets.CSS
	case "js":
		rel = m.Assets.JS
	case "tokens":
		rel = m.Assets.Tokens
	default:
		rel = file
	}
	if rel == "" {
		return "", validationError("asset does not declare payload %q", file)
	}
	if err := validatePayloadPath(rel); err != nil {
		return "", err
	}
	return rel, nil
}

func validatePayloadPath(rel string) error {
	if rel == "" {
		return validationError("payload path is empty")
	}
	clean := path.Clean(rel)
	if path.IsAbs(rel) || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return validationError("payload path escapes asset dir: %q", rel)
	}
	return nil
}

func kindDir(k asset.Kind) (string, error) {
	switch k {
	case asset.KindTheme:
		return "themes", nil
	case asset.KindLayout:
		return "layouts", nil
	case asset.KindComponent:
		return "components", nil
	case asset.KindFx:
		return "fx", nil
	default:
		return "", validationError("unsupported asset kind %q", k)
	}
}

func copyDir(sandbox *tools.Sandbox, srcRel, dstRel string) error {
	src, err := resolveSandboxPath(sandbox, srcRel)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", srcRel)
	}
	dst, err := resolveSandboxPath(sandbox, dstRel)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}

func removeDir(sandbox *tools.Sandbox, rel string) error {
	abs, err := resolveSandboxPath(sandbox, rel)
	if err != nil {
		return err
	}
	return os.RemoveAll(abs)
}

func restoreFiles(sandbox *tools.Sandbox, originals map[string][]byte) {
	for rel, raw := range originals {
		_ = sandbox.Write(rel, raw)
	}
}

func assetIDLockKey(id string) string {
	return "asset:" + id
}

func assetNameLockKey(kind, name string) string {
	return "asset-name:" + kind + "/" + name
}

func resolveSandboxPath(sandbox *tools.Sandbox, rel string) (string, error) {
	abs, err := sandbox.Resolve(rel)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") &&
		strings.Contains(msg, "assets.name") &&
		strings.Contains(msg, "assets.kind")
}

func nowUnixAsset() int64 { return time.Now().Unix() }
