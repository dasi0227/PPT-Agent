package sqlite

import (
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

// All sections and their continuation cursor come from the same short snapshot.
// This path never assembles a model request, reads project files, or replays logs.
func (s *Store) SessionView(ctx context.Context, threadID string, groupLimit int) (sessionstate.View, error) {
	view := sessionstate.View{ActiveCommands: []json.RawMessage{}, PendingInteractions: []json.RawMessage{}}
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		view.ProjectID, view.ThreadID, view.SceneRevision, view.Seq = head.ProjectID, head.ID, head.SceneRevision, head.LastSeq
		view.Cursor = (sessionstate.Cursor{ProjectID: head.ProjectID, ThreadID: head.ID, SceneRevision: head.SceneRevision, Seq: head.LastSeq}).Encode()
		view.Thread, err = readThreadView(tx, threadID)
		if err != nil {
			return err
		}
		if view.Thread.ActiveRunID != nil {
			var row runPO
			if err := tx.Where("id = ? AND thread_id = ?", *view.Thread.ActiveRunID, threadID).Take(&row).Error; err != nil {
				return err
			}
			current := row.view()
			view.ActiveRun = &current
			view.ContextWindow = currentContextWindow(current.ContextWindow, head.ContextRevision, view.Thread.PlanRevision)
		} else {
			var latest struct{ ContextWindowJSON *string }
			if err := tx.Table("runs").Select("context_window_json").Where("id = (SELECT last_context_run_id FROM threads WHERE id = ?)", threadID).Scan(&latest).Error; err != nil {
				return err
			}
			view.ContextWindow = currentContextWindow(rawOrNull(latest.ContextWindowJSON), head.ContextRevision, view.Thread.PlanRevision)
		}
		var commands []struct{ SummaryJSON *string }
		if err := tx.Raw(`SELECT i.summary_json FROM command_executions c
 LEFT JOIN timeline_items i ON i.id = 'command:' || c.command_id AND i.thread_id = c.thread_id
 WHERE c.thread_id = ? AND c.kind NOT IN ('rename','suggest') AND c.status IN ('accepted','running','cancel_requested') ORDER BY c.created_at,c.id`, threadID).Scan(&commands).Error; err != nil {
			return err
		}
		for _, row := range commands {
			if row.SummaryJSON == nil {
				return sessionstate.ErrInvalidState
			}
			view.ActiveCommands = append(view.ActiveCommands, json.RawMessage(*row.SummaryJSON))
		}
		// These commands have no visible timeline item. Their recovery summary
		// needs only execution metadata, never the original prompt or result body.
		var metadataCommands []commandExecutionPO
		if err := tx.Select("id,command_id,attempt_no,thread_id,project_id,kind,source,status,phase,scene_revision,entity_revision,updated_seq,created_at,updated_at").
			Where("thread_id = ? AND kind IN ('rename','suggest') AND status IN ('accepted','running','cancel_requested')", threadID).
			Order("created_at,id").Find(&metadataCommands).Error; err != nil {
			return err
		}
		for _, row := range metadataCommands {
			row.RequestJSON = "{}"
			command, err := row.toModel()
			if err != nil {
				return err
			}
			raw, err := json.Marshal(command)
			if err != nil {
				return err
			}
			view.ActiveCommands = append(view.ActiveCommands, raw)
		}
		var interactions []struct{ SummaryJSON *string }
		if err := tx.Raw(`SELECT i.summary_json FROM run_interactions r
 LEFT JOIN timeline_items i ON i.id = 'interaction:' || r.id AND i.thread_id = r.thread_id
 WHERE r.thread_id = ? AND r.state IN ('pending','answered') ORDER BY r.created_at,r.id`, threadID).Scan(&interactions).Error; err != nil {
			return err
		}
		for _, row := range interactions {
			if row.SummaryJSON == nil {
				return sessionstate.ErrInvalidState
			}
			view.PendingInteractions = append(view.PendingInteractions, json.RawMessage(*row.SummaryJSON))
		}
		view.Timeline, err = readTimeline(tx, head, nil, pageLimit(groupLimit, 20, 50))
		return err
	})
	return view, err
}

// A retained estimate describes a specific context, not whichever context is
// current after a later append or compaction.
func currentContextWindow(raw json.RawMessage, revision, planRevision int64) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var value struct {
		ContextRevision int64 `json:"context_revision,string"`
		PlanRevision    int64 `json:"plan_revision,string"`
	}
	if json.Unmarshal(raw, &value) != nil || value.ContextRevision != revision || value.PlanRevision != planRevision {
		return nil
	}
	return raw
}
