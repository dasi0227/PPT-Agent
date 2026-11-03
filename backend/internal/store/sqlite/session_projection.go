package sqlite

import (
	"encoding/json"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

func readThreadView(tx *gorm.DB, threadID string) (sessionstate.ThreadView, error) {
	var row threadPO
	if err := tx.Where("id = ?", threadID).Take(&row).Error; err != nil {
		return sessionstate.ThreadView{}, mapErr(err)
	}
	view := sessionstate.ThreadView{
		ID: row.ID, ProjectID: row.ProjectID, Title: row.Title, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		AutoSuggestEnabled: row.AutoSuggestEnabled != 0, AutoRenameEnabled: row.AutoRenameEnabled != 0, NamingRevision: row.NamingRevision,
		EntityRevision: row.EntityRevision, UpdatedSeq: row.UpdatedSeq, PlanRevision: row.PlanRevision, ActiveRunID: row.ActiveRunID,
		PlanRunID: row.PlanRunID, ContextRevision: row.ContextRevision,
	}
	if row.PlanJSON != nil {
		if err := json.Unmarshal([]byte(*row.PlanJSON), &view.Plan); err != nil {
			return view, err
		}
	}
	if row.NextInputSuggestionsJSON != nil {
		view.Suggestions = json.RawMessage(*row.NextInputSuggestionsJSON)
	}
	return view, nil
}

func (w *sessionWrite) publishThread() error {
	if err := w.tx.Table("threads").Where("id = ?", w.head.ID).Updates(map[string]any{"entity_revision": gorm.Expr("entity_revision + 1"), "updated_seq": w.seq, "updated_at": w.now}).Error; err != nil {
		return err
	}
	view, err := readThreadView(w.tx, w.head.ID)
	if err != nil {
		return err
	}
	return w.upsert("thread", w.head.ID, view)
}

type timelineGroupPO struct {
	ID             string `gorm:"primaryKey"`
	ThreadID       string
	OrderSeq       int64
	Kind           string
	RunID          *string
	CommandID      *string
	InputID        *string
	Status         string
	SummaryJSON    string
	ItemCount      int
	EntityRevision int64
	CreatedSeq     int64
	UpdatedSeq     int64
}

func (timelineGroupPO) TableName() string { return "timeline_groups" }

func textOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (p timelineGroupPO) view() sessionstate.TimelineGroup {
	return sessionstate.TimelineGroup{ID: p.ID, ThreadID: p.ThreadID, OrderSeq: p.OrderSeq, Kind: p.Kind, RunID: textOrEmpty(p.RunID), CommandID: textOrEmpty(p.CommandID), InputID: textOrEmpty(p.InputID), Status: p.Status, Summary: json.RawMessage(p.SummaryJSON), ItemCount: p.ItemCount, EntityRevision: p.EntityRevision, UpdatedSeq: p.UpdatedSeq}
}

func (w *sessionWrite) putGroup(group sessionstate.TimelineGroup) error {
	if group.ID == "" || group.ThreadID != w.head.ID || !json.Valid(group.Summary) {
		return sessionstate.ErrInvalidState
	}
	var row timelineGroupPO
	err := w.tx.Where("id = ?", group.ID).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		if row.ThreadID != w.head.ID || row.Kind != group.Kind {
			return sessionstate.ErrInvalidState
		}
		row.Status, row.SummaryJSON, row.UpdatedSeq = group.Status, string(group.Summary), w.seq
		row.EntityRevision++
		if err := w.tx.Save(&row).Error; err != nil {
			return err
		}
	} else {
		row = timelineGroupPO{ID: group.ID, ThreadID: w.head.ID, OrderSeq: w.seq, Kind: group.Kind, RunID: stringPointer(group.RunID), CommandID: stringPointer(group.CommandID), InputID: stringPointer(group.InputID), Status: group.Status, SummaryJSON: string(group.Summary), EntityRevision: 1, CreatedSeq: w.seq, UpdatedSeq: w.seq}
		if err := w.tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return w.upsert("timeline.group", row.ID, row.view())
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

type timelineItemPO struct {
	ID             string `gorm:"primaryKey"`
	ThreadID       string
	GroupID        string
	Position       int64
	Kind           string
	EntityRevision int64
	MessageID      *string
	ToolCallID     *string
	InteractionID  *string
	AttemptID      *string
	SummaryJSON    string
	BodyRef        *string
	DetailRef      *string
	CreatedSeq     int64
	UpdatedSeq     int64
	CreatedAt      int64
}

func (timelineItemPO) TableName() string { return "timeline_items" }

func (p timelineItemPO) view() sessionstate.TimelineItem {
	return sessionstate.TimelineItem{CreatedSeq: p.CreatedSeq, ID: p.ID, ThreadID: p.ThreadID, GroupID: p.GroupID, Position: p.Position, Kind: p.Kind, EntityRevision: p.EntityRevision, UpdatedSeq: p.UpdatedSeq, CreatedAt: p.CreatedAt, MessageID: textOrEmpty(p.MessageID), ToolCallID: textOrEmpty(p.ToolCallID), InteractionID: textOrEmpty(p.InteractionID), AttemptID: textOrEmpty(p.AttemptID), Summary: json.RawMessage(p.SummaryJSON), BodyRef: textOrEmpty(p.BodyRef), DetailRef: textOrEmpty(p.DetailRef)}
}

func (w *sessionWrite) putItem(item sessionstate.TimelineItem) error {
	if item.ID == "" || item.GroupID == "" || item.ThreadID != w.head.ID || !json.Valid(item.Summary) {
		return sessionstate.ErrInvalidState
	}
	var group timelineGroupPO
	if err := w.tx.Where("id = ? AND thread_id = ?", item.GroupID, w.head.ID).Take(&group).Error; err != nil {
		return err
	}
	var row timelineItemPO
	err := w.tx.Where("id = ?", item.ID).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	created := errors.Is(err, gorm.ErrRecordNotFound)
	if created {
		var position int64
		if err := w.tx.Table("timeline_items").Select("COALESCE(MAX(position), 0) + 1").Where("group_id = ?", group.ID).Scan(&position).Error; err != nil {
			return err
		}
		row = timelineItemPO{ID: item.ID, ThreadID: w.head.ID, GroupID: group.ID, Position: position, EntityRevision: 0, CreatedSeq: w.seq, CreatedAt: w.now}
	} else if row.ThreadID != w.head.ID || row.GroupID != group.ID {
		return sessionstate.ErrInvalidState
	}
	row.Kind, row.SummaryJSON, row.UpdatedSeq = item.Kind, string(item.Summary), w.seq
	row.MessageID, row.ToolCallID, row.InteractionID, row.AttemptID = stringPointer(item.MessageID), stringPointer(item.ToolCallID), stringPointer(item.InteractionID), stringPointer(item.AttemptID)
	row.BodyRef, row.DetailRef = stringPointer(item.BodyRef), stringPointer(item.DetailRef)
	row.EntityRevision++
	if created {
		err = w.tx.Create(&row).Error
	} else {
		err = w.tx.Save(&row).Error
	}
	if err != nil {
		return err
	}
	if created {
		group.ItemCount++
		group.EntityRevision++
		group.UpdatedSeq = w.seq
		if err := w.tx.Save(&group).Error; err != nil {
			return err
		}
		if err := w.upsert("timeline.group", group.ID, group.view()); err != nil {
			return err
		}
	}
	return w.upsert("timeline.item", row.ID, row.view())
}
