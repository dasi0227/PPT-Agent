package sqlite

import (
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"gorm.io/gorm"
)

type conversationRoundPO struct {
	ID                 string `gorm:"primaryKey"`
	ThreadID           string
	RunID              string
	OpenedSeq          int64
	ClosedSeq          *int64
	LastMessageSeq     int64
	PublicMessageCount int64
	HasTaskInput       int
}

func (conversationRoundPO) TableName() string { return "conversation_rounds" }

// A round is an explicit conversation boundary, not a reconstruction of Run events.
func (w *sessionWrite) openConversationRound(runID string) (string, error) {
	var row conversationRoundPO
	err := w.tx.Where("thread_id = ? AND run_id = ? AND closed_seq IS NULL", w.head.ID, runID).Take(&row).Error
	if err == nil {
		return row.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	row = conversationRoundPO{ID: model.MustShortID("round"), ThreadID: w.head.ID, RunID: runID, OpenedSeq: w.seq, LastMessageSeq: w.seq}
	return row.ID, w.tx.Create(&row).Error
}

func (w *sessionWrite) recordRoundMessage(roundID string) error {
	result := w.tx.Model(&conversationRoundPO{}).Where("id = ? AND thread_id = ?", roundID, w.head.ID).
		Updates(map[string]any{"last_message_seq": w.seq, "public_message_count": gorm.Expr("public_message_count + 1")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return model.ErrRunNotFound
	}
	return nil
}

func (w *sessionWrite) closeConversationRound(runID string) error {
	return w.tx.Model(&conversationRoundPO{}).Where("thread_id = ? AND run_id = ? AND closed_seq IS NULL", w.head.ID, runID).
		Update("closed_seq", w.seq).Error
}
