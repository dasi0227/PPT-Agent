package sqlite

import (
	"context"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
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
	if err := s.db.WithContext(ctx).Where("scope = ? AND status = ? AND (key = 'project' OR request_hash NOT IN (SELECT owner_id FROM idempotency_records WHERE scope = 'delete' AND key = 'project'))", deletionScope, "in_progress").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]store.Deletion, 0, len(rows))
	for _, row := range rows {
		out = append(out, store.Deletion{Kind: row.Key, ID: row.OwnerID, ProjectID: row.RequestHash})
	}
	return out, nil
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
	threads := []string{target.ID}
	if target.Kind == "project" {
		var err error
		threads, err = s.DeletedThreads(ctx, target.ProjectID)
		if err != nil {
			return err
		}
	}
	// A delivery that read its row before deletion may still be appending bytes.
	// Waiting here keeps that work out of the request's acceptance path.
	for _, id := range threads {
		unlock := s.lockThreadDelivery(id)
		defer unlock()
		s.deliveryFailures.Delete(id)
	}
	path := s.projectRoot(target.ProjectID)
	if target.Kind == "thread" {
		path = filepath.Join(path, "threads", target.ID)
	}
	return os.RemoveAll(path)
}

// Delete the authoritative rows and accept cleanup in one transaction. No
// runtime, delivery, history or project execution lock is acquired here.
func (s *Store) deleteTarget(ctx context.Context, kind, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if deleted, err := deletionRecorded(tx, kind, id); err != nil || deleted {
			return err
		}
		projectID := id
		if kind == "thread" {
			var thread threadPO
			if err := tx.Select("id", "project_id").First(&thread, "id = ?", id).Error; err != nil {
				return mapErr(err)
			}
			projectID = thread.ProjectID
		} else {
			var project struct{ ID string }
			if err := tx.Table("projects").Select("id").Where("id = ?", id).Take(&project).Error; err != nil {
				return mapErr(err)
			}
		}
		if err := tx.Create(&idempotencyPO{Scope: deletionScope, OwnerID: id, Key: kind, RequestHash: projectID, Status: "in_progress", ResultJSON: "{}"}).Error; err != nil {
			return err
		}
		if kind == "project" {
			if err := tx.Exec("INSERT OR IGNORE INTO idempotency_records(scope,owner_id,key,request_hash,status,result_json) SELECT 'delete',id,'thread',project_id,'in_progress','{}' FROM threads WHERE project_id = ?", projectID).Error; err != nil {
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
		if result.RowsAffected != 1 {
			return run.ErrRunNotFound
		}
		return nil
	})
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
	var deleted []string
	if err := tx.Model(&idempotencyPO{}).Where("scope = ? AND key = ? AND request_hash = ?", deletionScope, "thread", projectID).Pluck("owner_id", &deleted).Error; err != nil {
		return err
	}
	removed := map[string]bool{}
	for _, id := range deleted {
		removed[id] = true
	}
	for _, table := range []string{"threads", "runs", "command_executions", "steering_inbox", "idempotency_records"} {
		rows := data[table][:0]
		for _, row := range data[table] {
			owner, _ := row["thread_id"].(string)
			if table == "threads" {
				owner, _ = row["id"].(string)
			} else if table == "idempotency_records" {
				owner, _ = row["owner_id"].(string)
			}
			if removed[owner] || (table == "idempotency_records" && row["scope"] == deletionScope) {
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
