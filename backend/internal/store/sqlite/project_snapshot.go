package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"strings"
)

// The ordered inventory is intentionally explicit: global libraries and real Git
// history are excluded. Child rows precede their parents when deleting.
var projectTables = []string{"projects", "slides", "threads", "thread_naming_inputs", "thread_naming_operations", "runs", "deleted_slides", "run_events", "run_contexts", "steering_inbox", "run_checkpoints", "context_index_snapshots", "semantic_reviews", "git_commit_operations", "git_commit_events", "briefing_versions", "context_compactions", "command_activities", "idempotency_records"}

func projectPredicate(table string) string {
	switch table {
	case "projects":
		return "id = @project"
	case "thread_naming_inputs", "thread_naming_operations":
		return "thread_id IN (SELECT id FROM threads WHERE project_id = @project)"
	case "run_events", "run_contexts", "steering_inbox", "run_checkpoints", "context_index_snapshots", "semantic_reviews":
		return "run_id IN (SELECT id FROM runs WHERE project_id = @project)"
	case "git_commit_events":
		return "operation_id IN (SELECT id FROM git_commit_operations WHERE project_id = @project)"
	case "idempotency_records":
		return "(owner_id = @project OR owner_id IN (SELECT id FROM threads WHERE project_id = @project) OR owner_id IN (SELECT id FROM runs WHERE project_id = @project))"
	default:
		return "project_id = @project"
	}
}
func snapshotArgs(id string) map[string]any {
	return map[string]any{"project": id}
}
func (s *Store) CaptureProject(ctx context.Context, id string) (json.RawMessage, error) {
	data := map[string][]map[string]any{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range projectTables {
			var rows []map[string]any
			if err := tx.Raw("SELECT * FROM "+table+" WHERE "+projectPredicate(table)+snapshotFilter(table), snapshotArgs(id)).Scan(&rows).Error; err != nil {
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
func (s *Store) RestoreProject(ctx context.Context, id string, raw json.RawMessage) error {
	var data map[string][]map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return err
	}
	if len(data) != len(projectTables) || len(data["projects"]) != 1 || data["projects"][0]["id"] != id {
		return fmt.Errorf("invalid project snapshot inventory")
	}
	for _, table := range projectTables {
		if _, present := data[table]; !present {
			return fmt.Errorf("invalid project snapshot inventory: missing %s", table)
		}
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Defer FK checks until all parents and children have been restored.
		if err := tx.Exec("PRAGMA defer_foreign_keys = ON").Error; err != nil {
			return err
		}
		// Deleted threads/runs can leave non-FK idempotency records behind.
		// Include target owners, not only owners still present in the live DB.
		owners := []string{id}
		for _, table := range []string{"threads", "runs"} {
			for _, row := range data[table] {
				if owner, ok := row["id"].(string); ok {
					owners = append(owners, owner)
				}
			}
		}
		if err := tx.Exec("DELETE FROM idempotency_records WHERE owner_id IN ?", owners).Error; err != nil {
			return err
		}
		for i := len(projectTables) - 1; i >= 0; i-- {
			table := projectTables[i]
			if err := tx.Exec("DELETE FROM "+table+" WHERE "+projectPredicate(table), snapshotArgs(id)).Error; err != nil {
				return err
			}
		}
		for _, table := range projectTables {
			for _, row := range data[table] {
				cols := []string{}
				args := []any{}
				marks := []string{}
				for col, v := range row {
					cols = append(cols, `"`+strings.ReplaceAll(col, `"`, `""`)+`"`)
					marks = append(marks, "?")
					if n, ok := v.(json.Number); ok {
						v = string(n)
					}
					args = append(args, v)
				}
				if err := tx.Exec("INSERT INTO "+table+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(marks, ",")+")", args...).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func snapshotFilter(table string) string {
	if table == "command_activities" {
		return " AND status != 'loading'"
	}
	if table == "idempotency_records" {
		return " AND NOT (scope = 'create_run' AND status = 'in_progress')"
	}
	if table == "thread_naming_operations" {
		return " AND status NOT IN ('in_progress','accepted')"
	}
	return ""
}
