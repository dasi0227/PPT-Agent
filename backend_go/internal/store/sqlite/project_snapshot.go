package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

// Parents precede children. Global libraries/settings, live history control and
// delivery batches are intentionally outside a project business snapshot.
var projectTables = []string{
	"projects", "slides", "threads", "runs", "conversation_rounds", "conversation_messages",
	"thread_context_versions", "thread_inputs", "model_request_snapshots", "tool_executions",
	"run_interactions", "steering_inbox", "command_executions", "briefing_versions", "context_compactions",
	"timeline_groups", "timeline_details", "timeline_items", "idempotency_records",
}

func projectPredicate(table string) string {
	switch table {
	case "projects":
		return "id = @project"
	case "slides", "threads", "runs", "command_executions", "briefing_versions":
		return "project_id = @project"
	case "idempotency_records":
		return "scope NOT IN ('delete','project_history') AND (owner_id = @project OR owner_id IN (SELECT id FROM threads WHERE project_id = @project) OR owner_id IN (SELECT id FROM runs WHERE project_id = @project))"
	default:
		return "thread_id IN (SELECT id FROM threads WHERE project_id = @project)"
	}
}
func snapshotArgs(id string) map[string]any { return map[string]any{"project": id} }

func (s *Store) CaptureProject(ctx context.Context, id string) (json.RawMessage, error) {
	data := map[string][]map[string]any{}
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		control, err := readProjectHistory(tx, id)
		if err != nil {
			return err
		}
		if control.Restoring {
			return sessionstate.ErrProjectRestoring
		}
		for _, table := range projectTables {
			rows := []map[string]any{}
			if err := tx.Raw("SELECT * FROM "+table+" WHERE "+projectPredicate(table), snapshotArgs(id)).Scan(&rows).Error; err != nil {
				return err
			}
			data[table] = rows
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(data)
}

func decodeProjectSnapshot(id string, raw json.RawMessage) (map[string][]map[string]any, error) {
	var data map[string][]map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	if len(data) != len(projectTables) || len(data["projects"]) != 1 || data["projects"][0]["id"] != id {
		return nil, fmt.Errorf("invalid project snapshot inventory")
	}
	threads := map[string]bool{}
	for _, row := range data["threads"] {
		threads[fmt.Sprint(row["id"])] = true
	}
	for _, table := range projectTables {
		rows, present := data[table]
		if !present || rows == nil {
			return nil, fmt.Errorf("invalid snapshot table %s", table)
		}
		for _, row := range rows {
			if project, exists := row["project_id"]; exists && project != id {
				return nil, sessionstate.ErrInvalidState
			}
			if thread, exists := row["thread_id"]; exists && !threads[fmt.Sprint(thread)] {
				return nil, sessionstate.ErrInvalidState
			}
		}
	}
	return data, nil
}

// Restore rows, advance the live scene, publish its reset watermarks and record
// the switch outcome in one transaction. No restored project row may cascade
// away project_history_state or the receipt that fences this operation.
func (s *Store) FinishHistoryRestore(ctx context.Context, id, operationID string, raw json.RawMessage, preimageState sessionstate.HistoryState, success bool) (sessionstate.HistoryState, error) {
	data, err := decodeProjectSnapshot(id, raw)
	if err != nil {
		return sessionstate.HistoryState{}, err
	}
	var next sessionstate.HistoryState
	var notify []string
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := readProjectHistory(tx, id)
		if err != nil {
			return err
		}
		if !current.Restoring || current.Pending == nil || current.Pending.ID != operationID {
			return sessionstate.ErrVersionConflict
		}
		if success && current.Pending.Kind != "switch" {
			return sessionstate.ErrInvalidState
		}
		if current.State.SceneRevision == math.MaxInt64 || current.State.Revision == math.MaxInt64 {
			return sessionstate.ErrVersionConflict
		}
		next = preimageState
		if success {
			next = current.Pending.Next
		}
		next.SceneRevision, next.Revision = current.State.SceneRevision+1, current.State.Revision+1
		if err := filterDeletedSnapshot(tx, id, data); err != nil {
			return err
		}
		keptRuns := make(map[string]string, len(data["runs"]))
		for _, row := range data["runs"] {
			keptRuns[fmt.Sprint(row["id"])] = fmt.Sprint(row["thread_id"])
		}
		checkpoints := make([]sessionstate.ProjectCheckpoint, 0, len(next.Checkpoints))
		for _, checkpoint := range next.Checkpoints {
			if keptRuns[checkpoint.RunID] == checkpoint.ThreadID {
				checkpoints = append(checkpoints, checkpoint)
			}
		}
		next.Checkpoints = checkpoints
		notify, err = projectThreadIDs(tx, id)
		if err != nil {
			return err
		}
		if err := restoreProjectRows(tx, id, next.SceneRevision, data); err != nil {
			return err
		}
		if err := reprojectRestoredThreads(tx, id, next.SceneRevision); err != nil {
			return err
		}
		restored, err := projectThreadIDs(tx, id)
		if err != nil {
			return err
		}
		notify = append(notify, restored...)
		updates, err := historyUpdates(next)
		if err != nil {
			return err
		}
		updates["switch_state"], updates["switch_operation_id"], updates["pending_operation_json"] = "ready", nil, nil
		updates["last_operation_id"], updates["last_command"] = operationID, current.Pending.Kind
		if err := tx.Table("project_history_state").Where("project_id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		return writeHistoryReceipt(tx, id, *current.Pending, next, success)
	})
	if err == nil {
		s.notifySessions(notify)
	}
	return next, err
}

func snapshotInteger(row map[string]any, key string) (int64, error) {
	value, err := strconv.ParseInt(fmt.Sprint(row[key]), 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid snapshot %s", key)
	}
	return value, nil
}

func restoreProjectRows(tx *gorm.DB, id string, scene int64, data map[string][]map[string]any) error {
	epochs := map[string]int64{}
	for _, table := range []string{"runs", "command_executions"} {
		var live []struct {
			ID                string
			ExecutionRevision int64
		}
		if err := tx.Table(table).Select("id,execution_revision").Where("project_id = ?", id).Find(&live).Error; err != nil {
			return err
		}
		for _, row := range live {
			epochs[table+":"+row.ID] = row.ExecutionRevision
		}
		for _, row := range data[table] {
			previous, err := snapshotInteger(row, "execution_revision")
			if err != nil {
				return err
			}
			revision := max(previous, epochs[table+":"+fmt.Sprint(row["id"])])
			if revision == math.MaxInt64 {
				return sessionstate.ErrVersionConflict
			}
			row["execution_revision"], row["owner_instance_id"], row["scene_revision"] = revision+1, "", scene
			epochs[table+":"+fmt.Sprint(row["id"])] = revision + 1
			if table == "runs" {
				row["active_since"] = nil
				checkpointExecution, err := snapshotInteger(row, "checkpoint_execution_revision")
				if err != nil {
					return err
				}
				if checkpointExecution == previous && row["checkpoint_json"] != nil {
					row["checkpoint_execution_revision"] = revision + 1
				} else {
					row["continuation_allowed"] = 0
				}
				switch row["status"] {
				case "pending", "running", "waiting", "recovering":
					row["status"], row["pause_reason"] = "paused", "project_restored"
				}
				row["context_window_json"] = nil
				if raw, ok := row["checkpoint_json"].(string); ok {
					var checkpoint map[string]json.RawMessage
					if err := json.Unmarshal([]byte(raw), &checkpoint); err != nil {
						return err
					}
					checkpoint["scene_revision"], _ = json.Marshal(scene)
					checkpoint["execution_revision"], _ = json.Marshal(revision + 1)
					checkpoint["owner_instance_id"] = json.RawMessage(`""`)
					if raw := checkpoint["pending_model_call"]; len(raw) > 0 && string(raw) != "null" {
						var pending sessionstate.PendingModelCall
						if err := json.Unmarshal(raw, &pending); err != nil {
							return err
						}
						if pending.Stage == "dispatched" {
							pending.Stage = "outcome_unknown"
							checkpoint["pending_model_call"], _ = json.Marshal(pending)
						}
					}
					encoded, err := json.Marshal(checkpoint)
					if err != nil {
						return err
					}
					row["checkpoint_json"] = string(encoded)
				}
			} else if activeCommandStatus(fmt.Sprint(row["status"])) {
				// The snapshot may include automatic naming, never an active
				// user command or dispatched Git operation.
				if row["kind"] != "rename" || row["source"] != "automatic" {
					return sessionstate.ErrInvalidState
				}
				row["status"], row["error_json"] = "interrupted", `{"code":"COMMAND_INTERRUPTED","retryable":true}`
			}
		}
	}
	var liveThreads []threadPO
	if err := tx.Where("project_id = ?", id).Find(&liveThreads).Error; err != nil {
		return err
	}
	for _, row := range data["threads"] {
		for _, field := range []string{"naming_revision", "rename_operation_version", "suggest_version"} {
			version, err := snapshotInteger(row, field)
			if err != nil {
				return err
			}
			for _, live := range liveThreads {
				if live.ID != row["id"] {
					continue
				}
				value := live.NamingRevision
				if field == "rename_operation_version" {
					value = live.RenameOperationVersion
				}
				if field == "suggest_version" {
					value = live.SuggestVersion
				}
				version = max(version, value)
			}
			if version == math.MaxInt64 {
				return sessionstate.ErrVersionConflict
			}
			row[field] = version + 1
		}
		row["retained_after_seq"] = row["last_seq"]
	}
	for _, row := range data["tool_executions"] {
		row["scene_revision"] = scene
		row["execution_revision"] = epochs["runs:"+fmt.Sprint(row["run_id"])]
		if row["status"] == "executing" {
			row["status"] = "outcome_unknown"
		}
	}
	// Content references are scoped to a scene. Their immutable source rows are
	// restored with the items; only the scene token changes, never user text.
	for _, table := range []string{"timeline_items", "timeline_details", "runs", "run_interactions"} {
		for _, row := range data[table] {
			for _, column := range []string{"body_ref", "detail_ref", "terminal_ref"} {
				if raw, ok := row[column].(string); ok && raw != "" {
					ref, err := sessionstate.DecodeContentRef(raw)
					if err != nil {
						return err
					}
					ref.SceneRevision = scene
					row[column] = ref.Encode()
				}
			}
			for _, column := range []string{"summary_json", "public_json", "request_json", "terminal_json"} {
				if raw, ok := row[column].(string); ok {
					updated, err := rebasePublicReferences(raw, scene)
					if err != nil {
						return err
					}
					row[column] = updated
				}
			}
		}
	}
	if err := tx.Exec("PRAGMA defer_foreign_keys = ON").Error; err != nil {
		return err
	}
	owners := []string{id}
	for _, table := range []string{"threads", "runs"} {
		for _, row := range data[table] {
			owners = append(owners, fmt.Sprint(row["id"]))
		}
	}
	if err := tx.Exec("DELETE FROM idempotency_records WHERE scope NOT IN ('delete','project_history') AND owner_id IN ?", owners).Error; err != nil {
		return err
	}
	if err := tx.Exec("DELETE FROM thread_changes WHERE thread_id IN (SELECT id FROM threads WHERE project_id = ?)", id).Error; err != nil {
		return err
	}
	for i := len(projectTables) - 1; i > 0; i-- {
		table := projectTables[i]
		if err := tx.Exec("DELETE FROM "+table+" WHERE "+projectPredicate(table), snapshotArgs(id)).Error; err != nil {
			return err
		}
	}
	project := data["projects"][0]
	updates := map[string]any{}
	for key, value := range project {
		if key != "id" {
			updates[key] = snapshotSQLValue(value)
		}
	}
	if err := tx.Table("projects").Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	for _, table := range projectTables[1:] {
		for _, row := range data[table] {
			cols, marks, args := []string{}, []string{}, []any{}
			for col, value := range row {
				cols = append(cols, `"`+strings.ReplaceAll(col, `"`, `""`)+`"`)
				marks = append(marks, "?")
				args = append(args, snapshotSQLValue(value))
			}
			if err := tx.Exec("INSERT INTO "+table+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(marks, ",")+")", args...).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
func snapshotSQLValue(value any) any {
	if n, ok := value.(json.Number); ok {
		return string(n)
	}
	return value
}

func rebasePublicReferences(raw string, scene int64) (string, error) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	var visit func(any) error
	visit = func(value any) error {
		switch object := value.(type) {
		case map[string]any:
			for key, child := range object {
				switch key {
				case "scene_revision":
					object[key] = strconv.FormatInt(scene, 10)
				case "content_ref", "body_ref", "detail_ref", "terminal_ref", "diff_ref":
					if encoded, ok := child.(string); ok && encoded != "" {
						ref, err := sessionstate.DecodeContentRef(encoded)
						if err != nil {
							return err
						}
						ref.SceneRevision = scene
						object[key] = ref.Encode()
					}
				default:
					if err := visit(child); err != nil {
						return err
					}
				}
			}
		case []any:
			for _, child := range object {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(value); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(value)
	return string(encoded), err
}

func reprojectRestoredThreads(tx *gorm.DB, projectID string, scene int64) error {
	var threads []threadPO
	if err := tx.Where("project_id = ?", projectID).Find(&threads).Error; err != nil {
		return err
	}
	for _, thread := range threads {
		w := newSessionWrite(tx, sessionHead{ID: thread.ID, ProjectID: projectID, SceneRevision: scene, LastSeq: thread.LastSeq, ContextRevision: thread.ContextRevision})
		var runs []runPO
		if err := tx.Where("thread_id = ?", thread.ID).Find(&runs).Error; err != nil {
			return err
		}
		for _, row := range runs {
			if err := w.publishRun(row.ID); err != nil {
				return err
			}
		}
		var commands []commandExecutionPO
		if err := tx.Table("command_executions AS c").Select("c.*").Where("c.thread_id = ? AND NOT EXISTS (SELECT 1 FROM command_executions AS newer WHERE newer.command_id = c.command_id AND newer.attempt_no > c.attempt_no)", thread.ID).Scan(&commands).Error; err != nil {
			return err
		}
		for _, row := range commands {
			command, err := row.toModel()
			if err != nil {
				return err
			}
			command.EntityRevision, command.UpdatedSeq = row.EntityRevision+1, w.seq
			if err := tx.Model(&row).Updates(map[string]any{"entity_revision": command.EntityRevision, "updated_seq": w.seq}).Error; err != nil {
				return err
			}
			if err := w.projectCommand(command); err != nil {
				return err
			}
		}
		if err := w.publishThread(); err != nil {
			return err
		}
		if err := tx.Table("threads").Where("id = ?", thread.ID).Updates(map[string]any{"last_seq": w.seq, "retained_after_seq": w.seq}).Error; err != nil {
			return err
		}
	}
	return nil
}
