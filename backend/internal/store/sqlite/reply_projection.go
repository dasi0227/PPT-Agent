package sqlite

import (
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

// Replies keep their complete text and change list inline. Each immutable diff
// is read separately when its target is opened.
func (w *sessionWrite) putReplyItem(item sessionstate.TimelineItem, public json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(public, &fields); err != nil {
		return err
	}
	if raw, ok := fields["affected_targets"]; ok {
		targets := []model.PublicTarget{}
		if err := json.Unmarshal(raw, &targets); err != nil {
			return err
		}
		var revision int64
		if err := w.tx.Table("timeline_items").Select("COALESCE(MAX(entity_revision),0)").Where("id = ? AND thread_id = ?", item.ID, w.head.ID).Scan(&revision).Error; err != nil {
			return err
		}
		for i := range targets {
			target := &targets[i]
			if target.Diff == nil {
				continue
			}
			diff := target.Diff
			content, err := json.Marshal(diff)
			if err != nil {
				return err
			}
			detailID := model.MustShortID("detail")
			if err := w.tx.Table("timeline_details").Create(map[string]any{"id": detailID, "thread_id": w.head.ID, "public_json": string(content), "created_seq": w.seq}).Error; err != nil {
				return err
			}
			target.DiffRef = sessionstate.ContentRef{ThreadID: w.head.ID, ItemID: item.ID, SceneRevision: w.head.SceneRevision, ItemRevision: revision + 1, SourceType: "detail", SourceID: detailID}.Encode()
			target.Diff = &model.ArtifactDiff{Kind: diff.Kind, Status: diff.Status, Filename: diff.Filename}
		}
		var err error
		fields["affected_targets"], err = json.Marshal(targets)
		if err != nil {
			return err
		}
	}
	summary, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	item.Summary, item.BodyRef, item.DetailRef = summary, "", ""
	return w.putItem(item)
}

func (item timelineItemPO) hasChangeDiffRef(encoded string) (bool, error) {
	if item.Kind != "final" && item.Kind != "message" && item.Kind != "terminal_notice" {
		return false, nil
	}
	var summary struct {
		Targets []struct {
			DiffRef string `json:"diff_ref"`
		} `json:"affected_targets"`
	}
	if err := json.Unmarshal([]byte(item.SummaryJSON), &summary); err != nil {
		return false, err
	}
	for _, target := range summary.Targets {
		if target.DiffRef == encoded {
			return true, nil
		}
	}
	return false, nil
}
