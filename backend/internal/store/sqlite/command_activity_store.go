package sqlite

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type commandActivityPO struct {
	ID            string `gorm:"primaryKey"`
	AttemptID     string
	ThreadID      string
	ProjectID     string
	Kind          string
	Method        string
	Status        string
	Phase         int
	PreviousTitle string
	Request       string
	Result        string
	CreatedAt     int64
	UpdatedAt     int64
}

func (commandActivityPO) TableName() string { return "command_activities" }
func commandActivityToPO(a model.CommandActivity) commandActivityPO {
	result := string(a.Result)
	if result == "" {
		result = "null"
	}
	return commandActivityPO{a.ID, a.AttemptID, a.ThreadID, a.ProjectID, a.Kind, a.Method, a.Status, a.Phase, a.PreviousTitle, string(a.Request), result, a.CreatedAt, a.UpdatedAt}
}
func (p commandActivityPO) toModel() model.CommandActivity {
	return model.CommandActivity{ID: p.ID, AttemptID: p.AttemptID, ThreadID: p.ThreadID, ProjectID: p.ProjectID, Kind: p.Kind, Method: p.Method, Status: p.Status, Phase: p.Phase, PreviousTitle: p.PreviousTitle, Request: json.RawMessage(p.Request), Result: json.RawMessage(p.Result), CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func (s *Store) BeginCommandActivity(ctx context.Context, activity model.CommandActivity) (model.CommandActivity, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous commandActivityPO
		err := tx.First(&previous, "id = ?", activity.ID).Error
		if err == nil {
			if previous.ThreadID != activity.ThreadID || previous.ProjectID != activity.ProjectID || previous.Kind != activity.Kind || previous.Status == "loading" {
				return store.ErrCommandActivityConflict
			}
			activity.CreatedAt = previous.CreatedAt
			activity.Result = json.RawMessage(previous.Result)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row := commandActivityToPO(activity)
		return tx.Save(&row).Error
	})
	return activity, err
}

func (s *Store) SaveCommandActivity(ctx context.Context, activity model.CommandActivity) error {
	row := commandActivityToPO(activity)
	// A canceled authoring transaction may have rolled back the initial insert.
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "phase", "result", "updated_at"}),
		Where:     clause.Where{Exprs: []clause.Expression{clause.Eq{Column: clause.Column{Table: "command_activities", Name: "attempt_id"}, Value: activity.AttemptID}}},
	}).Create(&row).Error
}

func (s *Store) ListThreadCommandActivities(ctx context.Context, threadID string) ([]model.CommandActivity, error) {
	var rows []commandActivityPO
	if err := s.db.WithContext(ctx).Where("thread_id = ?", threadID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]model.CommandActivity, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toModel())
	}
	return result, nil
}
