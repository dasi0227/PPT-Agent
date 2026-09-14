package projecthistory

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"go.uber.org/zap"
)

func fixture(t *testing.T) (*Manager, model.Project) {
	t.Helper()
	root := t.TempDir()
	db, close, err := sqlite.Open(&config.Config{DBPath: filepath.Join(root, "db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(close)
	st, err := sqlite.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	p := model.Project{ID: "p1", Title: "present", WorkDir: filepath.Join(root, "projects", "p1"), Status: "draft", Theme: "default", LayoutVersion: 6, CreatedAt: 1, UpdatedAt: 1}
	must(t, st.CreateProject(context.Background(), p))
	must(t, os.MkdirAll(p.WorkDir, 0755))
	must(t, st.CreateThread(context.Background(), model.Thread{ID: "t1", ProjectID: p.ID, Status: "active", CreatedAt: 1, UpdatedAt: 1, HistoryPath: "threads/t1.jsonl"}))
	return New(st, run.NewLockManager(), root), p
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func put(t *testing.T, p model.Project, path, text string) {
	t.Helper()
	full := filepath.Join(p.WorkDir, path)
	must(t, os.MkdirAll(filepath.Dir(full), 0755))
	must(t, os.WriteFile(full, []byte(text), 0644))
}
func content(t *testing.T, p model.Project, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(p.WorkDir, path))
	must(t, err)
	if string(raw) != want {
		t.Fatalf("%s = %q, want %q", path, raw, want)
	}
}
func cp(t *testing.T, m *Manager, p model.Project, id string, status model.RunStatus) {
	t.Helper()
	ctx := context.Background()
	cmd := model.RunCommand{Instruction: id, Mode: model.ModeChat, Scope: model.NewRunScope(model.ScopeObjectSpec, model.ScopeAllPages)}
	revision, err := m.Baseline(ctx, p, id, "t1", model.CreateRunParams{Command: cmd})
	must(t, err)
	state, err := m.State(p.ID)
	must(t, err)
	if revision < 1 || revision != state.Revision {
		t.Fatalf("baseline revision=%d state revision=%d", revision, state.Revision)
	}
	must(t, m.Store.CreateRun(ctx, model.Run{ID: id, ProjectID: p.ID, ThreadID: "t1", Status: status, Command: cmd}))
}
func switchTo(t *testing.T, m *Manager, p model.Project, id, op string) State {
	t.Helper()
	s, err := m.State(p.ID)
	must(t, err)
	next, err := m.Switch(context.Background(), p.ID, id, s.Revision, op, json.RawMessage(`{"draft":"original"}`))
	must(t, err)
	return next
}
func TestWholeProjectSingleFuture(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	put(t, p, "outline.json", "initial")
	put(t, p, "threads/t1.transcript.jsonl", "before first")
	put(t, p, ".git/HEAD", "real git")
	cp(t, m, p, "cp1", model.RunDone)
	put(t, p, "outline.json", "manual before cp2")
	cp(t, m, p, "cp2", model.RunFailed)
	put(t, p, "threads/t1.transcript.jsonl", "future compacted summary")
	must(t, m.Store.CreateThread(ctx, model.Thread{ID: "t2", ProjectID: p.ID, Status: "active", CreatedAt: 1, UpdatedAt: 1, HistoryPath: "threads/t2.jsonl"}))
	put(t, p, "threads/t2.jsonl", "future conversation")
	cp(t, m, p, "cp3", model.RunCanceled)
	cp(t, m, p, "cp4", model.RunDone)
	put(t, p, "outline.json", "manual latest")
	put(t, p, "attachments/new/original.png", "bytes")
	s := switchTo(t, m, p, "cp2", "rollback2")
	content(t, p, "outline.json", "manual before cp2")
	content(t, p, "threads/t1.transcript.jsonl", "before first")
	if _, err := m.Store.GetRun(ctx, "cp2"); err == nil {
		t.Fatal("target execution remains visible")
	}
	threads, err := m.Store.ListThreads(ctx, p.ID)
	must(t, err)
	if len(threads) != 1 {
		t.Fatal("future thread visible")
	}
	if _, err = m.Preview(ctx, p.ID, "cp3"); !errors.Is(err, ErrTarget) {
		t.Fatalf("hidden target accepted: %v", err)
	}
	replay, err := m.Switch(ctx, p.ID, "cp2", s.Revision-1, "rollback2", nil)
	must(t, err)
	if replay.Revision != s.Revision {
		t.Fatal("duplicate switch")
	}
	s = switchTo(t, m, p, "cp1", "rollback1")
	content(t, p, "outline.json", "initial")
	if len(s.Checkpoints) != 0 {
		t.Fatal("first run should remove all runs")
	}
	restarted := New(m.Store, m.Locks, filepath.Dir(m.root))
	must(t, restarted.Initialize(ctx))
	s = switchTo(t, restarted, p, "", "restore")
	content(t, p, "outline.json", "manual latest")
	content(t, p, "threads/t1.transcript.jsonl", "future compacted summary")
	content(t, p, "attachments/new/original.png", "bytes")
	content(t, p, ".git/HEAD", "real git")
	if string(s.Scene) != `{"draft":"original"}` {
		t.Fatalf("draft not restored: %s", s.Scene)
	}
	if _, err = m.Store.GetRun(ctx, "cp3"); err != nil {
		t.Fatal(err)
	}
	switchTo(t, m, p, "cp3", "rollback3")
}
func TestConflictAndMutationFailurePreservesFuture(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	put(t, p, "file", "before")
	cp(t, m, p, "cp1", model.RunDone)
	put(t, p, "file", "latest")
	s := switchTo(t, m, p, "cp1", "back")
	if _, err := m.Switch(ctx, p.ID, "", s.Revision-1, "stale", nil); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	finish, err := m.BeginMutation(ctx, p.ID, s.Revision)
	must(t, err)
	put(t, p, "file", "bad partial mutation")
	must(t, finish(false))
	content(t, p, "file", "before")
	state, _ := m.State(p.ID)
	if state.Latest == "" {
		t.Fatal("failed validation lost future")
	}
	finish, err = m.BeginMutation(ctx, p.ID, state.Revision)
	must(t, err)
	put(t, p, "file", "new branch")
	must(t, finish(true))
	if _, err = m.Preview(ctx, p.ID, ""); !errors.Is(err, ErrTarget) {
		t.Fatal("discarded future restored")
	}
	content(t, p, "file", "new branch")
}
func TestCrashJournalAndIntegrity(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	put(t, p, "file", "before")
	cp(t, m, p, "cp1", model.RunDone)
	put(t, p, "file", "live")
	s, _ := m.State(p.ID)
	before, err := m.Capture(ctx, p, s)
	must(t, err)
	must(t, writeJSON(filepath.Join(m.dir(p.ID), "journal.json"), Journal{before, "crashed"}))
	put(t, p, "file", "half switched")
	must(t, m.Store.DeleteThread(ctx, "t1"))
	must(t, m.Initialize(ctx))
	content(t, p, "file", "live")
	if _, err = m.Store.GetRun(ctx, "cp1"); err != nil {
		t.Fatal(err)
	}
	cpref := s.Checkpoints[0].Snapshot
	must(t, os.WriteFile(filepath.Join(m.dir(p.ID), cpref+".json"), []byte(`{}`), 0600))
	if _, err = m.Switch(ctx, p.ID, "cp1", s.Revision, "corrupt", nil); err == nil {
		t.Fatal("corrupt snapshot accepted")
	}
	content(t, p, "file", "live")
}
func TestBusyAndSymlink(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	cp(t, m, p, "cp1", model.RunPaused)
	for _, status := range []model.RunStatus{model.RunPending, model.RunRunning, model.RunWaiting, model.RunPaused, model.RunRecovering} {
		must(t, m.Store.SetRunStatus(ctx, "cp1", status))
		if _, err := m.Preview(ctx, p.ID, "cp1"); !errors.Is(err, ErrBusy) {
			t.Fatalf("status %s: %v", status, err)
		}
	}
	must(t, m.Store.SetRunStatus(ctx, "cp1", model.RunDone))
	release, _ := m.Locks.TryAcquire(p.ID)
	s, _ := m.State(p.ID)
	if _, err := m.Switch(ctx, p.ID, "cp1", s.Revision, "busy", nil); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	release()
	must(t, os.Symlink(t.TempDir(), filepath.Join(p.WorkDir, "outside")))
	if _, err := m.Capture(ctx, p, s); err == nil {
		t.Fatal("symlink followed")
	}
}

type failingRestore struct {
	SnapshotStore
	fail bool
}

func (s *failingRestore) RestoreProject(ctx context.Context, id string, raw json.RawMessage) error {
	if s.fail {
		s.fail = false
		return errors.New("injected disk/database failure")
	}
	return s.SnapshotStore.RestoreProject(ctx, id, raw)
}
func TestDatabaseFailureRestoresFilesAndLatestEligibility(t *testing.T) {
	m, p := fixture(t)
	put(t, p, "file", "before")
	cp(t, m, p, "cp1", model.RunDone)
	put(t, p, "file", "latest")
	s, _ := m.State(p.ID)
	m.Store = &failingRestore{m.Store, true}
	if _, err := m.Switch(context.Background(), p.ID, "cp1", s.Revision, "fail", nil); err == nil {
		t.Fatal("injected failure ignored")
	}
	content(t, p, "file", "latest")
	if _, err := m.Store.GetRun(context.Background(), "cp1"); err != nil {
		t.Fatal(err)
	}
	after, _ := m.State(p.ID)
	if after.Revision != s.Revision || after.Latest != "" {
		t.Fatal("failed operation published history")
	}
}
func TestPreviewBusyExportAndSequence(t *testing.T) {
	m, p := fixture(t)
	cp(t, m, p, "cp1", model.RunDone)
	cp(t, m, p, "cp2", model.RunDone)
	s, _ := m.State(p.ID)
	if s.Checkpoints[0].Sequence >= s.Checkpoints[1].Sequence {
		t.Fatal("time anchors not ordered")
	}
	m.ExportActive = func(string) bool { return true }
	if _, err := m.Preview(context.Background(), p.ID, "cp1"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
}

func TestInterruptedDeletionRecoversWithoutProjectRow(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	put(t, p, "file", "before")
	cp(t, m, p, "cp1", model.RunDone)
	put(t, p, "file", "latest")
	s := switchTo(t, m, p, "cp1", "back")
	_, err := m.BeginMutation(ctx, p.ID, s.Revision)
	must(t, err)
	must(t, m.Store.DeleteProject(ctx, p.ID))
	must(t, os.RemoveAll(p.WorkDir))
	// No finish callback: recreate the manager as at process startup.
	restarted := New(m.Store, m.Locks, filepath.Dir(m.root))
	must(t, restarted.Initialize(ctx))
	content(t, p, "file", "before")
	if _, err := m.Store.GetProject(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	switchTo(t, restarted, p, "", "restore")
	content(t, p, "file", "latest")
}

func TestCommittedMutationSurvivesRestartAndCannotReopenFuture(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	cp(t, m, p, "cp1", model.RunDone)
	s := switchTo(t, m, p, "cp1", "back")
	finish, err := m.BeginMutation(ctx, p.ID, s.Revision)
	must(t, err)
	raw, err := os.ReadFile(filepath.Join(m.dir(p.ID), "journal.json"))
	must(t, err)
	put(t, p, "file", "accepted branch")
	must(t, finish(true))
	accepted, _ := m.State(p.ID)
	must(t, finish(false)) // A later handler failure cannot undo accepted work.
	must(t, os.WriteFile(filepath.Join(m.dir(p.ID), "journal.json"), raw, 0600))
	restarted := New(m.Store, m.Locks, filepath.Dir(m.root))
	must(t, restarted.Initialize(ctx))
	content(t, p, "file", "accepted branch")
	after, _ := m.State(p.ID)
	if after.Revision != accepted.Revision || after.Latest != "" {
		t.Fatal("accepted branch resurrected its future")
	}
}

func TestTranscriptLoaderSeesOnlyActiveBoundary(t *testing.T) {
	m, p := fixture(t)
	transcript := contextengine.NewFSTranscriptStore()
	messages := func(text string) []llm.Message {
		return []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentPart{{Type: "text", Text: text}}}}
	}
	must(t, transcript.Replace(p.WorkDir, "t1", messages("before compaction")))
	cp(t, m, p, "cp1", model.RunDone)
	must(t, transcript.Replace(p.WorkDir, "t1", messages("future compacted summary")))
	switchTo(t, m, p, "cp1", "back")
	loaded, err := transcript.Load(p.WorkDir, "t1")
	must(t, err)
	if len(loaded) != 1 || loaded[0].Text() != "before compaction" {
		t.Fatalf("future leaked: %+v", loaded)
	}
	switchTo(t, m, p, "", "restore")
	loaded, err = transcript.Load(p.WorkDir, "t1")
	must(t, err)
	if len(loaded) != 1 || loaded[0].Text() != "future compacted summary" {
		t.Fatalf("summary not restored: %+v", loaded)
	}
}

func TestSnapshotWriteFailureAndConcurrentGate(t *testing.T) {
	m, p := fixture(t)
	put(t, p, "file", "live")
	must(t, os.MkdirAll(m.root, 0700))
	must(t, os.WriteFile(m.dir(p.ID), []byte("not a directory"), 0600))
	if _, err := m.Baseline(context.Background(), p, "cp1", "t1", model.CreateRunParams{}); err == nil {
		t.Fatal("snapshot write failure ignored")
	}
	content(t, p, "file", "live")
	release := m.Gate(p.ID)
	if unlock, ok := m.TryGate(p.ID); ok {
		unlock()
		t.Fatal("concurrent mutation admitted")
	}
	release()
}
