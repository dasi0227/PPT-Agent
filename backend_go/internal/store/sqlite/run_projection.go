package sqlite

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

func rawOrNull(value *string) json.RawMessage {
	if value == nil {
		return nil
	}
	return json.RawMessage(*value)
}

func (row runPO) view() sessionstate.RunView {
	return sessionstate.RunView{
		ID: row.ID, ThreadID: row.ThreadID, ProjectID: row.ProjectID, SceneRevision: row.SceneRevision,
		EntityRevision: row.EntityRevision, UpdatedSeq: row.UpdatedSeq, Status: row.Status,
		Scope: row.Command.Scope, Mode: row.Command.Mode, Model: stringPointer(row.ModelProfileName),
		Skills: row.Command.PublicSkills(), Components: row.Command.PublicComponents(), DroppedMentionedSlideIDs: row.Command.DroppedMentionedSlideIDs,
		CanContinue: row.continuable(),
		PauseReason: row.PauseReason, PausedAt: valueOrZero(row.PausedAt), CancelRequestedAt: valueOrZero(row.CancelRequestedAt),
		ActiveDurationMS: row.ActiveDurationMS, ActiveSince: row.ActiveSince, Progress: rawOrNull(row.ProgressJSON), Usage: rawOrNull(row.UsageJSON), Terminal: rawOrNull(row.TerminalJSON),
		ContextWindow: rawOrNull(row.ContextWindowJSON),
		TerminalRef:   textOrEmpty(row.TerminalRef),
		CreatedAt:     row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func (w *sessionWrite) publishRun(runID string) error {
	if err := w.tx.Table("runs").Where("id = ? AND thread_id = ?", runID, w.head.ID).Updates(map[string]any{"entity_revision": gorm.Expr("entity_revision + 1"), "updated_seq": w.seq, "updated_at": w.now}).Error; err != nil {
		return err
	}
	var row runPO
	if err := w.tx.Where("id = ? AND thread_id = ?", runID, w.head.ID).Take(&row).Error; err != nil {
		return err
	}
	summary, err := json.Marshal(map[string]any{"usage": rawOrNull(row.UsageJSON), "active_duration_ms": row.ActiveDurationMS, "progress": rawOrNull(row.ProgressJSON), "terminal": rawOrNull(row.TerminalJSON), "terminal_ref": row.TerminalRef})
	if err != nil {
		return err
	}
	if err := w.putGroup(sessionstate.TimelineGroup{ID: "run:" + runID, ThreadID: w.head.ID, Kind: "run", RunID: runID, InputID: row.InputID, Status: row.Status, Summary: summary}); err != nil {
		return err
	}
	query := w.tx.Table("threads").Where("id = ?", w.head.ID)
	var active any = runID
	if model.RunStatus(row.Status).Terminal() {
		query = query.Where("active_run_id = ?", runID)
		active = nil
	}
	if err := query.Update("active_run_id", active).Error; err != nil {
		return err
	}
	if err := w.upsert("run", row.ID, row.view()); err != nil {
		return err
	}
	return w.publishThread()
}
