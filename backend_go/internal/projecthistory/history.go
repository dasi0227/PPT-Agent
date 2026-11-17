// Package projecthistory owns whole-project history, independently of runtime checkpoints.
package projecthistory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/google/uuid"
)

var ErrConflict = errors.New("项目历史已变化，请重新确认")
var ErrBusy = errors.New("项目存在未结束任务或冲突操作，请结束后重试")
var ErrTarget = errors.New("该任务不在当前可回退历史中")

type SnapshotStore interface {
	store.Store
	CaptureProject(context.Context, string) (json.RawMessage, error)
	ReadProjectHistory(context.Context, string) (sessionstate.HistoryControl, error)
	SaveProjectHistory(context.Context, string, int64, int64, sessionstate.HistoryState) (sessionstate.HistoryState, error)
	ProjectHistoryReceipt(context.Context, string, string, string) (*sessionstate.HistoryState, error)
	BeginHistoryOperation(context.Context, string, sessionstate.HistoryOperation) error
	IsolateHistoryOperation(context.Context, string, string) error
	FinishHistoryMutation(context.Context, string, string) (sessionstate.HistoryState, error)
	FinishHistoryRestore(context.Context, string, string, json.RawMessage, sessionstate.HistoryState, bool) (sessionstate.HistoryState, error)
	PendingHistoryProjects(context.Context) ([]string, error)
}
type Checkpoint = sessionstate.ProjectCheckpoint
type State = sessionstate.HistoryState
type File struct {
	Data []byte `json:"data"`
	Mode uint32 `json:"mode"`
}
type Snapshot struct {
	ProjectID string          `json:"project_id"`
	Database  json.RawMessage `json:"database"`
	Files     map[string]File `json:"files"`
	State     State           `json:"state"`
}
type Preview struct {
	Revision      int64  `json:"revision,string"`
	SceneRevision int64  `json:"scene_revision,string"`
	Time          int64  `json:"time"`
	Input         string `json:"input"`
	Runs          int    `json:"runs"`
}
type Manager struct {
	Store         SnapshotStore
	Locks         *run.LockManager
	ExportActive  func(string) bool
	SnapshotGuard func(context.Context, string) func()
	root          string
	gates         sync.Map
	switching     sync.Map
}

// Switching reports the short interval in which restored files may precede
// publication of their new scene revision.
func (m *Manager) Switching(id string) bool {
	value, ok := m.switching.Load(id)
	return ok && value.(bool)
}

func New(s SnapshotStore, locks *run.LockManager, root string) *Manager {
	return &Manager{Store: s, Locks: locks, root: filepath.Join(root, "projects")}
}
func (m *Manager) TryGate(id string) (func(), bool) {
	v, _ := m.gates.LoadOrStore(id, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	if !mu.TryLock() {
		return nil, false
	}
	return mu.Unlock, true
}
func (m *Manager) Gate(id string) func() {
	v, _ := m.gates.LoadOrStore(id, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
func (m *Manager) container(id string) string {
	if id == "" || id == "." || !filepath.IsLocal(id) || filepath.Base(id) != id {
		return filepath.Join(m.root, ".invalid-project-id")
	}
	return filepath.Join(m.root, id)
}
func (m *Manager) dir(id string) string {
	return filepath.Join(m.container(id), "checkpoints")
}

func writeJSON(path string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err = mkdirDurable(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".prepare-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func (m *Manager) State(id string) (State, error) {
	control, err := m.Store.ReadProjectHistory(context.Background(), id)
	if err != nil {
		return State{}, err
	}
	if control.Restoring {
		return State{}, sessionstate.ErrProjectRestoring
	}
	return control.State, nil
}
func excluded(path string) bool {
	first := strings.Split(filepath.ToSlash(path), "/")[0]
	return first == "versions" || first == ".git" || first == ".runtime" || first == ".run" || first == ".commit-tmp"
}
func files(root string) (map[string]File, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("project snapshot requires a real directory")
	}
	out := map[string]File{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if excluded(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("project snapshot refuses symlink: %s", rel)
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported project file")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = File{raw, uint32(info.Mode().Perm())}
		return nil
	})
	return out, err
}
func (m *Manager) Capture(ctx context.Context, p model.Project, s State) (string, error) {
	if m.SnapshotGuard != nil && ctx.Value(snapshotGuardKey{}) != p.ID {
		release := m.SnapshotGuard(ctx, p.ID)
		defer release()
	}
	db, err := m.Store.CaptureProject(ctx, p.ID)
	if err != nil {
		return "", err
	}
	f, err := projectFiles(p.WorkDir)
	if err != nil {
		return "", err
	}
	snap := Snapshot{ProjectID: p.ID, Database: db, Files: f, State: s}
	raw, err := json.Marshal(snap)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	id := hex.EncodeToString(sum[:])
	return id, writeJSON(filepath.Join(m.dir(p.ID), id+".json"), snap)
}
func (m *Manager) load(id, ref string) (Snapshot, error) {
	var snap Snapshot
	if id == "" || id == "." || !filepath.IsLocal(id) || filepath.Base(id) != id {
		return snap, ErrTarget
	}
	if len(ref) != 64 || strings.ContainsAny(ref, "/\\") {
		return snap, ErrTarget
	}
	raw, err := os.ReadFile(filepath.Join(m.dir(id), ref+".json"))
	if err != nil {
		return snap, err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != ref {
		return snap, errors.New("checkpoint integrity check failed")
	}
	err = json.Unmarshal(raw, &snap)
	if err == nil && snap.ProjectID != id {
		err = ErrTarget
	}
	return snap, err
}

// Only these managed trees belong to a project restore. Checkpoints never snapshot themselves.
func projectFiles(workDir string) (map[string]File, error) {
	out := map[string]File{}
	for _, tree := range []string{"artifacts", "threads", "branding"} {
		root := filepath.Join(model.ProjectRoot(workDir), tree)
		values, err := files(root)
		if errors.Is(err, os.ErrNotExist) && tree != "artifacts" {
			continue
		}
		if err != nil {
			return nil, err
		}
		for name, value := range values {
			out[tree+"/"+name] = value
		}
	}
	return out, nil
}

func (m *Manager) applyFiles(ctx context.Context, p model.Project, snap Snapshot) error {
	release := pptmutation.LockProject(p.WorkDir)
	defer release()
	if err := pptmutation.CheckProjectWritable(p.WorkDir); err != nil {
		return err
	}
	if deleted, err := m.Store.ProjectDeleted(ctx, p.ID); err != nil {
		return err
	} else if deleted {
		return run.ErrRunNotFound
	}
	if pending, err := m.Store.HasPendingDeletion(ctx, p.ID); err != nil {
		return err
	} else if pending {
		return store.ErrDeletionPending
	}
	deletedThreads, err := m.Store.DeletedThreads(ctx, p.ID)
	if err != nil {
		return err
	}
	deleted := map[string]bool{}
	for _, id := range deletedThreads {
		deleted[id] = true
	}
	trees := map[string]map[string]File{"artifacts": {}, "threads": {}, "branding": {}}
	for name, value := range snap.Files {
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 || trees[parts[0]] == nil || !filepath.IsLocal(parts[1]) || excluded(parts[1]) {
			return errors.New("invalid snapshot path")
		}
		if parts[0] == "threads" && deleted[strings.SplitN(parts[1], "/", 2)[0]] {
			continue
		}
		trees[parts[0]][parts[1]] = value
	}
	for _, tree := range []string{"artifacts", "threads", "branding"} {
		if err := restoreFiles(filepath.Join(model.ProjectRoot(p.WorkDir), tree), trees[tree]); err != nil {
			return err
		}
	}
	return nil
}

func restoreFiles(root string, snapshotFiles map[string]File) error {
	// A durable database operation and preimage exist before touching any bytes.
	if info, err := os.Lstat(root); err == nil && !info.IsDir() {
		return errors.New("project restore requires a real directory")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for rel := range snapshotFiles {
		if !filepath.IsLocal(rel) || excluded(rel) {
			return errors.New("invalid snapshot path")
		}
	}
	if err := mkdirDurable(root, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !excluded(entry.Name()) {
			if err = os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	for rel, f := range snapshotFiles {
		if !filepath.IsLocal(rel) || excluded(rel) {
			return errors.New("invalid snapshot path")
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(f.Mode))
		if err != nil {
			return err
		}
		_, err = file.Write(f.Data)
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err := syncDirectories(root); err != nil {
		return err
	}
	return nil
}

// Sync newly created directory links too: syncing only a file's immediate
// directory would not persist a newly created project checkpoint parent.
func mkdirDurable(path string, mode fs.FileMode) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return errors.New("snapshot directory is not a directory")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(path)
	if err := mkdirDurable(parent, mode); err != nil {
		return err
	}
	if err := os.Mkdir(path, mode); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	dir, err := os.Open(parent)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (m *Manager) Recover(ctx context.Context, id string) error {
	if deleted, err := m.Store.ProjectDeleted(ctx, id); err != nil || deleted {
		return err
	}
	control, err := m.Store.ReadProjectHistory(ctx, id)
	if err != nil {
		return err
	}
	if control.Pending == nil {
		return nil
	}
	operation := *control.Pending
	if err := m.Store.IsolateHistoryOperation(ctx, id, operation.ID); err != nil {
		return err
	}
	m.switching.Store(id, true)
	defer m.switching.Delete(id)
	snap, err := m.load(id, operation.Before)
	if err != nil {
		return err
	}
	p := model.Project{ID: id, WorkDir: filepath.Join(m.container(id), "artifacts")}
	if err := m.applyFiles(ctx, p, snap); err != nil {
		return err
	}
	if _, err := m.Store.FinishHistoryRestore(ctx, id, operation.ID, snap.Database, snap.State, false); err != nil {
		// An acknowledgement may be lost after the restoration committed.
		latest, readErr := m.Store.ReadProjectHistory(context.WithoutCancel(ctx), id)
		if readErr == nil && latest.Pending == nil && !latest.Restoring {
			return nil
		}
		return errors.Join(err, readErr)
	}
	return nil
}
func (m *Manager) Initialize(ctx context.Context) error {
	ids, err := m.Store.PendingHistoryProjects(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if deleted, err := m.Store.ProjectDeleted(ctx, id); err != nil {
			return err
		} else if deleted {
			continue
		}
		control, err := m.Store.ReadProjectHistory(ctx, id)
		if err != nil {
			return err
		}
		if control.Pending == nil {
			continue
		}
		if err := m.Store.IsolateHistoryOperation(ctx, id, control.Pending.ID); err != nil {
			return err
		}
		if pending, err := m.Store.HasPendingDeletion(ctx, id); err != nil {
			return err
		} else if pending {
			continue
		}
		if err := m.Recover(ctx, id); err != nil {
			// The database retains restoring + preimage. Other projects may
			// start; this one cannot serve a mixed scene or launch workers.
			slog.Error("project history recovery deferred", "project_id", id, "error", err)
		}
	}
	return nil
}
func (m *Manager) idle(ctx context.Context, id string) error {
	active, err := m.Store.HasActiveRun(ctx, id)
	if err != nil {
		return err
	}
	if active {
		return ErrBusy
	}
	active, err = m.Store.HasBlockingCommand(ctx, id)
	if err != nil {
		return err
	}
	if active || (m.ExportActive != nil && m.ExportActive(id)) {
		return ErrBusy
	}
	project, err := m.Store.GetProject(ctx, id)
	if err != nil {
		return err
	}
	release := pptmutation.ReadLockProject(project.WorkDir)
	defer release()
	return pptmutation.CheckProjectWritable(project.WorkDir)
}
func (m *Manager) Baseline(ctx context.Context, p model.Project, runID, threadID string, input model.CreateRunParams) (int64, error) {
	release, ok := m.Locks.TryAcquire(p.ID)
	if !ok {
		return 0, ErrBusy
	}
	defer release()
	if err := m.idle(ctx, p.ID); err != nil {
		return 0, err
	}
	s, err := m.State(p.ID)
	if err != nil {
		return 0, err
	}
	valid := []Checkpoint{}
	for _, cp := range s.Checkpoints {
		if _, lookupErr := m.Store.GetRun(ctx, cp.RunID); lookupErr == nil {
			valid = append(valid, cp)
		}
	}
	s.Checkpoints = valid
	ref, err := m.Capture(ctx, p, s)
	if err != nil {
		return 0, err
	}
	s.Checkpoints = append(s.Checkpoints, Checkpoint{RunID: runID, ThreadID: threadID, Time: time.Now().UnixMilli(), Snapshot: ref, Input: input})
	next, err := m.Store.SaveProjectHistory(ctx, p.ID, s.SceneRevision, s.Revision, s)
	if err != nil {
		return 0, err
	}
	return next.Revision, nil
}
func (m *Manager) target(ctx context.Context, id, runID string) (State, Snapshot, *Checkpoint, error) {
	s, err := m.State(id)
	if err != nil {
		return s, Snapshot{}, nil, err
	}
	if runID == "" {
		if s.Latest == "" {
			return s, Snapshot{}, nil, ErrTarget
		}
		snap, err := m.load(id, s.Latest)
		return s, snap, nil, err
	}
	for _, cp := range s.Checkpoints {
		if cp.RunID == runID {
			r, err := m.Store.GetRun(ctx, runID)
			if err != nil || !r.Status.Terminal() {
				return s, Snapshot{}, nil, ErrTarget
			}
			snap, err := m.load(id, cp.Snapshot)
			return s, snap, &cp, err
		}
	}
	return s, Snapshot{}, nil, ErrTarget
}
func (m *Manager) Preview(ctx context.Context, id, runID string) (Preview, error) {
	release, ok := m.Locks.TryAcquire(id)
	if !ok {
		return Preview{}, ErrBusy
	}
	defer release()
	if err := m.idle(ctx, id); err != nil {
		return Preview{}, err
	}
	s, target, cp, err := m.target(ctx, id, runID)
	if err != nil {
		return Preview{}, err
	}
	v := Preview{Revision: s.Revision, SceneRevision: s.SceneRevision, Time: s.LatestTime}
	if cp != nil {
		v.Time = cp.Time
		v.Input = cp.Input.Command.Instruction
	} else if len(target.State.Scene) > 0 {
		// Restore previews show the saved active thread's draft, not the
		// rollback task's input or any draft edited after the first rollback.
		var scene struct {
			ActiveThreadID string `json:"active_thread_id"`
			Composer       struct {
				ThreadDrafts map[string]string `json:"threadDrafts"`
			} `json:"composer"`
		}
		if err := json.Unmarshal(target.State.Scene, &scene); err != nil {
			return v, err
		}
		v.Input = scene.Composer.ThreadDrafts[scene.ActiveThreadID]
	}
	currentDB, err := m.Store.CaptureProject(ctx, id)
	if err != nil {
		return v, err
	}
	// Count tasks crossing the boundary, including the rollback target itself.
	var targetRows map[string][]map[string]any
	var currentRows map[string][]map[string]any
	if err := json.Unmarshal(target.Database, &targetRows); err != nil {
		return v, err
	}
	if err := json.Unmarshal(currentDB, &currentRows); err != nil {
		return v, err
	}
	runIDs := map[string]int{}
	for _, row := range currentRows["runs"] {
		if runID, ok := row["id"].(string); ok {
			runIDs[runID]++
		}
	}
	for _, row := range targetRows["runs"] {
		if runID, ok := row["id"].(string); ok {
			runIDs[runID]--
		}
	}
	for _, delta := range runIDs {
		if delta != 0 {
			v.Runs++
		}
	}
	return v, nil
}
func (m *Manager) Switch(ctx context.Context, id, runID string, revision int64, operationID string, scene json.RawMessage) (State, error) {
	expectedScene, ok := sessionstate.RequestScene(ctx)
	if !ok {
		return State{}, sessionstate.ErrSceneChanged
	}
	if operationID == "" || len(operationID) > 128 || revision < 1 {
		return State{}, ErrConflict
	}
	raw, err := json.Marshal(struct {
		RunID                   string
		Revision, SceneRevision int64
		Scene                   json.RawMessage
	}{runID, revision, expectedScene, scene})
	if err != nil {
		return State{}, err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	if receipt, err := m.Store.ProjectHistoryReceipt(ctx, id, operationID, hash); err != nil {
		return State{}, err
	} else if receipt != nil {
		return *receipt, nil
	}
	release, ok := m.Locks.TryAcquire(id)
	if !ok {
		return State{}, ErrBusy
	}
	defer release()
	s, err := m.State(id)
	if err != nil {
		return s, err
	}
	if s.SceneRevision != expectedScene {
		return s, sessionstate.ErrSceneChanged
	}
	if revision != s.Revision {
		return s, ErrConflict
	}
	if err := m.idle(ctx, id); err != nil {
		return s, err
	}
	s, target, cp, err := m.target(ctx, id, runID)
	if err != nil {
		return s, err
	}
	p, err := m.Store.GetProject(ctx, id)
	if err != nil {
		return s, err
	}
	beforeState := s
	beforeState.Scene = scene
	before, err := m.Capture(ctx, p, beforeState)
	if err != nil {
		return s, err
	}
	next := target.State
	if cp != nil {
		next.Latest, next.LatestTime = s.Latest, s.LatestTime
		if next.Latest == "" {
			next.Latest, next.LatestTime = before, time.Now().UnixMilli()
		}
		next.Scene, _ = json.Marshal(map[string]any{"thread_id": cp.ThreadID, "input": cp.Input})
	} else {
		next.Latest, next.LatestTime = "", 0
	}
	pending := sessionstate.HistoryOperation{ID: operationID, Kind: "switch", RequestHash: hash, SceneRevision: s.SceneRevision, HistoryRevision: s.Revision, Before: before, Next: next}
	if cp != nil {
		pending.Target = cp.Snapshot
	} else {
		pending.Target = s.Latest
	}
	if err := m.Store.BeginHistoryOperation(ctx, id, pending); err != nil {
		return s, err
	}
	m.switching.Store(id, true)
	defer m.switching.Delete(id)
	commitCtx := context.WithoutCancel(ctx)
	if err = m.applyFiles(commitCtx, p, target); err == nil {
		next, err = m.Store.FinishHistoryRestore(commitCtx, id, operationID, target.Database, State{}, true)
	}
	if err != nil {
		if receipt, lookupErr := m.Store.ProjectHistoryReceipt(commitCtx, id, operationID, hash); lookupErr == nil && receipt != nil {
			return *receipt, nil
		}
		return s, errors.Join(err, m.Recover(commitCtx, id))
	}
	return next, nil
}

// A future-discarding mutation pins its preimage before any authoring work.
// The HTTP start barrier commits acceptance before asynchronous work may run.
func (m *Manager) BeginMutation(ctx context.Context, id string, revision int64) (func(bool) error, error) {
	s, err := m.State(id)
	if err != nil {
		return nil, err
	}
	if s.Latest != "" && s.Revision != revision {
		return nil, ErrConflict
	}
	var operationID string
	if s.Latest != "" {
		if err := m.idle(ctx, id); err != nil {
			return nil, err
		}
		p, err := m.Store.GetProject(ctx, id)
		if err != nil {
			return nil, err
		}
		before, err := m.Capture(ctx, p, s)
		if err != nil {
			return nil, err
		}
		operationID = uuid.NewString()
		operation := sessionstate.HistoryOperation{ID: operationID, Kind: "mutation", RequestHash: operationID, SceneRevision: s.SceneRevision, HistoryRevision: s.Revision, Before: before, Next: s}
		if err := m.Store.BeginHistoryOperation(ctx, id, operation); err != nil {
			return nil, err
		}
	}
	settled := false
	return func(success bool) error {
		if settled {
			return nil
		}
		commitCtx := context.WithoutCancel(ctx)
		if !success {
			if operationID != "" {
				if err := m.Recover(commitCtx, id); err != nil {
					return err
				}
			}
			settled = true
			return nil
		}
		if operationID != "" {
			if _, err := m.Store.FinishHistoryMutation(commitCtx, id, operationID); err != nil {
				receipt, lookupErr := m.Store.ProjectHistoryReceipt(commitCtx, id, operationID, operationID)
				if lookupErr != nil || receipt == nil {
					return errors.Join(err, lookupErr)
				}
			}
		} else {
			current, err := m.Store.ReadProjectHistory(commitCtx, id)
			if err != nil {
				return err
			}
			if current.State.SceneRevision != s.SceneRevision {
				return sessionstate.ErrSceneChanged
			}
			next := current.State
			next.Latest, next.LatestTime = "", 0
			if _, err := m.Store.SaveProjectHistory(commitCtx, id, s.SceneRevision, current.State.Revision, next); err != nil {
				return err
			}
		}
		settled = true
		return nil
	}, nil
}

// Persist directory entries as well as file bytes before publishing DB state.
func syncDirectories(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if excluded(rel) && d.IsDir() {
			return filepath.SkipDir
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		d, err := os.Open(dirs[i])
		if err != nil {
			return err
		}
		err = d.Sync()
		_ = d.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// Collect removes unreachable snapshots only after the authoritative state is
// durable. Current baselines and the single live restore point pin their bytes.
func (m *Manager) Collect(id string) error {
	control, err := m.Store.ReadProjectHistory(context.Background(), id)
	if err != nil {
		return err
	}
	if control.Pending != nil || control.Restoring {
		return nil
	}
	state := control.State
	keep := map[string]bool{}
	var visit func(string) error
	visit = func(ref string) error {
		if ref == "" || keep[ref] {
			return nil
		}
		keep[ref] = true
		snap, err := m.load(id, ref)
		if err != nil {
			return err
		}
		for _, cp := range snap.State.Checkpoints {
			if err = visit(cp.Snapshot); err != nil {
				return err
			}
		}
		return visit(snap.State.Latest)
	}
	for _, cp := range state.Checkpoints {
		if err = visit(cp.Snapshot); err != nil {
			return err
		}
	}
	if err = visit(state.Latest); err != nil {
		return err
	}
	entries, err := os.ReadDir(m.dir(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		ref := strings.TrimSuffix(entry.Name(), ".json")
		if len(ref) == 64 && !keep[ref] {
			if err = os.Remove(filepath.Join(m.dir(id), entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

type snapshotGuardKey struct{}

func WithSnapshotGuardHeld(ctx context.Context, projectID string) context.Context {
	return context.WithValue(ctx, snapshotGuardKey{}, projectID)
}
