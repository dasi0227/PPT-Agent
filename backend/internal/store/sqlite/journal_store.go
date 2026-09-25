package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	"gorm.io/gorm"
)

type journalThreadPO struct {
	ID                string `gorm:"primaryKey"`
	ProjectID         string
	NextEventSeq      int64
	DeliveredEventSeq int64
}

func (journalThreadPO) TableName() string { return "threads" }

type outboxPO struct {
	ThreadID  string `gorm:"primaryKey"`
	Seq       int64  `gorm:"primaryKey"`
	EventID   string
	EventJSON string
}

func (outboxPO) TableName() string { return "thread_event_outbox" }

func (s *Store) projectRoot(projectID string) string {
	return filepath.Join(s.workRoot, "projects", projectID)
}

// enqueueEvent participates in the caller's business transaction. Delivery is
// deliberately outside that transaction; failure leaves the intent recoverable.
func (s *Store) enqueueEvent(tx *gorm.DB, threadID string, event threadjournal.Event) (threadjournal.Event, error) {
	if event.RunID != "" {
		if owner, execution, ok := workflow.CheckpointOwnership(tx.Statement.Context); ok {
			var row struct {
				ExecutionRevision int64
				OwnerInstanceID   string
			}
			if err := tx.Session(&gorm.Session{NewDB: true}).Table("runs").Select("execution_revision", "owner_instance_id").Where("id = ?", event.RunID).Take(&row).Error; err != nil {
				return event, mapErr(err)
			}
			if row.ExecutionRevision != execution || row.OwnerInstanceID != owner {
				return event, run.ErrRunRevisionConflict
			}
		}
	}

	var pending int64
	if err := tx.Model(&outboxPO{}).Where("thread_id = ?", threadID).Count(&pending).Error; err != nil {
		return event, err
	}
	if pending >= 1024 {
		return event, errors.New("thread outbox is full; execution must pause")
	}
	var thread journalThreadPO
	if err := tx.First(&thread, "id = ?", threadID).Error; err != nil {
		return event, mapErr(err)
	}
	if event.ID == "" {
		event.ID = model.MustShortID("evt")
	}
	if event.TS == 0 {
		event.TS = time.Now().UnixMilli()
	}
	event.Seq = thread.NextEventSeq
	if event.Type == "" || !json.Valid(event.Payload) {
		return event, errors.New("event requires type and valid payload")
	}
	var err error
	event, err = threadjournal.Prepare(s.projectRoot(thread.ProjectID), threadID, event)
	if err != nil {
		return event, err
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return event, err
	}
	if len(raw) > threadjournal.MaxRecordBytes {
		return event, errors.New("event envelope exceeds journal limit")
	}
	result := tx.Model(&journalThreadPO{}).Where("id = ? AND next_event_seq = ?", threadID, event.Seq).
		Updates(map[string]any{"next_event_seq": event.Seq + 1})
	if result.Error != nil {
		return event, result.Error
	}
	if result.RowsAffected != 1 {
		return event, errors.New("thread event sequence changed")
	}
	err = tx.Create(&outboxPO{ThreadID: threadID, Seq: event.Seq, EventID: event.ID, EventJSON: string(raw)}).Error
	return event, err
}

func (s *Store) AppendThreadEvent(ctx context.Context, threadID string, event threadjournal.Event) (threadjournal.Event, error) {
	var saved threadjournal.Event
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		saved, err = s.enqueueEvent(tx, threadID, event)
		return err
	})
	if err != nil {
		return saved, err
	}
	return saved, s.FlushThreadEvents(ctx, threadID)
}

func (s *Store) FlushThreadEvents(ctx context.Context, threadID string) error {
	value, _ := s.deliveryLocks.LoadOrStore(threadID, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	err := s.flushThreadEventsLocked(ctx, threadID)
	if err != nil {
		s.deliveryFailures.Store(threadID, err)
	} else {
		s.deliveryFailures.Delete(threadID)
	}
	return err
}

func (s *Store) flushThreadEventsLocked(ctx context.Context, threadID string) error {
	var thread journalThreadPO
	if err := s.db.WithContext(ctx).First(&thread, "id = ?", threadID).Error; err != nil {
		return mapErr(err)
	}
	for {
		var rows []outboxPO
		if err := s.db.WithContext(ctx).Where("thread_id = ?", threadID).Order("seq ASC").Limit(64).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			var event threadjournal.Event
			if json.Unmarshal([]byte(row.EventJSON), &event) != nil || event.ID != row.EventID || event.Seq != row.Seq {
				return threadjournal.ErrCorrupt
			}
			if err := threadjournal.Deliver(s.projectRoot(thread.ProjectID), thread.ProjectID, threadID, event); err != nil {
				return err
			}
			if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				result := tx.Model(&journalThreadPO{}).Where("id = ? AND delivered_event_seq = ?", threadID, event.Seq-1).
					Update("delivered_event_seq", event.Seq)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return fmt.Errorf("journal delivery watermark conflict for %s", threadID)
				}
				return tx.Where("thread_id = ? AND seq = ? AND event_id = ?", threadID, event.Seq, event.ID).Delete(&outboxPO{}).Error
			}); err != nil {
				return err
			}
		}
	}
}

func (s *Store) ThreadEvents(ctx context.Context, threadID string, after int64) ([]threadjournal.Event, error) {
	value, _ := s.deliveryLocks.LoadOrStore(threadID, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	if err := s.flushThreadEventsLocked(ctx, threadID); err != nil {
		s.deliveryFailures.Store(threadID, err)
		return nil, err
	}
	s.deliveryFailures.Delete(threadID)
	var thread journalThreadPO
	if err := s.db.WithContext(ctx).First(&thread, "id = ?", threadID).Error; err != nil {
		return nil, mapErr(err)
	}
	events, err := threadjournal.Read(s.projectRoot(thread.ProjectID), thread.ProjectID, threadID)
	if err != nil {
		s.deliveryFailures.Store(threadID, err)
		return nil, err
	}
	if int64(len(events)) != thread.DeliveredEventSeq {
		return nil, fmt.Errorf("%w: database and journal watermarks differ", threadjournal.ErrCorrupt)
	}
	out := make([]threadjournal.Event, 0)
	for _, event := range events {
		if event.Seq > after {
			out = append(out, event)
		}
	}
	return out, nil
}

func (s *Store) FlushProjectEvents(ctx context.Context, projectID string) error {
	threads, err := s.ListThreads(ctx, projectID)
	if err != nil {
		return err
	}
	for _, thread := range threads {
		if err := s.FlushThreadEvents(ctx, thread.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) recoverOutbox(ctx context.Context) error {
	var threads []journalThreadPO
	if err := s.db.WithContext(ctx).Find(&threads).Error; err != nil {
		return err
	}
	for _, thread := range threads {
		if err := threadjournal.Recover(s.projectRoot(thread.ProjectID), thread.ProjectID, thread.ID, thread.DeliveredEventSeq); err != nil {
			return err
		}
		if _, err := s.ThreadEvents(ctx, thread.ID, 0); err != nil {
			return err
		}
	}
	return nil
}

// Reserve space for final events of already accepted work. Delivery recovery
// does not enqueue records and can always drain the bounded outbox.
func (s *Store) checkAcceptance(tx *gorm.DB, threadID string) error {
	var blocked bool
	s.deliveryFailures.Range(func(_, _ any) bool { blocked = true; return false })
	if blocked {
		return errors.New("history delivery failed; restore storage before accepting work")
	}
	var count int64
	if err := tx.Model(&outboxPO{}).Where("thread_id = ?", threadID).Count(&count).Error; err != nil {
		return err
	}
	if count >= 768 {
		return errors.New("history delivery is blocked; retry after storage recovers")
	}
	return nil
}
