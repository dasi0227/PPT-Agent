package projecthistory

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"go.uber.org/zap"
)

func TestPendingArtifactJournalBlocksHistoryAfterRunCancellation(t *testing.T) {
	m, p := fixture(t)
	put(t, p, ".outline.json", "before")
	cp(t, m, p, "canceled", model.RunCanceled)
	put(t, p, ".outline.json", "unconfirmed")
	put(t, p, ".runtime/mutations/pending.json", `{}`)
	state, err := m.State(p.ID)
	must(t, err)
	ctx := sessionstate.WithRequestScene(context.Background(), state.SceneRevision)
	if _, err := m.Switch(ctx, p.ID, "canceled", state.Revision, "switch", nil); !errors.Is(err, pptmutation.ErrRecoveryRequired) {
		t.Fatalf("history switch bypassed unresolved artifact: %v", err)
	}
	control, err := m.Store.ReadProjectHistory(ctx, p.ID)
	must(t, err)
	if control.Pending != nil || control.Restoring {
		t.Fatal("blocked switch created a history operation")
	}
	if err := m.applyFiles(ctx, p, Snapshot{Files: map[string]File{"artifacts/.outline.json": {Data: []byte("overwritten"), Mode: 0644}}}); !errors.Is(err, pptmutation.ErrRecoveryRequired) {
		t.Fatalf("history file restore bypassed unresolved artifact: %v", err)
	}
	content(t, p, ".outline.json", "unconfirmed")
	must(t, os.Remove(filepath.Join(p.WorkDir, ".runtime", "mutations", "pending.json")))
	if _, err := m.Switch(ctx, p.ID, "canceled", state.Revision, "switch", nil); err != nil {
		t.Fatalf("cleared artifact journal kept history blocked: %v", err)
	}
	content(t, p, ".outline.json", "before")
}

func fixture(t *testing.T) (*Manager, model.Project) {
	t.Helper()
	root := t.TempDir()
	db, close, err := sqlite.Open(&config.Config{WorkRoot: root, DBPath: filepath.Join(root, "db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(close)
	st, err := sqlite.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	p := model.Project{ID: "p1", Title: "present", WorkDir: filepath.Join(root, "projects", "p1", "artifacts"), Theme: "default", CreatedAt: 1, UpdatedAt: 1}
	must(t, st.CreateProject(context.Background(), p))
	must(t, os.MkdirAll(p.WorkDir, 0755))
	must(t, os.MkdirAll(filepath.Join(model.ProjectRoot(p.WorkDir), "threads", "t1"), 0755))
	must(t, st.CreateThread(context.Background(), model.Thread{ID: "t1", ProjectID: p.ID, CreatedAt: 1, UpdatedAt: 1}))
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
	full := projectPath(p, path)
	must(t, os.MkdirAll(filepath.Dir(full), 0755))
	must(t, os.WriteFile(full, []byte(text), 0644))
}
func content(t *testing.T, p model.Project, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(projectPath(p, path))
	must(t, err)
	if string(raw) != want {
		t.Fatalf("%s = %q, want %q", path, raw, want)
	}
}
func projectPath(p model.Project, path string) string {
	if strings.HasPrefix(filepath.ToSlash(path), "threads/") {
		return filepath.Join(model.ProjectRoot(p.WorkDir), filepath.FromSlash(path))
	}
	return filepath.Join(p.WorkDir, filepath.FromSlash(path))
}
func cp(t *testing.T, m *Manager, p model.Project, id string, status model.RunStatus) {
	t.Helper()
	ctx := context.Background()
	cmd := model.RunCommand{Instruction: id, Mode: model.ModeChat, Scope: model.NewRunScope(model.ScopeAllPages)}
	revision, err := m.Baseline(ctx, p, id, "t1", model.CreateRunParams{Command: cmd})
	must(t, err)
	state, err := m.State(p.ID)
	must(t, err)
	if revision < 1 || revision != state.Revision {
		t.Fatalf("baseline revision=%d state revision=%d", revision, state.Revision)
	}
	must(t, m.Store.CreateRun(ctx, model.Run{ID: id, ProjectID: p.ID, ThreadID: "t1", SceneRevision: state.SceneRevision, ClientRequestID: id, Status: status, Command: cmd}))
}
func switchTo(t *testing.T, m *Manager, p model.Project, id, op string) State {
	t.Helper()
	s, err := m.State(p.ID)
	must(t, err)
	next, err := m.Switch(sessionstate.WithRequestScene(context.Background(), s.SceneRevision), p.ID, id, s.Revision, op, json.RawMessage(`{"draft":"original"}`))
	must(t, err)
	return next
}
func TestWholeProjectSingleFuture(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	put(t, p, ".outline.json", "initial")
	replaceContext(t, m, p, "t1", "before first")
	put(t, p, ".git/HEAD", "real git")
	put(t, p, "versions/legacy.html", "untouched legacy")
	must(t, m.Store.ReplaceSlides(ctx, p.ID, []model.Slide{{ID: "slide", ProjectID: p.ID}}))
	cp(t, m, p, "cp1", model.RunRunning)
	put(t, p, ".outline.json", "manual before cp2")
	artifactCtx := workflow.WithCheckpointLease(sessionstate.WithRequestScene(ctx, 1), "", 1, 0)
	must(t, m.Store.CommitWorkflow(artifactCtx, model.ArtifactCommit{ProjectID: p.ID, RunID: "cp1", OperationID: "html", RequestHash: "hash"}))
	must(t, m.Store.SetRunStatus(ctx, "cp1", model.RunDone))
	cp(t, m, p, "cp2", model.RunRunning)
	must(t, m.Store.ReplaceSlides(ctx, p.ID, nil))
	must(t, m.Store.CommitWorkflow(artifactCtx, model.ArtifactCommit{ProjectID: p.ID, RunID: "cp2", OperationID: "delete", RequestHash: "hash"}))
	must(t, m.Store.SetRunStatus(ctx, "cp2", model.RunFailed))
	replaceContext(t, m, p, "t1", "future compacted summary")
	must(t, m.Store.CreateThread(ctx, model.Thread{ID: "t2", ProjectID: p.ID, CreatedAt: 1, UpdatedAt: 1}))
	replaceContext(t, m, p, "t2", "future conversation")
	cp(t, m, p, "cp3", model.RunCanceled)
	cp(t, m, p, "cp4", model.RunDone)
	put(t, p, ".outline.json", "manual latest")
	put(t, p, "attachments/new.png", "bytes")
	preview, err := m.Preview(ctx, p.ID, "cp2")
	must(t, err)
	if preview.Runs != 3 || preview.Input != "cp2" {
		t.Fatalf("target-inclusive preview: %+v", preview)
	}
	before, err := m.State(p.ID)
	must(t, err)
	for _, checkpoint := range before.Checkpoints {
		snap, err := m.load(p.ID, checkpoint.Snapshot)
		must(t, err)
		for path := range snap.Files {
			if strings.HasPrefix(path, "artifacts/versions/") {
				t.Fatal("legacy copies included in checkpoint")
			}
		}
	}
	s := switchTo(t, m, p, "cp2", "rollback2")
	slide, err := m.Store.GetSlide(ctx, "slide")
	if err != nil || slide.ProjectID != p.ID {
		t.Fatalf("restored identity: %+v %v", slide, err)
	}
	if _, err := m.Store.GetIdempotency(ctx, "artifact_commit", "cp1", "html"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Store.GetIdempotency(ctx, "artifact_commit", "cp2", "delete"); err == nil {
		t.Fatal("future receipt survived")
	}
	content(t, p, "versions/legacy.html", "untouched legacy")
	content(t, p, ".outline.json", "manual before cp2")
	assertContextText(t, m, "t1", "future compacted summary", false)
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
	replay, err := m.Switch(sessionstate.WithRequestScene(ctx, before.SceneRevision), p.ID, "cp2", before.Revision, "rollback2", json.RawMessage(`{"draft":"original"}`))
	must(t, err)
	if replay.Revision != s.Revision {
		t.Fatal("duplicate switch")
	}
	preview, err = m.Preview(ctx, p.ID, "cp1")
	must(t, err)
	if preview.Runs != 1 {
		t.Fatalf("continuous rollback counted hidden future: %+v", preview)
	}
	s = switchTo(t, m, p, "cp1", "rollback1")
	slide, err = m.Store.GetSlide(ctx, "slide")
	if err != nil || slide.ProjectID != p.ID {
		t.Fatalf("initial identity: %+v %v", slide, err)
	}
	if _, err := m.Store.GetIdempotency(ctx, "artifact_commit", "cp1", "html"); err == nil {
		t.Fatal("continuous rollback retained receipt")
	}
	content(t, p, ".outline.json", "initial")
	if len(s.Checkpoints) != 0 {
		t.Fatal("first run should remove all runs")
	}
	restarted := New(m.Store, m.Locks, filepath.Dir(m.root))
	must(t, restarted.Initialize(ctx))
	s = switchTo(t, restarted, p, "", "restore")
	content(t, p, ".outline.json", "manual latest")
	assertContextText(t, m, "t1", "future compacted summary", true)
	content(t, p, "attachments/new.png", "bytes")
	content(t, p, ".git/HEAD", "real git")
	content(t, p, "versions/legacy.html", "untouched legacy")
	if _, err := m.Store.GetSlide(ctx, "slide"); err == nil {
		t.Fatal("latest snapshot retained deleted slide")
	}
	if _, err := m.Store.GetIdempotency(ctx, "artifact_commit", "cp2", "delete"); err != nil {
		t.Fatal("latest receipt missing", err)
	}
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
	if _, err := m.Switch(sessionstate.WithRequestScene(ctx, s.SceneRevision), p.ID, "", s.Revision-1, "stale", nil); !errors.Is(err, ErrConflict) {
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
func TestDurableSwitchRecoveryAndIntegrity(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	put(t, p, "file", "before")
	cp(t, m, p, "cp1", model.RunDone)
	put(t, p, "file", "live")
	s, _ := m.State(p.ID)
	before, err := m.Capture(ctx, p, s)
	must(t, err)
	must(t, m.Store.BeginHistoryOperation(ctx, p.ID, sessionstate.HistoryOperation{ID: "crashed", Kind: "switch", RequestHash: "crashed", SceneRevision: s.SceneRevision, HistoryRevision: s.Revision, Before: before, Next: s}))
	put(t, p, "file", "half switched")
	must(t, m.Store.DeleteThread(ctx, "t1"))
	// An undo window keeps the live thread rows out of history replacement.
	must(t, m.Initialize(ctx))
	content(t, p, "file", "half switched")
	deletion, err := m.Store.GetDeletion(ctx, "thread", "t1")
	must(t, err)
	must(t, m.Store.RequestDeletionRestore(ctx, deletion))
	must(t, m.Store.FinishDeletionRestore(ctx, deletion))
	must(t, m.Initialize(ctx))
	content(t, p, "file", "live")
	if _, err = m.Store.GetRun(ctx, "cp1"); err != nil {
		t.Fatal(err)
	}
	cpref := s.Checkpoints[0].Snapshot
	must(t, os.WriteFile(filepath.Join(m.dir(p.ID), cpref+".json"), []byte(`{}`), 0600))
	s, err = m.State(p.ID)
	must(t, err)
	if _, err = m.Switch(sessionstate.WithRequestScene(ctx, s.SceneRevision), p.ID, "cp1", s.Revision, "corrupt", nil); err == nil {
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
	if _, err := m.Switch(sessionstate.WithRequestScene(ctx, s.SceneRevision), p.ID, "cp1", s.Revision, "busy", nil); !errors.Is(err, ErrBusy) {
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

func (s *failingRestore) FinishHistoryRestore(ctx context.Context, id, operationID string, raw json.RawMessage, preimage State, success bool) (State, error) {
	if s.fail {
		s.fail = false
		return State{}, errors.New("injected disk/database failure")
	}
	return s.SnapshotStore.FinishHistoryRestore(ctx, id, operationID, raw, preimage, success)
}
func TestDatabaseFailureRestoresFilesAndLatestEligibility(t *testing.T) {
	m, p := fixture(t)
	put(t, p, "file", "before")
	cp(t, m, p, "cp1", model.RunDone)
	put(t, p, "file", "latest")
	s, _ := m.State(p.ID)
	m.Store = &failingRestore{m.Store, true}
	if _, err := m.Switch(sessionstate.WithRequestScene(context.Background(), s.SceneRevision), p.ID, "cp1", s.Revision, "fail", nil); err == nil {
		t.Fatal("injected failure ignored")
	}
	content(t, p, "file", "latest")
	if _, err := m.Store.GetRun(context.Background(), "cp1"); err != nil {
		t.Fatal(err)
	}
	after, _ := m.State(p.ID)
	if after.Revision <= s.Revision || after.SceneRevision <= s.SceneRevision || after.Latest != "" {
		t.Fatal("failed operation published history")
	}
}
func TestPreviewBusyExportAndCheckpointOrder(t *testing.T) {
	m, p := fixture(t)
	cp(t, m, p, "cp1", model.RunDone)
	cp(t, m, p, "cp2", model.RunDone)
	s, _ := m.State(p.ID)
	if len(s.Checkpoints) != 2 || s.Checkpoints[0].RunID != "cp1" || s.Checkpoints[1].RunID != "cp2" {
		t.Fatal("checkpoint order changed")
	}
	ctx := context.Background()
	accepted, _, err := m.Store.AcceptCommand(ctx, "t1", model.CommandRequest{RequestKey: "automatic-name", Kind: "rename", Source: "automatic", Input: json.RawMessage(`{}`)}, s.SceneRevision)
	must(t, err)
	naming := accepted.Command
	naming.Status = "running"
	must(t, m.Store.SaveCommandExecution(ctx, naming))
	if _, err := m.Baseline(ctx, p, "cp3", "t1", model.CreateRunParams{}); err != nil {
		t.Fatalf("background naming blocked the next run baseline: %v", err)
	}
	m.ExportActive = func(string) bool { return true }
	if _, err := m.Preview(context.Background(), p.ID, "cp1"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
}

func TestAcceptedDeletionCannotBeReversedByInterruptedHistory(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	put(t, p, "file", "before")
	cp(t, m, p, "cp1", model.RunDone)
	put(t, p, "file", "latest")
	s := switchTo(t, m, p, "cp1", "back")
	_, err := m.BeginMutation(ctx, p.ID, s.Revision)
	must(t, err)
	must(t, m.Store.DeleteProject(ctx, p.ID))
	must(t, os.RemoveAll(filepath.Join(model.ProjectRoot(p.WorkDir), "artifacts")))
	must(t, os.RemoveAll(filepath.Join(model.ProjectRoot(p.WorkDir), "threads")))
	// No finish callback: recreate the manager as at process startup.
	restarted := New(m.Store, m.Locks, filepath.Dir(m.root))
	must(t, restarted.Initialize(ctx))
	if _, err := m.Store.GetProject(ctx, p.ID); !errors.Is(err, run.ErrRunNotFound) {
		t.Fatalf("history revived deleted project: %v", err)
	}
	if _, err := os.Stat(p.WorkDir); !os.IsNotExist(err) {
		t.Fatalf("history recreated deleted files: %v", err)
	}
}

func TestCommittedMutationSurvivesRestartAndCannotReopenFuture(t *testing.T) {
	m, p := fixture(t)
	ctx := context.Background()
	cp(t, m, p, "cp1", model.RunDone)
	s := switchTo(t, m, p, "cp1", "back")
	finish, err := m.BeginMutation(ctx, p.ID, s.Revision)
	must(t, err)
	pending, err := m.Store.ReadProjectHistory(ctx, p.ID)
	must(t, err)
	if pending.Pending == nil {
		t.Fatal("mutation has no durable preimage")
	}
	put(t, p, "file", "accepted branch")
	must(t, finish(true))
	accepted, _ := m.State(p.ID)
	must(t, finish(false)) // A later handler failure cannot undo accepted work.
	if pending, err := m.Store.ReadProjectHistory(ctx, p.ID); err != nil || pending.Pending != nil {
		t.Fatalf("accepted operation remained recoverable: %+v %v", pending, err)
	}
	restarted := New(m.Store, m.Locks, filepath.Dir(m.root))
	must(t, restarted.Initialize(ctx))
	content(t, p, "file", "accepted branch")
	after, _ := m.State(p.ID)
	if after.Revision != accepted.Revision || after.Latest != "" {
		t.Fatal("accepted branch resurrected its future")
	}
}

func TestCurrentContextRestoresOnlyActiveBoundary(t *testing.T) {
	m, p := fixture(t)
	replaceContext(t, m, p, "t1", "before compaction")
	cp(t, m, p, "cp1", model.RunDone)
	replaceContext(t, m, p, "t1", "future compacted summary")
	switchTo(t, m, p, "cp1", "back")
	assertContextText(t, m, "t1", "before compaction", true)
	assertContextText(t, m, "t1", "future compacted summary", false)
	switchTo(t, m, p, "", "restore")
	assertContextText(t, m, "t1", "future compacted summary", true)
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

func TestRestorePreviewUsesSavedActiveThreadDraft(t *testing.T) {
	for _, draft := range []string{"首次回退前未发送的草稿", ""} {
		name := "with_draft"
		if draft == "" {
			name = "empty_draft"
		}
		t.Run(name, func(t *testing.T) {
			m, p := fixture(t)
			ctx := context.Background()
			cp(t, m, p, "cp1", model.RunDone)
			cp(t, m, p, "cp2", model.RunDone)
			scene, err := json.Marshal(map[string]any{
				"active_thread_id": "t1",
				"composer":         map[string]any{"threadDrafts": map[string]string{"t1": draft, "another": "不要展示其他会话的草稿"}},
			})
			must(t, err)
			state, err := m.State(p.ID)
			must(t, err)
			_, err = m.Switch(sessionstate.WithRequestScene(ctx, state.SceneRevision), p.ID, "cp2", state.Revision, "first", scene)
			must(t, err)
			preview, err := m.Preview(ctx, p.ID, "")
			must(t, err)
			if preview.Input != draft {
				t.Fatalf("restore preview=%q want=%q", preview.Input, draft)
			}
			// A second rollback saves another current scene, but must retain the
			// original latest scene for both preview and actual restoration.
			state = switchTo(t, m, p, "cp1", "second")
			preview, err = m.Preview(ctx, p.ID, "")
			must(t, err)
			if preview.Input != draft {
				t.Fatalf("continuous rollback changed draft: %q", preview.Input)
			}
			restored, err := m.Switch(sessionstate.WithRequestScene(ctx, state.SceneRevision), p.ID, "", state.Revision, "latest", json.RawMessage(`{"active_thread_id":"t1","composer":{"threadDrafts":{"t1":"current draft"}}}`))
			must(t, err)
			if string(restored.Scene) != string(scene) {
				t.Fatalf("restored scene differs from preview source: %s", restored.Scene)
			}
		})
	}
}

func TestAuthoringHiddenFilesAndFlatHTMLRestoreTogether(t *testing.T) {
	m, p := fixture(t)
	files := map[string]string{
		".meta.json":        `{"title":"Before"}`,
		".constraints.json": `{"content":{"requirements":[],"prohibitions":[]},"visual":{"requirements":["Before"],"prohibitions":[]}}`,
		".outline.json":     `{"sections":[]}`,
		".sli_a.json":       `{"purpose":"content_explanation","layout":"Title above content","core":"Before","elements":[]}`,
		"sli_a.html":        `<html>Before</html>`,
	}
	for path, raw := range files {
		put(t, p, path, raw)
	}
	cp(t, m, p, "authoring", model.RunDone)
	must(t, os.Remove(filepath.Join(p.WorkDir, ".sli_a.json")))
	must(t, os.Remove(filepath.Join(p.WorkDir, "sli_a.html")))
	put(t, p, ".meta.json", `{"title":"After"}`)
	switchTo(t, m, p, "authoring", "restore-authoring")
	for path, raw := range files {
		content(t, p, path, raw)
	}
	switchTo(t, m, p, "", "restore-latest")
	if _, err := os.Stat(filepath.Join(p.WorkDir, ".sli_a.json")); !os.IsNotExist(err) {
		t.Fatalf("deleted Spec restored into latest: %v", err)
	}
	content(t, p, ".meta.json", `{"title":"After"}`)
	if _, err := os.Stat(filepath.Join(p.WorkDir, "sli_a.html")); !os.IsNotExist(err) {
		t.Fatalf("deleted HTML restored into latest: %v", err)
	}
}

func replaceContext(t *testing.T, m *Manager, p model.Project, thread, text string) {
	t.Helper()
	ctx := context.Background()
	session, err := contextengine.NewContextStore(m.Store).Open(ctx, thread, nil)
	must(t, err)
	prepared, err := session.Prepare([]llm.Message{{Role: llm.RoleUser, Content: llm.TextContent(text)}}, "", "replace")
	must(t, err)
	compaction := model.ContextCompaction{ID: model.MustShortID("cmp"), ProjectID: p.ID, ThreadID: thread, Trigger: model.ContextCompactionManual, Content: text, CreatedAt: 1}
	accepted, _, err := m.Store.AcceptCommand(ctx, thread, model.CommandRequest{RequestKey: compaction.ID, Kind: "compact", Input: json.RawMessage(`{}`)}, session.Snapshot.SceneRevision)
	must(t, err)
	command := accepted.Command
	command.Status = "completed"
	command.Result, err = json.Marshal(compaction)
	must(t, err)
	_, err = m.Store.CompleteCommand(ctx, command, sessionstate.CommandEffects{Context: &prepared.Update, Compaction: &compaction})
	must(t, err)
}

func assertContextText(t *testing.T, m *Manager, thread, text string, want bool) {
	t.Helper()
	snapshot, err := m.Store.LoadContext(context.Background(), thread)
	must(t, err)
	found := false
	for _, message := range snapshot.Messages {
		if strings.Contains(message.Native.Text(), text) {
			found = true
		}
	}
	if found != want {
		t.Fatalf("current context contains %q: %v", text, found)
	}
}
