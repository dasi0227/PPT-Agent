package sqlite

import (
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

// Read one assessment without expanding any other timeline item or changing its
// immutable public detail. File validity is checked outside this transaction.
func (s *Store) TimelinePreview(ctx context.Context, threadID, itemID string) (*model.PreviewAssessment, error) {
	var assessment *model.PreviewAssessment
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); !ok || scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		var item timelineItemPO
		if err := tx.Where("id = ? AND thread_id = ?", itemID, threadID).Take(&item).Error; err != nil {
			return mapErr(err)
		}
		if item.Kind != "tool" {
			return nil
		}
		public := item.SummaryJSON
		if item.DetailRef != nil {
			ref, err := sessionstate.DecodeContentRef(*item.DetailRef)
			if err != nil {
				return err
			}
			if ref.ThreadID != threadID || ref.ItemID != itemID || ref.SceneRevision != head.SceneRevision || ref.ItemRevision != item.EntityRevision || ref.SourceType != "detail" {
				return sessionstate.ErrContentExpired
			}
			var detail struct{ PublicJSON string }
			if err := tx.Table("timeline_details").Where("id = ? AND thread_id = ?", ref.SourceID, threadID).Take(&detail).Error; err != nil {
				return mapErr(err)
			}
			public = detail.PublicJSON
		}
		var value struct {
			Assessment *model.PreviewAssessment `json:"preview_assessment"`
		}
		if err := json.Unmarshal([]byte(public), &value); err != nil {
			return err
		}
		assessment = value.Assessment
		return nil
	})
	return assessment, err
}
