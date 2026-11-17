package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

func (s *Store) LoadContext(ctx context.Context, threadID string) (sessionstate.ContextSnapshot, error) {
	return s.loadContext(ctx, threadID, nil)
}

func (s *Store) LoadContextVersion(ctx context.Context, threadID string, revision int64) (sessionstate.ContextSnapshot, error) {
	return s.loadContext(ctx, threadID, &revision)
}

func (s *Store) loadContext(ctx context.Context, threadID string, revision *int64) (sessionstate.ContextSnapshot, error) {
	var snapshot sessionstate.ContextSnapshot
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		current := head.ContextRevision
		if revision != nil {
			current = *revision
		}
		snapshot = sessionstate.ContextSnapshot{ThreadID: threadID, SceneRevision: head.SceneRevision, Revision: current, Messages: []sessionstate.Message{}}
		var version struct{ MessageIDsJSON string }
		if err := tx.Table("thread_context_versions").Select("message_ids_json").Where("thread_id = ? AND revision = ?", threadID, current).Take(&version).Error; err != nil {
			return fmt.Errorf("context version %d: %w", current, err)
		}
		var ids []string
		if err := json.Unmarshal([]byte(version.MessageIDsJSON), &ids); err != nil {
			return err
		}
		var rows []struct {
			ID                   string
			RunID                string
			InputID              string
			RoundID              string
			Kind                 string
			NativeJSON           string
			PublicJSON           *string
			SourceMessageIDsJSON *string
		}
		// One indexed join resolves the ordered ID collection. LEFT JOIN exposes
		// missing bodies instead of silently shortening a damaged context.
		if err := tx.Raw(`SELECT m.id, m.run_id, m.input_id, m.round_id, m.kind,
 m.native_json, m.public_json, m.source_message_ids_json
 FROM json_each(?) AS ids
 LEFT JOIN conversation_messages AS m ON m.id = ids.value AND m.thread_id = ?
 ORDER BY CAST(ids.key AS INTEGER)`, version.MessageIDsJSON, threadID).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return sessionstate.ErrInvalidState
		}
		for i, row := range rows {
			if row.ID == "" || row.ID != ids[i] {
				return fmt.Errorf("%w: missing context message %q", sessionstate.ErrInvalidState, ids[i])
			}
			message := sessionstate.Message{ID: row.ID, RunID: row.RunID, InputID: row.InputID, RoundID: row.RoundID, Kind: row.Kind}
			message.Native, err = sessionstate.DecodeMessage([]byte(row.NativeJSON))
			if err != nil {
				return err
			}
			message.Native.PersistentID = message.ID
			if row.PublicJSON != nil {
				message.Public = json.RawMessage(*row.PublicJSON)
			}
			if row.SourceMessageIDsJSON != nil {
				if err := json.Unmarshal([]byte(*row.SourceMessageIDsJSON), &message.SourceMessageIDs); err != nil {
					return err
				}
			}
			snapshot.Messages = append(snapshot.Messages, message)
		}
		return nil
	})
	return snapshot, err
}

func (w *sessionWrite) insertMessages(messages []sessionstate.Message) error {
	for _, message := range messages {
		if message.ID == "" || message.RoundID == "" {
			return fmt.Errorf("%w: message identity is required", sessionstate.ErrInvalidState)
		}
		raw, err := sessionstate.EncodeMessage(message.Native)
		if err != nil {
			return err
		}
		var public, sources any
		if len(message.Public) > 0 {
			if !json.Valid(message.Public) {
				return sessionstate.ErrInvalidState
			}
			public = string(message.Public)
		}
		if message.SourceMessageIDs != nil {
			encoded, err := json.Marshal(message.SourceMessageIDs)
			if err != nil {
				return err
			}
			sources = string(encoded)
		}
		// No upsert: reusing a message ID with different content is a bug.
		if err := w.tx.Table("conversation_messages").Create(map[string]any{
			"id": message.ID, "thread_id": w.head.ID, "run_id": nullableID(message.RunID), "input_id": nullableID(message.InputID),
			"round_id": message.RoundID, "created_seq": w.seq, "role": string(message.Native.Role), "kind": message.Kind,
			"native_json": string(raw), "public_json": public, "source_message_ids_json": sources, "created_at": w.now,
		}).Error; err != nil {
			return err
		}
		if len(message.Public) > 0 {
			if err := w.recordRoundMessage(message.RoundID); err != nil {
				return err
			}
		}
	}
	return nil
}

func nullableID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (w *sessionWrite) replaceContext(update sessionstate.ContextUpdate) error {
	if update.ExpectedRevision != w.head.ContextRevision {
		return sessionstate.ErrVersionConflict
	}
	if update.MessageIDs == nil {
		return fmt.Errorf("%w: ordered message IDs must be explicit", sessionstate.ErrInvalidState)
	}
	switch update.Reason {
	case "append", "compact", "replace":
	default:
		return sessionstate.ErrInvalidState
	}
	seen := make(map[string]bool, len(update.MessageIDs))
	for _, id := range update.MessageIDs {
		if id == "" || seen[id] {
			return sessionstate.ErrInvalidState
		}
		seen[id] = true
	}
	for _, message := range update.NewMessages {
		if !seen[message.ID] {
			return fmt.Errorf("%w: unreferenced context message", sessionstate.ErrInvalidState)
		}
	}
	var previous struct{ MessageIDsJSON string }
	if err := w.tx.Table("thread_context_versions").Select("message_ids_json").Where("thread_id = ? AND revision = ?", w.head.ID, update.ExpectedRevision).Take(&previous).Error; err != nil {
		return err
	}
	var before []string
	if err := json.Unmarshal([]byte(previous.MessageIDsJSON), &before); err != nil {
		return err
	}
	if update.Reason == "append" && (len(update.MessageIDs) < len(before) || !slices.Equal(before, update.MessageIDs[:len(before)])) {
		return sessionstate.ErrVersionConflict
	}
	if slices.Equal(before, update.MessageIDs) {
		if len(update.NewMessages) != 0 {
			return sessionstate.ErrInvalidState
		}
		return nil
	}
	if err := w.insertMessages(update.NewMessages); err != nil {
		return err
	}
	raw, err := json.Marshal(update.MessageIDs)
	if err != nil {
		return err
	}
	var missing int64
	if err := w.tx.Raw(`SELECT count(*) FROM json_each(?) AS ids
 LEFT JOIN conversation_messages AS m ON m.id = ids.value AND m.thread_id = ?
 WHERE m.id IS NULL`, string(raw), w.head.ID).Scan(&missing).Error; err != nil {
		return err
	}
	if missing != 0 {
		return fmt.Errorf("%w: context references missing messages", sessionstate.ErrInvalidState)
	}
	revision := update.ExpectedRevision + 1
	if err := w.tx.Table("thread_context_versions").Create(map[string]any{"thread_id": w.head.ID, "revision": revision, "message_ids_json": string(raw), "created_seq": w.seq, "created_at": w.now, "reason": update.Reason}).Error; err != nil {
		return err
	}
	result := w.tx.Table("threads").Where("id = ? AND context_revision = ?", w.head.ID, update.ExpectedRevision).Update("context_revision", revision)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return sessionstate.ErrVersionConflict
	}
	w.head.ContextRevision = revision
	value, err := json.Marshal(map[string]any{"context_revision": fmt.Sprint(revision)})
	if err != nil {
		return err
	}
	w.change("context_window", sessionstate.Change{Type: "context_window.invalidate", Entity: value})
	return w.publishThread()
}

// LatestContextRun follows the current thread pointer, not historical acceptance
// events. A scene switch restores this pointer together with its context rows.
func (s *Store) LatestContextRun(ctx context.Context, threadID string) (*model.Run, error) {
	var result *model.Run
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		var thread struct {
			ActiveRunID      *string
			LastContextRunID *string
		}
		if err := tx.Table("threads").Select("active_run_id,last_context_run_id").Where("id = ?", threadID).Take(&thread).Error; err != nil {
			return mapErr(err)
		}
		id := textOrEmpty(thread.ActiveRunID)
		if id == "" {
			id = textOrEmpty(thread.LastContextRunID)
		}
		if id == "" {
			return nil
		}
		var row runPO
		if err := tx.Where("id = ? AND thread_id = ?", id, threadID).Take(&row).Error; err != nil {
			return mapErr(err)
		}
		value := row.toModel()
		result = &value
		return nil
	})
	return result, err
}
