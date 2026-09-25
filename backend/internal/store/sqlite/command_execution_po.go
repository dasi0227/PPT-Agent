package sqlite

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"gorm.io/gorm"
)

type commandExecutionPO struct {
	ID                string `gorm:"primaryKey"`
	CommandID         string
	AttemptNo         int
	ThreadID          string
	ProjectID         string
	Kind              string
	Source            string
	Status            string
	Phase             string
	OwnerInstanceID   string
	ExecutionRevision int64
	SceneRevision     int64
	StartEventSeq     int64
	TerminalEventSeq  *int64
	CreatedAt         int64
	UpdatedAt         int64
}

func (commandExecutionPO) TableName() string { return "command_executions" }
func activeCommandStatus(status string) bool {
	return status == "accepted" || status == "running" || status == "cancel_requested"
}
func (s *Store) interruptCommands(ctx context.Context) error {
	var rows []commandExecutionPO
	if err := s.db.WithContext(ctx).Where("status IN ? AND kind != ?", []string{"accepted", "running", "cancel_requested"}, "commit").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			accepted, err := eventInTransaction(tx, row.ThreadID, row.StartEventSeq)
			if err != nil {
				return err
			}
			var execution model.CommandExecution
			if err := json.Unmarshal(accepted.Payload, &execution); err != nil {
				return err
			}
			execution.Status = "interrupted"
			execution.UpdatedAt = time.Now().UnixMilli()
			execution.Error = json.RawMessage(`{"code":"COMMAND_INTERRUPTED","message":"服务已重启，请重试。","retryable":true}`)
			raw, err := json.Marshal(execution)
			if err != nil {
				return err
			}
			event, err := s.enqueueEvent(tx, row.ThreadID, threadjournal.Event{Type: "command.interrupted", CommandID: row.CommandID, AttemptID: row.ID, Payload: raw})
			if err != nil {
				return err
			}
			return tx.Model(&commandExecutionPO{}).Where("id = ?", row.ID).Updates(map[string]any{"status": "interrupted", "terminal_event_seq": event.Seq, "owner_instance_id": "", "updated_at": time.Now().UnixMilli()}).Error
		})
		if err != nil {
			return err
		}
		if err := s.FlushThreadEvents(ctx, row.ThreadID); err != nil {
			return err
		}
	}
	return nil
}
