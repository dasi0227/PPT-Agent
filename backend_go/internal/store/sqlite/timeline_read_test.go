package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
)

func TestTimelinePaginationPreservesOversizedRecords(t *testing.T) {
	s := terminalStoreForTest(t)
	ctx := sessionstate.WithRequestScene(context.Background(), 1)
	large, err := json.Marshal(map[string]string{"text": strings.Repeat("完整正文", 50_000)})
	if err != nil {
		t.Fatal(err)
	}
	// Descending reads encounter consecutive oversized records, an ordinary
	// two-record page, then an oversized last record with no continuation.
	bodies := []json.RawMessage{large, json.RawMessage(`{"text":"older"}`), json.RawMessage(`{"text":"newer"}`), large, large}
	for i, body := range bodies {
		groupID := fmt.Sprintf("g%d", i)
		_, err := s.commitSession(ctx, sessionstate.Fence{ProjectID: "p", ThreadID: "t", SceneRevision: 1}, sessionstate.CommitIdentity{Key: groupID, RequestHash: groupID}, func(w *sessionWrite) error {
			if err := w.putGroup(sessionstate.TimelineGroup{ID: groupID, ThreadID: "t", Kind: "run", Status: "done", Summary: body}); err != nil {
				return err
			}
			if i == len(bodies)-1 {
				for j, content := range bodies {
					if err := w.putPublicItem(sessionstate.TimelineItem{ID: fmt.Sprintf("item%d", j), ThreadID: "t", GroupID: groupID, Kind: "final"}, content); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	nextCursor := func(t *testing.T, encoded *string, before *sessionstate.PageCursor) *sessionstate.PageCursor {
		t.Helper()
		if encoded == nil || before != nil && *encoded == before.Encode() {
			t.Fatal("nonterminal page did not advance its cursor")
		}
		cursor, err := sessionstate.DecodePageCursor(*encoded)
		if err != nil {
			t.Fatal(err)
		}
		return &cursor
	}
	t.Run("session view and older groups", func(t *testing.T) {
		view, err := s.SessionView(ctx, "t", 2)
		if err != nil {
			t.Fatal(err)
		}
		page := view.Timeline
		var before *sessionstate.PageCursor
		remaining := len(bodies)
		for {
			if len(page.Groups) == 0 || len(page.Groups) > 2 || len(page.Groups) > remaining || page.IsEmpty {
				t.Fatalf("invalid group page: count=%d remaining=%d empty=%v", len(page.Groups), remaining, page.IsEmpty)
			}
			for i, group := range page.Groups {
				index := remaining - len(page.Groups) + i
				if group.ID != fmt.Sprintf("g%d", index) || !bytes.Equal(group.Summary, bodies[index]) {
					t.Fatal("group skipped, repeated, reordered or truncated")
				}
			}
			remaining -= len(page.Groups)
			if !page.HasMore {
				if remaining != 0 || page.OlderCursor != nil {
					t.Fatal("group pagination ended before all records were read")
				}
				break
			}
			before = nextCursor(t, page.OlderCursor, before)
			page, err = s.Timeline(ctx, "t", before, 2)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("complete reply bodies", func(t *testing.T) {
		var before *sessionstate.PageCursor
		remaining := len(bodies)
		for {
			page, err := s.TimelineItems(ctx, "t", "g4", before, 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Items) == 0 || len(page.Items) > 2 || len(page.Items) > remaining {
				t.Fatalf("invalid item page: count=%d remaining=%d", len(page.Items), remaining)
			}
			for i, item := range page.Items {
				index := remaining - len(page.Items) + i
				if item.ID != fmt.Sprintf("item%d", index) || !bytes.Equal(item.Summary, bodies[index]) {
					t.Fatal("reply skipped, repeated, reordered or truncated")
				}
			}
			remaining -= len(page.Items)
			if !page.HasMore {
				if remaining != 0 || page.OlderCursor != nil {
					t.Fatal("item pagination ended before all records were read")
				}
				break
			}
			before = nextCursor(t, page.OlderCursor, before)
		}
	})
}
