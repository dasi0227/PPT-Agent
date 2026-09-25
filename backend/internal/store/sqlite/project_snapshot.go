package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

// The ordered inventory is intentionally explicit: global libraries and real Git
// history are excluded. Child rows precede their parents when deleting.
var projectTables = []string{"projects", "slides", "threads", "runs", "steering_inbox", "command_executions", "idempotency_records"}

func projectPredicate(table string) string {
	switch table {
	case "projects":
		return "id = @project"
	case "steering_inbox":
		return "thread_id IN (SELECT id FROM threads WHERE project_id = @project)"
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
	if err := s.FlushProjectEvents(ctx, id); err != nil {
		return nil, err
	}
	data := map[string][]map[string]any{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range projectTables {
			var rows []map[string]any
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
		// Remember the live epochs before replacing rows. A restored checkpoint
		// never grants a lease back to a worker from either side of the rollback.
		epochs := map[string]map[string]int64{}
		for _, table := range []string{"runs", "command_executions"} {
			var rows []struct {
				ID                string
				ExecutionRevision int64
			}
			if err := tx.Table(table).Select("id,execution_revision").Where("project_id = ?", id).Find(&rows).Error; err != nil {
				return err
			}
			epochs[table] = map[string]int64{}
			for _, row := range rows {
				epochs[table][row.ID] = row.ExecutionRevision
			}
			for _, row := range data[table] {
				revision, err := strconv.ParseInt(fmt.Sprint(row["execution_revision"]), 10, 64)
				if err != nil {
					return fmt.Errorf("invalid execution revision: %w", err)
				}
				if live := epochs[table][fmt.Sprint(row["id"])]; live > revision {
					revision = live
				}
				row["execution_revision"] = revision + 1
				row["owner_instance_id"] = ""
				if table == "runs" {
					switch row["status"] {
					case "pending", "running", "waiting", "recovering":
						row["status"] = "paused"
						row["pause_reason"] = "project_restored"
					}
				} else if activeCommandStatus(fmt.Sprint(row["status"])) {
					// Captures require idle commands; reject malformed snapshots.
					return fmt.Errorf("snapshot contains an active command")
				}
			}
		}
		var liveThreads []threadPO
		if err := tx.Where("project_id = ?", id).Find(&liveThreads).Error; err != nil {
			return err
		}
		for _, row := range data["threads"] {
			for _, field := range []string{"naming_revision", "rename_operation_version"} {
				version, err := strconv.ParseInt(fmt.Sprint(row[field]), 10, 64)
				if err != nil {
					return fmt.Errorf("invalid naming version: %w", err)
				}
				for _, live := range liveThreads {
					if live.ID == row["id"] {
						current := live.NamingRevision
						if field == "rename_operation_version" {
							current = live.RenameOperationVersion
						}
						if current > version {
							version = current
						}
					}
				}
				row[field] = version + 1
			}
		}
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
