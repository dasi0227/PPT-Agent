package sqlite

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
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
	RequestJSON       string
	ResultJSON        *string
	ErrorJSON         *string
	PreviousTitle     string
	RunID             *string
	ModelJSON         *string
	OperationJSON     *string
	EntityRevision    int64
	UpdatedSeq        int64
	CreatedAt         int64
	UpdatedAt         int64
}

func (commandExecutionPO) TableName() string { return "command_executions" }
func activeCommandStatus(status string) bool {
	return status == "accepted" || status == "running" || status == "cancel_requested"
}

func (row commandExecutionPO) toModel() (model.CommandExecution, error) {
	var out model.CommandExecution
	if err := json.Unmarshal([]byte(row.RequestJSON), &out); err != nil {
		return out, err
	}
	out.CommandID, out.AttemptID, out.AttemptNo = row.CommandID, row.ID, row.AttemptNo
	out.ThreadID, out.ProjectID, out.Kind, out.Source = row.ThreadID, row.ProjectID, row.Kind, row.Source
	out.Status, out.OwnerInstanceID = row.Status, row.OwnerInstanceID
	out.ExecutionRevision, out.SceneRevision = row.ExecutionRevision, row.SceneRevision
	out.EntityRevision, out.UpdatedSeq = row.EntityRevision, row.UpdatedSeq
	out.PreviousTitle, out.RunID = row.PreviousTitle, textOrEmpty(row.RunID)
	out.CreatedAt, out.UpdatedAt = row.CreatedAt, row.UpdatedAt
	var err error
	out.Phase, err = strconv.Atoi(row.Phase)
	if err != nil {
		return out, err
	}
	if row.ResultJSON != nil {
		out.Result = json.RawMessage(*row.ResultJSON)
	}
	if row.ErrorJSON != nil {
		out.Error = json.RawMessage(*row.ErrorJSON)
	}
	if row.OperationJSON != nil {
		out.Operation = json.RawMessage(*row.OperationJSON)
	}
	return out, nil
}

func (s *Store) interruptCommands(ctx context.Context) error {
	var rows []commandExecutionPO
	if err := s.db.WithContext(ctx).Where(settledHistoryProjects).Where("status IN ? AND kind != ?", []string{"accepted", "running", "cancel_requested"}, "commit").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		fence := sessionstate.Fence{ProjectID: row.ProjectID, ThreadID: row.ThreadID, SceneRevision: row.SceneRevision}
		_, err := s.commitSession(ctx, fence, sessionstate.CommitIdentity{Key: "interrupt:" + row.ID + ":" + strconv.FormatInt(row.ExecutionRevision, 10), RequestHash: "process-interrupted"}, func(w *sessionWrite) error {
			var current commandExecutionPO
			if err := w.tx.Where("id = ?", row.ID).Take(&current).Error; err != nil {
				return err
			}
			if !activeCommandStatus(current.Status) {
				return nil
			}
			execution, err := current.toModel()
			if err != nil {
				return err
			}
			execution.Status = "interrupted"
			execution.Error = json.RawMessage(`{"code":"COMMAND_INTERRUPTED","retryable":true}`)
			execution.OwnerInstanceID = ""
			execution.ExecutionRevision++
			return w.writeCommand(current, execution)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
