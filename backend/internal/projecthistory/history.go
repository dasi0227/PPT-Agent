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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/google/uuid"
)

var ErrConflict = errors.New("项目历史已变化，请重新确认")
var ErrBusy = errors.New("项目存在未结束任务或冲突操作，请结束后重试")
var ErrTarget = errors.New("该任务不在当前可回退历史中")

type SnapshotStore interface {
	store.Store
	CaptureProject(context.Context, string) (json.RawMessage, error)
	RestoreProject(context.Context, string, json.RawMessage) error
}
type Checkpoint struct {
	RunID    string                `json:"run_id"`
	ThreadID string                `json:"thread_id"`
	Time     int64                 `json:"time"`
	Sequence int64                 `json:"sequence"`
	Snapshot string                `json:"snapshot"`
	Input    model.CreateRunParams `json:"input"`
}
type State struct {
	SceneRevision int64           `json:"scene_revision"`
	Revision      int64           `json:"revision"`
	Sequence      int64           `json:"sequence"`
	Checkpoints   []Checkpoint    `json:"checkpoints"`
	Latest        string          `json:"latest,omitempty"`
	LatestTime    int64           `json:"latest_time,omitempty"`
	Scene         json.RawMessage `json:"scene,omitempty"`
	LastOperation string          `json:"last_operation,omitempty"`
	LastCommand   string          `json:"last_command,omitempty"`
}
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
	Revision int64    `json:"revision"`
	Time     int64    `json:"time"`
	Input    string   `json:"input"`
	Threads  int      `json:"threads"`
	Runs     int      `json:"runs"`
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted"`
}
type Journal struct {
	Before    string `json:"before"`
	Operation string `json:"operation"`
}
type Manager struct {
	Store        SnapshotStore
	Locks        *run.LockManager
	ExportActive func(string) bool
	root         string
	gates        sync.Map
}

func New(s SnapshotStore, locks *run.LockManager, root string) *Manager {
	return &Manager{Store: s, Locks: locks, root: filepath.Join(root, "project-history")}
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
func (m *Manager) dir(id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(m.root, hex.EncodeToString(sum[:]))
}
func writeJSON(path string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
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
	var s State
	raw, err := os.ReadFile(filepath.Join(m.dir(id), "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return State{Revision: 1, Checkpoints: []Checkpoint{}}, nil
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(raw, &s)
	return s, err
}
func (m *Manager) Save(id string, s State) error {
	return writeJSON(filepath.Join(m.dir(id), "state.json"), s)
}
func excluded(path string) bool {
	first := strings.Split(filepath.ToSlash(path), "/")[0]
	return first == ".git" || first == ".runtime" || first == ".run" || first == ".commit-tmp"
}
func files(root string) (map[string]File, error) {
	out := map[string]File{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
	f, err := files(p.WorkDir)
	if err != nil {
		return "", err
	}
	db, err := m.Store.CaptureProject(ctx, p.ID)
	if err != nil {
		return "", err
	}
	snap := Snapshot{p.ID, db, f, s}
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
func (m *Manager) apply(ctx context.Context, p model.Project, snap Snapshot) error {
	// A durable journal and preimage exist before touching any active bytes.
	for rel := range snap.Files {
		if !filepath.IsLocal(rel) || excluded(rel) {
			return errors.New("invalid snapshot path")
		}
	}
	if err := os.MkdirAll(p.WorkDir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(p.WorkDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !excluded(entry.Name()) {
			if err = os.RemoveAll(filepath.Join(p.WorkDir, entry.Name())); err != nil {
				return err
			}
		}
	}
	for rel, f := range snap.Files {
		if !filepath.IsLocal(rel) || excluded(rel) {
			return errors.New("invalid snapshot path")
		}
		path := filepath.Join(p.WorkDir, filepath.FromSlash(rel))
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
	if err := syncDirectories(p.WorkDir); err != nil {
		return err
	}
	return m.Store.RestoreProject(ctx, p.ID, snap.Database)
}
func (m *Manager) Recover(ctx context.Context, id string) error {
	path := filepath.Join(m.dir(id), "journal.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var journal Journal
	if err = json.Unmarshal(raw, &journal); err != nil {
		return err
	}
	s, err := m.State(id)
	if err != nil {
		return err
	}
	if s.LastOperation != journal.Operation {
		snap, err := m.load(id, journal.Before)
		if err != nil {
			return err
		}
		p, err := m.Store.GetProject(ctx, id)
		if err != nil {
			return err
		}
		if err = m.apply(ctx, p, snap); err != nil {
			return err
		}
		if err = m.Save(id, snap.State); err != nil {
			return err
		}
	}
	return os.Remove(path)
}
func (m *Manager) Initialize(ctx context.Context) error {
	projects, err := m.Store.ListProjects(ctx)
	if err != nil {
		return err
	}
	for _, p := range projects {
		if err = m.Recover(ctx, p.ID); err != nil {
			return err
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
	active, err = m.Store.HasActiveGitCommit(ctx, id)
	if err != nil {
		return err
	}
	if active || (m.ExportActive != nil && m.ExportActive(id)) {
		return ErrBusy
	}
	return nil
}
func (m *Manager) Baseline(ctx context.Context, p model.Project, runID, threadID string, input model.CreateRunParams) error {
	release, ok := m.Locks.TryAcquire(p.ID)
	if !ok {
		return ErrBusy
	}
	defer release()
	if err := m.idle(ctx, p.ID); err != nil {
		return err
	}
	s, err := m.State(p.ID)
	if err != nil {
		return err
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
		return err
	}
	s.Sequence++
	s.Revision++
	s.Checkpoints = append(s.Checkpoints, Checkpoint{runID, threadID, time.Now().UnixMilli(), s.Sequence, ref, input})
	return m.Save(p.ID, s)
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
	p, err := m.Store.GetProject(ctx, id)
	if err != nil {
		return Preview{}, err
	}
	current, err := files(p.WorkDir)
	if err != nil {
		return Preview{}, err
	}
	v := Preview{Revision: s.Revision, Time: s.LatestTime, Added: []string{}, Modified: []string{}, Deleted: []string{}}
	if cp != nil {
		v.Time = cp.Time
		v.Input = cp.Input.Command.Instruction
	}
	for name, f := range target.Files {
		old, ok := current[name]
		if !ok {
			v.Added = append(v.Added, name)
		} else if string(old.Data) != string(f.Data) {
			v.Modified = append(v.Modified, name)
		}
	}
	for name := range current {
		if _, ok := target.Files[name]; !ok {
			v.Deleted = append(v.Deleted, name)
		}
	}
	threads, err := m.Store.ListThreads(ctx, id)
	if err != nil {
		return v, err
	}
	v.Threads = len(threads)
	v.Runs = len(s.Checkpoints) - len(target.State.Checkpoints)
	if v.Runs < 0 {
		v.Runs = -v.Runs
	}
	return v, nil
}
func (m *Manager) Switch(ctx context.Context, id, runID string, revision int64, operation string, scene json.RawMessage) (State, error) {
	release, ok := m.Locks.TryAcquire(id)
	if !ok {
		return State{}, ErrBusy
	}
	defer release()
	command := runID
	if command == "" {
		command = "restore"
	}
	s, err := m.State(id)
	if err != nil {
		return s, err
	}
	if operation == "" {
		return s, ErrConflict
	}
	if s.LastOperation == operation {
		if s.LastCommand != command {
			return s, ErrConflict
		}
		return s, nil
	}
	if revision != s.Revision {
		return s, ErrConflict
	}
	if err = m.idle(ctx, id); err != nil {
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
	next.Revision = s.Revision + 1
	next.Sequence = s.Sequence
	next.LastOperation = operation
	next.LastCommand = command
	next.SceneRevision = next.Revision
	if cp != nil {
		next.Latest = s.Latest
		next.LatestTime = s.LatestTime
		if next.Latest == "" {
			next.Latest = before
			next.LatestTime = time.Now().UnixMilli()
		}
		raw, _ := json.Marshal(map[string]any{"thread_id": cp.ThreadID, "input": cp.Input})
		next.Scene = raw
	} else {
		next.Latest = ""
		next.LatestTime = 0
	}
	if err = writeJSON(filepath.Join(m.dir(id), "journal.json"), Journal{before, operation}); err != nil {
		return s, err
	}
	if err = m.apply(context.WithoutCancel(ctx), p, target); err == nil {
		err = m.Save(id, next)
	}
	if err != nil {
		recoveryErr := m.Recover(context.WithoutCancel(ctx), id)
		return s, errors.Join(err, recoveryErr)
	}
	if err = m.Recover(context.WithoutCancel(ctx), id); err != nil {
		return next, err
	}
	return next, nil
}

// Mutation preimages make cancellation/validation failure preserve the only future.
func (m *Manager) BeginMutation(ctx context.Context, id string, revision int64) (func(bool) error, error) {
	s, err := m.State(id)
	if err != nil {
		return nil, err
	}
	if s.Latest != "" && s.Revision != revision {
		return nil, ErrConflict
	}
	var before, op string
	if s.Latest != "" {
		if err = m.idle(ctx, id); err != nil {
			return nil, err
		}
		p, err := m.Store.GetProject(ctx, id)
		if err != nil {
			return nil, err
		}
		before, err = m.Capture(ctx, p, s)
		if err != nil {
			return nil, err
		}
		op = uuid.NewString()
		if err = writeJSON(filepath.Join(m.dir(id), "journal.json"), Journal{before, op}); err != nil {
			return nil, err
		}
	}
	return func(success bool) error {
		if !success {
			if before != "" {
				return m.Recover(context.WithoutCancel(ctx), id)
			}
			return nil
		}
		next, err := m.State(id)
		if err != nil {
			return err
		}
		next.Revision++
		next.Sequence++
		next.Latest = ""
		next.LatestTime = 0
		if before != "" {
			next.LastOperation = op
			next.LastCommand = "mutation"
		}
		if err = m.Save(id, next); err != nil {
			return err
		}
		if before != "" {
			return m.Recover(context.WithoutCancel(ctx), id)
		}
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
	if _, err := os.Stat(filepath.Join(m.dir(id), "journal.json")); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	state, err := m.State(id)
	if err != nil {
		return err
	}
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
		return nil
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
