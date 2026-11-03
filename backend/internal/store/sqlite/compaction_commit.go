package sqlite

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

func (w *sessionWrite) completeCompaction(execution model.CommandExecution, update sessionstate.ContextUpdate, compaction model.ContextCompaction) error {
	if compaction.Trigger != model.ContextCompactionManual {
		return sessionstate.ErrInvalidState
	}
	var active int64
	if err := w.tx.Table("runs").Where("project_id = ? AND status NOT IN ?", execution.ProjectID, []string{"done", "failed", "canceled"}).Count(&active).Error; err != nil {
		return err
	}
	if active > 0 {
		return store.ErrRunActive
	}
	if err := w.replaceContext(update); err != nil {
		return err
	}
	// A resumable completed task must not resurrect the pre-compaction window.
	if err := w.tx.Exec(`UPDATE runs SET context_revision = ?, checkpoint_revision = checkpoint_revision + 1,
 checkpoint_json = json_set(checkpoint_json, '$.context_revision', ?, '$.checkpoint_revision', checkpoint_revision + 1)
 WHERE thread_id = ? AND context_revision = ? AND status IN ('done','failed','canceled')
 AND json_extract(checkpoint_json, '$.continuation_allowed') = 1`, w.head.ContextRevision, w.head.ContextRevision, execution.ThreadID, update.ExpectedRevision).Error; err != nil {
		return err
	}
	return w.recordCompaction(compaction, update.ExpectedRevision)
}

func (w *sessionWrite) recordCompaction(compaction model.ContextCompaction, beforeRevision int64) error {
	if compaction.ID == "" || compaction.ThreadID != w.head.ID || compaction.ProjectID != w.head.ProjectID ||
		beforeRevision < 0 || w.head.ContextRevision <= beforeRevision || compaction.CreatedAt <= 0 {
		return sessionstate.ErrInvalidState
	}
	raw, err := json.Marshal(compaction)
	if err != nil {
		return err
	}
	return w.tx.Table("context_compactions").Create(map[string]any{
		"id": compaction.ID, "thread_id": w.head.ID, "run_id": nullableID(compaction.RunID),
		"before_revision": beforeRevision, "after_revision": w.head.ContextRevision,
		"result_json": string(raw), "created_at": compaction.CreatedAt,
	}).Error
}
