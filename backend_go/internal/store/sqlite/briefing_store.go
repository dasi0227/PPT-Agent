package sqlite

import (
	"context"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

type briefingVersionPO struct {
	BriefingID string
	VersionNo  int
	ThreadID   string
	ProjectID  string
	Title      string
	Content    string
	Feedback   string
	CreatedAt  int64
}

func (briefingVersionPO) TableName() string { return "briefing_versions" }

func (row briefingVersionPO) toModel() model.BriefingVersion {
	return model.BriefingVersion{BriefingID: row.BriefingID, VersionNo: row.VersionNo,
		ThreadID: row.ThreadID, ProjectID: row.ProjectID, Kind: model.BriefingHandoff,
		Title: row.Title, Content: row.Content, Feedback: row.Feedback, CreatedAt: row.CreatedAt}
}

// Revision prompts need only the latest complete document, not every version
// of every briefing in the conversation.
func (s *Store) LatestBriefingVersion(ctx context.Context, threadID, briefingID string) (model.BriefingVersion, error) {
	var row briefingVersionPO
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		return mapErr(tx.Where("thread_id = ? AND briefing_id = ?", threadID, briefingID).Order("version_no DESC").Take(&row).Error)
	})
	return row.toModel(), err
}

// The candidate and command completion share the caller's fenced transaction.
func (w *sessionWrite) appendBriefing(version model.BriefingVersion) error {
	if version.ThreadID != w.head.ID || version.ProjectID != w.head.ProjectID ||
		version.BriefingID == "" || version.Kind != model.BriefingHandoff || version.VersionNo < 1 || version.CreatedAt <= 0 {
		return sessionstate.ErrInvalidState
	}
	var previous briefingVersionPO
	err := w.tx.Where("briefing_id = ?", version.BriefingID).Order("version_no DESC").Take(&previous).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if version.VersionNo != 1 {
			return sessionstate.ErrVersionConflict
		}
	case err != nil:
		return err
	default:
		if previous.ThreadID != w.head.ID || previous.ProjectID != w.head.ProjectID || previous.VersionNo+1 != version.VersionNo {
			return sessionstate.ErrVersionConflict
		}
	}
	return w.tx.Create(&briefingVersionPO{BriefingID: version.BriefingID, VersionNo: version.VersionNo,
		ThreadID: version.ThreadID, ProjectID: version.ProjectID, Title: version.Title,
		Content: version.Content, Feedback: version.Feedback, CreatedAt: version.CreatedAt}).Error
}
