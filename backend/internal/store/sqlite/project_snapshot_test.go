package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

// Exercise the controller's two-phase contract, not an unfenced row replacement.
func restoreProjectSnapshot(ctx context.Context, s *Store, projectID string, raw json.RawMessage) error {
	control, err := s.ReadProjectHistory(ctx, projectID)
	if err != nil {
		return err
	}
	operation := sessionstate.HistoryOperation{ID: model.MustShortID("restore"), Kind: "switch", RequestHash: "snapshot-test", Before: strings.Repeat("a", 64), SceneRevision: control.State.SceneRevision, HistoryRevision: control.State.Revision, Next: control.State}
	if err := s.BeginHistoryOperation(ctx, projectID, operation); err != nil {
		return err
	}
	_, err = s.FinishHistoryRestore(ctx, projectID, operation.ID, raw, sessionstate.HistoryState{}, true)
	return err
}

func TestSnapshotRestoresProjectStateAndInvalidatesOldWorker(t *testing.T) {
	s, cp := checkpointFixture(t)
	ctx := context.Background()
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProject(ctx, model.Project{ID: "other", Title: "other", Theme: "theme", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Exec(`INSERT INTO resources VALUES('snippet','global','global','global','library',0,1,1)`).Error; err != nil {
		t.Fatal(err)
	}
	accepted, _, err := s.AcceptCommand(ctx, "t", model.CommandRequest{RequestKey: "background-rename", Kind: "rename", Source: "automatic", Input: json.RawMessage(`{}`)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	naming := accepted.Command
	naming.Status = "running"
	if err := s.SaveCommandExecution(ctx, naming); err != nil {
		t.Fatal(err)
	}
	raw, err := s.CaptureProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodeProjectSnapshot("p", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != len(projectTables) {
		t.Fatalf("project inventory: %v", snapshot)
	}
	for _, excluded := range []string{"project_history_state", "thread_changes", "resources"} {
		if _, exists := snapshot[excluded]; exists {
			t.Fatalf("control/global table %s entered snapshot", excluded)
		}
	}
	// No live task may start a history switch; the target can still contain a
	// saved running task, whose restored execution must be paused and re-fenced.
	if err := s.SetRunStatus(ctx, "r", model.RunDone); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Exec(`UPDATE projects SET title='changed' WHERE id='p'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := restoreProjectSnapshot(ctx, s, "p", raw); err != nil {
		t.Fatal(err)
	}
	project, err := s.GetProject(ctx, "p")
	if err != nil || project.Title != "project" {
		t.Fatalf("project=%+v err=%v", project, err)
	}
	current, err := s.GetRun(ctx, "r")
	if err != nil || current.Status != model.RunPaused || current.ExecutionRevision != 2 || current.SceneRevision != 2 {
		t.Fatalf("run=%+v err=%v", current, err)
	}
	control, err := s.ReadProjectHistory(ctx, "p")
	if err != nil || control.Restoring || control.Pending != nil || control.State.SceneRevision != 2 {
		t.Fatalf("unsettled history: %+v %v", control, err)
	}
	view, err := s.SessionView(ctx, "t", 20)
	if err != nil || view.SceneRevision != 2 || view.Thread.ActiveRunID == nil || *view.Thread.ActiveRunID != "r" {
		t.Fatalf("restored view: %+v %v", view, err)
	}
	restoredNaming, err := s.GetCommand(ctx, naming.CommandID)
	if err != nil || restoredNaming.Status != "interrupted" || restoredNaming.ExecutionRevision != naming.ExecutionRevision+1 {
		t.Fatalf("restored command authority: %+v %v", restoredNaming, err)
	}
	naming.Status = "completed"
	naming.Result = json.RawMessage(`{"title":"late name"}`)
	if _, err := s.CompleteCommand(ctx, naming, sessionstate.CommandEffects{Rename: &sessionstate.RenameEffect{Title: "late name"}}); !errors.Is(err, sessionstate.ErrSceneChanged) {
		t.Fatalf("late command survived switch: %v", err)
	}
	cp.CheckpointRevision = 1
	if err := s.SaveCheckpoint(ctx, cp); !errors.Is(err, sessionstate.ErrSceneChanged) {
		t.Fatalf("old checkpoint accepted: %v", err)
	}
	if _, err := s.GetProject(ctx, "other"); err != nil {
		t.Fatal("other project removed")
	}
	var count int64
	if err := s.db.Table("resources").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("global resources changed: %d %v", count, err)
	}
	if err := s.SetRunStatus(ctx, "r", model.RunDone); err != nil {
		t.Fatal(err)
	}
	snapshot["runs"][0]["status"] = "invalid"
	broken, _ := json.Marshal(snapshot)
	if err := restoreProjectSnapshot(ctx, s, "p", broken); err == nil {
		t.Fatal("invalid snapshot accepted")
	}
	// Failure leaves the controller isolated with its preimage; business rows
	// and ownership changes from the failed transaction must not leak.
	var row runPO
	if err := s.db.Where("id = ?", "r").Take(&row).Error; err != nil || row.ExecutionRevision != 2 || row.Status != string(model.RunDone) {
		t.Fatalf("failed restore changed rows: %+v %v", row, err)
	}
	control, err = s.ReadProjectHistory(ctx, "p")
	if err != nil || !control.Restoring || control.Pending == nil || control.State.SceneRevision != 2 {
		t.Fatalf("failed restore lost isolation: %+v %v", control, err)
	}
}
