package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
)

const deletionScope = "delete"

func deletionRecorded(tx *gorm.DB, kind, id string) (bool, error) {
	var count int64
	err := tx.Model(&idempotencyPO{}).Where("scope = ? AND owner_id = ? AND key = ?", deletionScope, id, kind).Count(&count).Error
	return count > 0, err
}

func (s *Store) ProjectDeleted(ctx context.Context, id string) (bool, error) {
	return deletionRecorded(s.db.WithContext(ctx), "project", id)
}

func (s *Store) DeletedThreads(ctx context.Context, projectID string) ([]string, error) {
	var ids []string
	err := s.db.WithContext(ctx).Model(&idempotencyPO{}).
		Where("scope = ? AND key = ? AND request_hash = ?", deletionScope, "thread", projectID).Pluck("owner_id", &ids).Error
	return ids, err
}

func (s *Store) PendingDeletions(ctx context.Context) ([]store.Deletion, error) {
	var rows []idempotencyPO
	if err := s.db.WithContext(ctx).Where("scope = ? AND status = 'in_progress' AND json_extract(result_json, '$.phase') IN ('pending','restoring','in_progress') AND (key = 'project' OR request_hash NOT IN (SELECT owner_id FROM idempotency_records WHERE scope = 'delete' AND key = 'project' AND (status = 'completed' OR json_extract(result_json, '$.phase') = 'in_progress')))", deletionScope).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]store.Deletion, 0, len(rows))
	for _, row := range rows {
		target, err := decodeDeletion(row)
		if err != nil {
			return nil, err
		}
		out = append(out, target)
	}
	return out, nil
}

type deletionReceipt struct {
	store.Deletion
	Phase string `json:"phase"`
}

func decodeDeletion(row idempotencyPO) (store.Deletion, error) {
	var receipt deletionReceipt
	if err := json.Unmarshal([]byte(row.ResultJSON), &receipt); err != nil {
		return store.Deletion{}, err
	}
	target := receipt.Deletion
	target.Kind, target.ID, target.ProjectID, target.Status = row.Key, row.OwnerID, row.RequestHash, row.Status
	if row.Status == "in_progress" {
		target.Status = receipt.Phase
	}
	return target, nil
}

func (s *Store) GetDeletion(ctx context.Context, kind, id string) (store.Deletion, error) {
	var row idempotencyPO
	if err := s.db.WithContext(ctx).First(&row, "scope = ? AND owner_id = ? AND key = ?", deletionScope, id, kind).Error; err != nil {
		return store.Deletion{}, err
	}
	return decodeDeletion(row)
}

func (s *Store) HasPendingDeletion(ctx context.Context, projectID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&idempotencyPO{}).Where("scope = ? AND request_hash = ? AND status = 'in_progress' AND json_extract(result_json, '$.phase') IN ('pending','restoring')", deletionScope, projectID).Count(&count).Error
	return count > 0, err
}

func rejectPendingDeletion(tx *gorm.DB, projectID string) error {
	var count int64
	if err := tx.Model(&idempotencyPO{}).Where("scope = ? AND request_hash = ? AND status = 'in_progress' AND json_extract(result_json, '$.phase') IN ('pending','restoring')", deletionScope, projectID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return store.ErrDeletionPending
	}
	return nil
}

// Claim restoration before waiting for canceled workers. The durable restoring
// state wins against expiry and keeps new work fenced until those workers exit.
func (s *Store) RequestDeletionRestore(ctx context.Context, target store.Deletion) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row idempotencyPO
		if err := tx.First(&row, "scope = ? AND owner_id = ? AND key = ?", deletionScope, target.ID, target.Kind).Error; err != nil {
			return store.ErrDeletionExpired
		}
		current, err := decodeDeletion(row)
		if err != nil {
			return err
		}
		if current.Token != target.Token {
			return store.ErrDeletionExpired
		}
		if target.Kind == "thread" {
			if err := rejectDeleted(tx, "project", current.ProjectID); err != nil {
				return err
			}
		}
		if current.Status == "restoring" {
			return nil
		}
		if current.Status != "pending" || time.Now().UnixMilli() >= current.UndoUntil {
			return store.ErrDeletionExpired
		}
		return tx.Model(&row).Update("result_json", gorm.Expr("json_set(result_json, '$.phase', 'restoring')")).Error
	})
}

func (s *Store) FinishDeletionRestore(ctx context.Context, target store.Deletion) error {
	return s.db.WithContext(ctx).Where("scope = ? AND owner_id = ? AND key = ? AND status = 'in_progress' AND json_extract(result_json, '$.phase') = 'restoring' AND json_extract(result_json, '$.token') = ?", deletionScope, target.ID, target.Kind, target.Token).Delete(&idempotencyPO{}).Error
}

// Workers are already stopped. Persist cancellation for retained runs, including
// paused runs recovered after a server restart; undo never resumes old work.
func (s *Store) QuiesceDeletion(ctx context.Context, target store.Deletion) error {
	changed := map[string]bool{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []runPO
		query := tx.Where("project_id = ? AND status NOT IN ?", target.ProjectID, []string{"done", "failed", "canceled"})
		if target.Kind == "thread" {
			query = query.Where("thread_id = ?", target.ID)
		}
		if err := query.Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			head, err := readSessionHeadRow(tx, row.ThreadID)
			if err != nil {
				return err
			}
			w := newSessionWrite(tx, head)
			if err := w.pauseRun(row, "deleted", w.now); err != nil {
				return err
			}
			terminal := model.NewRunTerminalPayload(row.ID, max(0, w.now-row.CreatedAt), nil, nil)
			payload, err := json.Marshal(terminal)
			if err != nil {
				return err
			}
			if err := tx.Table("runs").Where("id = ?", row.ID).Updates(map[string]any{
				"terminal_json": string(payload), "continuation_allowed": 0, "execution_revision": row.ExecutionRevision + 1,
				"checkpoint_json": gorm.Expr("CASE WHEN checkpoint_json IS NULL THEN NULL ELSE json_set(checkpoint_json, '$.continuation_allowed', json('false')) END"),
			}).Error; err != nil {
				return err
			}
			if err := w.setRunStatus(row, model.RunCanceled); err != nil {
				return err
			}
			if err := w.publishRun(row.ID); err != nil {
				return err
			}
			if err := w.putItem(sessionstate.TimelineItem{ID: "terminal:" + row.ID, ThreadID: row.ThreadID, GroupID: "run:" + row.ID, Kind: "terminal_notice", Summary: payload}); err != nil {
				return err
			}
			if _, err := w.persistBatch(); err != nil {
				return err
			}
			changed[row.ThreadID] = true
		}
		return nil
	})
	if err == nil {
		for threadID := range changed {
			s.notifySession(threadID)
		}
	}
	return err
}

func (s *Store) CompleteDeletion(ctx context.Context, target store.Deletion) error {
	query := s.db.WithContext(ctx).Model(&idempotencyPO{}).Where("scope = ?", deletionScope)
	if target.Kind == "project" {
		query = query.Where("request_hash = ?", target.ProjectID)
	} else {
		query = query.Where("owner_id = ? AND key = ?", target.ID, target.Kind)
	}
	return query.Update("status", "completed").Error
}

func (s *Store) PurgeDeletion(ctx context.Context, target store.Deletion) error {
	path := s.projectRoot(target.ProjectID)
	if target.Kind == "thread" {
		path = filepath.Join(path, "threads", target.ID)
	}
	return os.RemoveAll(path)
}

// Accept deletion without waiting for workers or history locks. Keep all rows
// and files until expiry; the receipt hides the target and fences new work.
func (s *Store) deleteTarget(ctx context.Context, kind, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if deleted, err := deletionRecorded(tx, kind, id); err != nil || deleted {
			return err
		}
		projectID := id
		title := ""
		if kind == "thread" {
			var thread threadPO
			if err := tx.Select("id", "project_id", "title").First(&thread, "id = ?", id).Error; err != nil {
				return mapErr(err)
			}
			projectID = thread.ProjectID
			title = thread.Title
			if err := rejectDeleted(tx, "project", projectID); err != nil {
				return err
			}
		} else {
			var project struct{ ID, Title string }
			if err := tx.Table("projects").Select("id", "title").Where("id = ?", id).Take(&project).Error; err != nil {
				return mapErr(err)
			}
			title = project.Title
		}
		target := store.Deletion{Kind: kind, ID: id, ProjectID: projectID, Title: title, Token: model.MustShortID("del"), UndoUntil: time.Now().Add(5 * time.Second).UnixMilli()}
		raw, err := json.Marshal(deletionReceipt{Deletion: target, Phase: "pending"})
		if err != nil {
			return err
		}
		return tx.Create(&idempotencyPO{Scope: deletionScope, OwnerID: id, Key: kind, RequestHash: projectID, Status: "in_progress", ResultJSON: string(raw)}).Error
	})
}

// Expiry and undo compete in the same database transaction. A stale worker or
// Toast token cannot finalize or restore a later deletion of the same target.
func (s *Store) FinalizeDeletion(ctx context.Context, target store.Deletion) (bool, error) {
	claimed := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row idempotencyPO
		if err := tx.First(&row, "scope = ? AND owner_id = ? AND key = ?", deletionScope, target.ID, target.Kind).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		current, err := decodeDeletion(row)
		if err != nil {
			return err
		}
		if current.Token != target.Token {
			return nil
		}
		if current.Status == "in_progress" {
			claimed = true
			return nil
		}
		if current.Status != "pending" || time.Now().UnixMilli() < current.UndoUntil {
			return nil
		}
		if err := tx.Model(&row).Update("result_json", gorm.Expr("json_set(result_json, '$.phase', 'in_progress')")).Error; err != nil {
			return err
		}
		kind, id, projectID := target.Kind, target.ID, target.ProjectID
		if kind == "project" {
			raw, err := json.Marshal(deletionReceipt{Deletion: target, Phase: "in_progress"})
			if err != nil {
				return err
			}
			if err := tx.Exec("INSERT OR IGNORE INTO idempotency_records(scope,owner_id,key,request_hash,status,result_json) SELECT 'delete',id,'thread',project_id,'in_progress',? FROM threads WHERE project_id = ?", string(raw), projectID).Error; err != nil {
				return err
			}
			if err := tx.Model(&idempotencyPO{}).Where("scope = ? AND key = 'thread' AND request_hash = ?", deletionScope, projectID).Update("result_json", gorm.Expr("json_set(result_json, '$.phase', 'in_progress')")).Error; err != nil {
				return err
			}
		}
		owners := tx.Table("runs").Select("id").Where("project_id = ?", projectID)
		threads := tx.Table("threads").Select("id").Where("project_id = ?", projectID)
		if kind == "thread" {
			owners = tx.Table("runs").Select("id").Where("thread_id = ?", id)
			threads = tx.Table("threads").Select("id").Where("id = ?", id)
		}
		if err := tx.Where("scope <> ? AND (owner_id = ? OR owner_id IN (?) OR owner_id IN (?))", deletionScope, id, owners, threads).Delete(&idempotencyPO{}).Error; err != nil {
			return err
		}
		var result *gorm.DB
		if kind == "thread" {
			result = tx.Where("id = ?", id).Delete(&threadPO{})
		} else {
			result = tx.Where("id = ?", id).Delete(&projectPO{})
		}
		if result.Error != nil {
			return result.Error
		}
		claimed = true
		return nil
	})
	return claimed, err
}

func rejectDeleted(tx *gorm.DB, kind, id string) error {
	deleted, err := deletionRecorded(tx, kind, id)
	if err != nil {
		return err
	}
	if deleted {
		return run.ErrRunNotFound
	}
	return nil
}

// A restore can be racing a delete, or can contain a previously deleted thread.
// Receipts are outside snapshots, so neither case can revive deleted targets.
func filterDeletedSnapshot(tx *gorm.DB, projectID string, data map[string][]map[string]any) error {
	if err := rejectDeleted(tx, "project", projectID); err != nil {
		return err
	}
	if err := rejectPendingDeletion(tx, projectID); err != nil {
		return err
	}
	var deleted []string
	if err := tx.Model(&idempotencyPO{}).Where("scope = ? AND key = ? AND request_hash = ?", deletionScope, "thread", projectID).Pluck("owner_id", &deleted).Error; err != nil {
		return err
	}
	removed := map[string]bool{}
	for _, id := range deleted {
		removed[id] = true
	}
	for _, table := range projectTables[1:] {
		rows := data[table][:0]
		for _, row := range data[table] {
			owner, _ := row["thread_id"].(string)
			if table == "threads" {
				owner, _ = row["id"].(string)
			} else if table == "idempotency_records" {
				owner, _ = row["owner_id"].(string)
			}
			if removed[owner] || (table == "idempotency_records" && (row["scope"] == deletionScope || row["scope"] == historyReceiptScope)) {
				if id, ok := row["id"].(string); ok {
					removed[id] = true
				}
				continue
			}
			rows = append(rows, row)
		}
		data[table] = rows
	}
	return nil
}
