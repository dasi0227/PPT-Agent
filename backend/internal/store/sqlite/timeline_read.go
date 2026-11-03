package sqlite

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"gorm.io/gorm"
)

func pageLimit(value, fallback, maximum int) int {
	if value < 1 {
		return fallback
	}
	return min(value, maximum)
}

func checkPageCursor(head sessionHead, cursor *sessionstate.PageCursor, groupID string) error {
	if cursor == nil {
		return nil
	}
	if cursor.ProjectID != head.ProjectID || cursor.ThreadID != head.ID || cursor.SceneRevision != head.SceneRevision {
		return sessionstate.ErrSceneChanged
	}
	if cursor.GroupID != groupID {
		return sessionstate.ErrCursorReset
	}
	return nil
}

func readGroupItems(tx *gorm.DB, head sessionHead, groupID string, before *sessionstate.PageCursor, limit int) (sessionstate.ItemPage, error) {
	page := sessionstate.ItemPage{SceneRevision: head.SceneRevision, ReadSeq: head.LastSeq, GroupID: groupID, Items: []sessionstate.TimelineItem{}}
	if err := checkPageCursor(head, before, groupID); err != nil {
		return page, err
	}
	query := tx.Where("thread_id = ? AND group_id = ?", head.ID, groupID)
	if before != nil {
		query = query.Where("position < ? OR (position = ? AND id < ?)", before.Position, before.Position, before.ID)
	}
	var rows []timelineItemPO
	if err := query.Order("position DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return page, err
	}
	if len(rows) > limit {
		page.HasMore = true
		rows = rows[:limit]
	}
	remaining := 512*1024 - 4096
	keep := 0
	for _, row := range rows {
		raw, err := json.Marshal(row.view())
		if err != nil {
			return page, err
		}
		if len(raw)+1 > remaining {
			page.HasMore = true
			break
		}
		remaining -= len(raw) + 1
		keep++
	}
	rows = rows[:keep]
	if page.HasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := sessionstate.PageCursor{ProjectID: head.ProjectID, ThreadID: head.ID, SceneRevision: head.SceneRevision, GroupID: groupID, Position: last.Position, ID: last.ID}.Encode()
		page.OlderCursor = &cursor
	}
	slices.Reverse(rows)
	for _, row := range rows {
		page.Items = append(page.Items, row.view())
	}
	return page, nil
}

func readTimeline(tx *gorm.DB, head sessionHead, before *sessionstate.PageCursor, limit int) (sessionstate.TimelinePage, error) {
	page := sessionstate.TimelinePage{SceneRevision: head.SceneRevision, ReadSeq: head.LastSeq, Groups: []sessionstate.TimelineGroup{}, Items: []sessionstate.TimelineItem{}}
	if err := checkPageCursor(head, before, ""); err != nil {
		return page, err
	}
	query := tx.Where("thread_id = ?", head.ID)
	if before != nil {
		query = query.Where("order_seq < ? OR (order_seq = ? AND id < ?)", before.Position, before.Position, before.ID)
	}
	var rows []timelineGroupPO
	if err := query.Order("order_seq DESC,id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return page, err
	}
	if len(rows) > limit {
		page.HasMore = true
		rows = rows[:limit]
	}
	// Reserve framing/cursors, then summaries before assigning any process rows.
	remainingBytes, remainingItems := 512*1024-8192, 200
	selectedGroups := rows[:0]
	for _, row := range rows {
		raw, err := json.Marshal(row.view())
		if err != nil {
			return page, err
		}
		cost := len(raw) + 512
		if cost > remainingBytes {
			page.HasMore = true
			break
		}
		remainingBytes -= cost
		selectedGroups = append(selectedGroups, row)
	}
	rows = selectedGroups
	if page.HasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := (sessionstate.PageCursor{ProjectID: head.ProjectID, ThreadID: head.ID, SceneRevision: head.SceneRevision, Position: last.OrderSeq, ID: last.ID}).Encode()
		page.OlderCursor = &cursor
	}
	// Newest groups get the finite inline budget. Each group has an independent
	// process-page entry point, including summary-only and zero-inline groups.
	for _, group := range rows {
		view := group.view()
		view.ItemsMode = "page"
		completed := group.Kind == "run" && (group.Status == "done" || group.Status == "failed" || group.Status == "canceled")
		if completed {
			view.ItemsMode = "summary"
		}
		items := []timelineItemPO{}
		if remainingItems > 0 && remainingBytes > 2048 {
			count := min(50, remainingItems)
			if completed {
				for _, kind := range []string{"user_turn", "final", "terminal_notice"} {
					var found []timelineItemPO
					if err := tx.Where("group_id = ? AND kind = ?", group.ID, kind).Order("position DESC,id DESC").Limit(count).Find(&found).Error; err != nil {
						return page, err
					}
					items = append(items, found...)
				}
				slices.SortFunc(items, func(a, b timelineItemPO) int {
					if a.Position != b.Position {
						return cmp.Compare(b.Position, a.Position)
					}
					return cmp.Compare(b.ID, a.ID)
				})
				if len(items) > count {
					items = items[:count]
				}
			} else if err := tx.Where("group_id = ? AND thread_id = ?", group.ID, head.ID).Order("position DESC,id DESC").Limit(count).Find(&items).Error; err != nil {
				return page, err
			}
		}
		selected := []sessionstate.TimelineItem{}
		for _, row := range items {
			item := row.view()
			raw, err := json.Marshal(item)
			if err != nil {
				return page, err
			}
			if len(raw)+1 > remainingBytes || remainingItems == 0 {
				break
			}
			remainingBytes -= len(raw) + 1
			remainingItems--
			selected = append(selected, item)
		}
		view.HasMoreItems = group.ItemCount > len(selected)
		if view.HasMoreItems && len(selected) == 0 {
			// No page was read for this group within the shared inline budget.
			// Expansion must begin at its latest page, without an older cursor.
			view.ItemsMode = "summary"
		}
		if !completed && view.HasMoreItems && len(selected) > 0 {
			last := selected[len(selected)-1]
			cursor := (sessionstate.PageCursor{ProjectID: head.ProjectID, ThreadID: head.ID, SceneRevision: head.SceneRevision, GroupID: group.ID, Position: last.Position, ID: last.ID}).Encode()
			view.OlderItemCursor = &cursor
		}
		slices.Reverse(selected)
		page.Groups = append(page.Groups, view)
		page.Items = append(page.Items, selected...)
	}
	slices.Reverse(page.Groups)
	// Cross-group ordering follows group order; positions are group-local.
	order := map[string]int{}
	for i, group := range page.Groups {
		order[group.ID] = i
	}
	slices.SortStableFunc(page.Items, func(a, b sessionstate.TimelineItem) int {
		if a.GroupID != b.GroupID {
			return cmp.Compare(order[a.GroupID], order[b.GroupID])
		}
		return cmp.Compare(a.Position, b.Position)
	})
	var nonempty int
	if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM timeline_groups WHERE thread_id = ?)
 OR EXISTS(SELECT 1 FROM runs WHERE thread_id = ? AND status NOT IN ('done','failed','canceled'))
 OR EXISTS(SELECT 1 FROM command_executions WHERE thread_id = ? AND kind NOT IN ('rename','suggest') AND status IN ('accepted','running','cancel_requested'))`, head.ID, head.ID, head.ID).Scan(&nonempty).Error; err != nil {
		return page, err
	}
	page.IsEmpty = nonempty == 0
	return page, nil
}

func (s *Store) Timeline(ctx context.Context, threadID string, before *sessionstate.PageCursor, limit int) (sessionstate.TimelinePage, error) {
	var page sessionstate.TimelinePage
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		page, err = readTimeline(tx, head, before, pageLimit(limit, 20, 50))
		return err
	})
	return page, err
}

func (s *Store) TimelineItems(ctx context.Context, threadID, groupID string, before *sessionstate.PageCursor, limit int) (sessionstate.ItemPage, error) {
	var page sessionstate.ItemPage
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		if scene, ok := sessionstate.RequestScene(ctx); ok && scene != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		var group timelineGroupPO
		if err := tx.Select("id").Where("id = ? AND thread_id = ?", groupID, threadID).Take(&group).Error; err != nil {
			return mapErr(err)
		}
		page, err = readGroupItems(tx, head, groupID, before, pageLimit(limit, 50, 100))
		return err
	})
	return page, err
}

func (s *Store) TimelineContent(ctx context.Context, threadID, itemID string, ref sessionstate.ContentRef) (json.RawMessage, error) {
	var content json.RawMessage
	err := s.reader.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := readSessionHead(tx, threadID)
		if err != nil {
			return err
		}
		if ref.ThreadID != threadID || ref.ItemID != itemID || ref.SceneRevision != head.SceneRevision {
			return sessionstate.ErrSceneChanged
		}
		var item timelineItemPO
		if err := tx.Where("id = ? AND thread_id = ?", itemID, threadID).Take(&item).Error; err != nil {
			return mapErr(err)
		}
		encoded := ref.Encode()
		if encoded != textOrEmpty(item.BodyRef) && encoded != textOrEmpty(item.DetailRef) {
			return sessionstate.ErrContentExpired
		}
		var row struct{ PublicJSON *string }
		switch ref.SourceType {
		case "message":
			err = tx.Table("conversation_messages").Select("public_json").Where("id = ? AND thread_id = ?", ref.SourceID, threadID).Take(&row).Error
		case "detail":
			err = tx.Table("timeline_details").Select("public_json").Where("id = ? AND thread_id = ?", ref.SourceID, threadID).Take(&row).Error
		default:
			return sessionstate.ErrContentExpired
		}
		if err != nil {
			return mapErr(err)
		}
		if row.PublicJSON == nil {
			return sessionstate.ErrContentExpired
		}
		content = json.RawMessage(*row.PublicJSON)
		return nil
	})
	return content, err
}
