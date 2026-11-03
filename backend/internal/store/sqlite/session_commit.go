package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

type sessionHead struct {
	ID               string
	ProjectID        string
	SceneRevision    int64
	SwitchState      string
	LastSeq          int64
	RetainedAfterSeq int64
	ContextRevision  int64
}

func (s *Store) ThreadSceneRevision(ctx context.Context, threadID string) (int64, error) {
	head, err := readSessionHead(s.reader.WithContext(ctx), threadID)
	return head.SceneRevision, err
}

func readSessionHead(tx *gorm.DB, threadID string) (sessionHead, error) {
	var head sessionHead
	if err := rejectDeleted(tx, "thread", threadID); err != nil {
		return head, err
	}
	head, err := readSessionHeadRow(tx, threadID)
	if err != nil {
		return head, err
	}
	if err := rejectDeleted(tx, "project", head.ProjectID); err != nil {
		return head, err
	}
	if head.SwitchState != "ready" {
		return head, sessionstate.ErrProjectRestoring
	}
	return head, nil
}

// Only deletion/recovery controllers may read a head after accepting a tombstone.
func readSessionHeadRow(tx *gorm.DB, threadID string) (sessionHead, error) {
	var head sessionHead
	err := tx.Table("threads AS t").Select("t.id,t.project_id,t.last_seq,t.retained_after_seq,t.context_revision,h.scene_revision,h.switch_state").
		Joins("JOIN project_history_state AS h ON h.project_id = t.project_id").Where("t.id = ?", threadID).Take(&head).Error
	return head, mapErr(err)
}

// sessionWrite exists for one database transaction only. Domain methods write
// business rows and their public projection through this same transaction.
type sessionWrite struct {
	tx                 *gorm.DB
	head               sessionHead
	seq                int64
	now                int64
	commitID           string
	checkpointRevision int64
	result             json.RawMessage
	changes            []sessionstate.Change
	changeIndex        map[string]int
	related            map[string]*sessionWrite
}

func (w *sessionWrite) forThread(id string) (*sessionWrite, error) {
	if id == w.head.ID {
		return w, nil
	}
	if other := w.related[id]; other != nil {
		return other, nil
	}
	head, err := readSessionHead(w.tx, id)
	if err != nil {
		return nil, err
	}
	if head.ProjectID != w.head.ProjectID || head.SceneRevision != w.head.SceneRevision {
		return nil, sessionstate.ErrSceneChanged
	}
	if w.related == nil {
		w.related = map[string]*sessionWrite{}
	}
	other := newSessionWrite(w.tx, head)
	other.now = w.now
	w.related[id] = other
	return other, nil
}

func (w *sessionWrite) change(key string, change sessionstate.Change) {
	if index, ok := w.changeIndex[key]; ok {
		w.changes[index] = change
		return
	}
	w.changeIndex[key] = len(w.changes)
	w.changes = append(w.changes, change)
}

func (w *sessionWrite) upsert(kind, id string, value any) error {
	change, err := sessionstate.Upsert(kind, value)
	if err != nil {
		return err
	}
	w.change(kind+":"+id, change)
	return nil
}

func (s *Store) commitSession(ctx context.Context, fence sessionstate.Fence, identity sessionstate.CommitIdentity, apply func(*sessionWrite) error) (sessionstate.Receipt, error) {
	var receipt sessionstate.Receipt
	if fence.ThreadID == "" || fence.ProjectID == "" || fence.SceneRevision < 1 || identity.Key == "" || identity.RequestHash == "" {
		return receipt, fmt.Errorf("%w: commit requires scene and idempotency identity", sessionstate.ErrInvalidState)
	}
	committed := false
	var relatedThreads []string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, fence.ThreadID)
		if err != nil {
			return err
		}
		if head.ProjectID != fence.ProjectID || head.SceneRevision != fence.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		scope := "session:" + strconv.FormatInt(head.SceneRevision, 10)
		var previous idempotencyPO
		err = tx.Where("scope = ? AND owner_id = ? AND key = ?", scope, fence.ThreadID, identity.Key).Take(&previous).Error
		if err == nil {
			if previous.RequestHash != identity.RequestHash {
				return sessionstate.ErrIdempotencyConflict
			}
			if previous.Status != "completed" {
				return sessionstate.ErrInvalidState
			}
			return json.Unmarshal([]byte(previous.ResultJSON), &receipt)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if fence.RunID != "" {
			var worker struct {
				ThreadID          string
				SceneRevision     int64
				OwnerInstanceID   string
				ExecutionRevision int64
				Status            string
			}
			if err := tx.Table("runs").Select("thread_id,scene_revision,owner_instance_id,execution_revision,status").Where("id = ?", fence.RunID).Take(&worker).Error; err != nil {
				return mapErr(err)
			}
			if worker.ThreadID != fence.ThreadID || worker.SceneRevision != fence.SceneRevision || worker.OwnerInstanceID != fence.OwnerInstanceID || worker.ExecutionRevision != fence.ExecutionRevision {
				return sessionstate.ErrVersionConflict
			}
			switch worker.Status {
			case "pending", "running", "waiting", "recovering":
			default:
				return sessionstate.ErrVersionConflict
			}
		}
		w := newSessionWrite(tx, head)
		if err := apply(w); err != nil {
			return err
		}
		for id := range w.related {
			relatedThreads = append(relatedThreads, id)
		}
		slices.Sort(relatedThreads)
		var related []sessionstate.RelatedCommit
		for _, id := range relatedThreads {
			other, err := w.related[id].persistBatch()
			if err != nil {
				return err
			}
			related = append(related, sessionstate.RelatedCommit{ThreadID: id, CommitID: other.CommitID, Cursor: other.Cursor, SceneRevision: other.SceneRevision, Seq: other.Seq})
		}
		receipt, err = w.persistBatch()
		if err != nil {
			return err
		}
		receipt.Related = related
		raw, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if err := tx.Create(&idempotencyPO{Scope: scope, OwnerID: head.ID, Key: identity.Key, RequestHash: identity.RequestHash, Status: "completed", ResultJSON: string(raw)}).Error; err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	if committed {
		s.notifySession(fence.ThreadID)
		for _, id := range relatedThreads {
			s.notifySession(id)
		}
	}
	return receipt, nil
}

func newSessionWrite(tx *gorm.DB, head sessionHead) *sessionWrite {
	return &sessionWrite{tx: tx, head: head, seq: head.LastSeq + 1, now: time.Now().UnixMilli(), commitID: model.MustShortID("commit"), changes: []sessionstate.Change{}, changeIndex: map[string]int{}}
}

// Persist the batch inside the caller's transaction; notifications must wait
// until that transaction has committed, including multi-thread deletion work.
func (w *sessionWrite) persistBatch() (sessionstate.Receipt, error) {
	batch := sessionstate.Batch{ProjectID: w.head.ProjectID, ThreadID: w.head.ID, SceneRevision: w.head.SceneRevision, Seq: w.seq, CommitID: w.commitID, Changes: w.changes}
	raw, err := json.Marshal(batch)
	if err != nil {
		return sessionstate.Receipt{}, err
	}
	if err := w.tx.Table("thread_changes").Create(map[string]any{"thread_id": w.head.ID, "scene_revision": w.head.SceneRevision, "seq": w.seq, "commit_id": w.commitID, "payload_json": string(raw), "created_at": w.now}).Error; err != nil {
		return sessionstate.Receipt{}, err
	}
	result := w.tx.Table("threads").Where("id = ? AND last_seq = ?", w.head.ID, w.head.LastSeq).Update("last_seq", w.seq)
	if result.Error != nil {
		return sessionstate.Receipt{}, result.Error
	}
	if result.RowsAffected != 1 {
		return sessionstate.Receipt{}, sessionstate.ErrVersionConflict
	}
	return sessionstate.Receipt{CommitID: w.commitID, Cursor: batch.Cursor().Encode(), SceneRevision: w.head.SceneRevision, Seq: w.seq, ContextRevision: w.head.ContextRevision, CheckpointRevision: w.checkpointRevision, Result: w.result}, nil
}

// SubscribeSession must precede range catch-up. Signals may coalesce; consumers
// always read the durable range rather than treating a signal as a payload.
func (s *Store) SubscribeSession(threadID string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.changeMu.Lock()
	if s.changeListeners == nil {
		s.changeListeners = make(map[string]map[chan struct{}]struct{})
	}
	if s.changeListeners[threadID] == nil {
		s.changeListeners[threadID] = make(map[chan struct{}]struct{})
	}
	s.changeListeners[threadID][ch] = struct{}{}
	s.changeMu.Unlock()
	return ch, func() {
		s.changeMu.Lock()
		defer s.changeMu.Unlock()
		delete(s.changeListeners[threadID], ch)
		if len(s.changeListeners[threadID]) == 0 {
			delete(s.changeListeners, threadID)
		}
	}
}

func (s *Store) notifySession(threadID string) {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	for ch := range s.changeListeners[threadID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *Store) SessionChanges(ctx context.Context, cursor sessionstate.Cursor, limit int) (sessionstate.ChangePage, error) {
	page := sessionstate.ChangePage{Batches: []sessionstate.Batch{}, Cursor: cursor.Encode()}
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, cursor.ThreadID)
		if err != nil {
			return err
		}
		if head.ProjectID != cursor.ProjectID || head.SceneRevision != cursor.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		if cursor.Seq < head.RetainedAfterSeq || cursor.Seq > head.LastSeq {
			return sessionstate.ErrCursorReset
		}
		var rows []struct {
			Seq         int64
			PayloadJSON string
		}
		if err := tx.Table("thread_changes").Select("seq,payload_json").Where("thread_id = ? AND scene_revision = ? AND seq > ?", cursor.ThreadID, cursor.SceneRevision, cursor.Seq).Order("seq ASC").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		position := cursor.Seq
		for _, row := range rows {
			var batch sessionstate.Batch
			if err := json.Unmarshal([]byte(row.PayloadJSON), &batch); err != nil {
				return err
			}
			if row.Seq != position+1 || batch.Seq != row.Seq || batch.ThreadID != cursor.ThreadID || batch.ProjectID != cursor.ProjectID || batch.SceneRevision != cursor.SceneRevision {
				return sessionstate.ErrInvalidState
			}
			page.Batches = append(page.Batches, batch)
			page.Cursor = batch.Cursor().Encode()
			position = row.Seq
		}
		if len(rows) == 0 && position < head.LastSeq {
			return sessionstate.ErrInvalidState
		}
		page.HasMore = position < head.LastSeq
		return nil
	})
	return page, err
}

// PruneSessionChanges bounds each maintenance pass to 20 short transactions.
// Deleted/restoring projects retain their recovery data until their own lifecycle
// has settled. Ordinary reads never wait for this maintenance pass.
func (s *Store) PruneSessionChanges(ctx context.Context) error {
	cutoff := time.Now().Add(-7 * 24 * time.Hour).UnixMilli()
	var ids []string
	if err := s.reader.WithContext(ctx).Raw(`SELECT t.id FROM threads t
 JOIN project_history_state h ON h.project_id = t.project_id
 JOIN thread_changes c ON c.thread_id = t.id AND c.scene_revision = h.scene_revision AND c.seq = t.retained_after_seq + 1
 WHERE h.switch_state = 'ready' AND t.last_seq - t.retained_after_seq > 10000
 AND NOT EXISTS (SELECT 1 FROM idempotency_records d WHERE d.scope = 'delete'
   AND ((d.key = 'thread' AND d.owner_id = t.id) OR (d.key = 'project' AND d.owner_id = t.project_id)))
 AND c.created_at < ?
 ORDER BY c.created_at,t.id LIMIT 20`, cutoff).Scan(&ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.pruneSessionChanges(ctx, id, cutoff); err != nil && !errors.Is(err, run.ErrRunNotFound) && !errors.Is(err, sessionstate.ErrProjectRestoring) {
			return err
		}
	}
	return nil
}

// Delete only a contiguous prefix outside both retention windows. State and
// operation receipts remain independently readable.
func (s *Store) pruneSessionChanges(ctx context.Context, threadID string, cutoff int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		if head.LastSeq-head.RetainedAfterSeq <= 10000 {
			return nil
		}
		var rows []struct {
			Seq       int64
			CreatedAt int64
		}
		if err := tx.Table("thread_changes").Select("seq,created_at").Where("thread_id = ? AND scene_revision = ? AND seq > ? AND seq <= ?", threadID, head.SceneRevision, head.RetainedAfterSeq, head.LastSeq-10000).Order("seq ASC").Limit(500).Find(&rows).Error; err != nil {
			return err
		}
		through := head.RetainedAfterSeq
		for _, row := range rows {
			if row.Seq != through+1 {
				return sessionstate.ErrInvalidState
			}
			if row.CreatedAt >= cutoff {
				break
			}
			through = row.Seq
		}
		if through == head.RetainedAfterSeq {
			return nil
		}
		if err := tx.Exec("DELETE FROM thread_changes WHERE thread_id = ? AND scene_revision = ? AND seq <= ?", threadID, head.SceneRevision, through).Error; err != nil {
			return err
		}
		return tx.Table("threads").Where("id = ?", threadID).Update("retained_after_seq", through).Error
	})
}
