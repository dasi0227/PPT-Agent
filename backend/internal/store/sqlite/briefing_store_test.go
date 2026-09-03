package sqlite

import (
	"context"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestBriefingStoreKeepsFullGroupsAndLimitsContextVersions(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	if err := st.CreateProject(ctx, model.Project{
		ID: "p1", Title: "Deck", WorkDir: t.TempDir(), Theme: "default",
		Status: "ready", LayoutVersion: currentProjectLayoutVersion, CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{
		ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl",
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	for versionNo := 1; versionNo <= 3; versionNo++ {
		if err := st.AppendBriefingVersion(ctx, model.BriefingVersion{
			BriefingID: "b1", ThreadID: "t1", ProjectID: "p1",
			Kind: model.BriefingKickoff, VersionNo: versionNo,
			Content: "content", Feedback: "feedback", CreatedAt: int64(versionNo),
		}); err != nil {
			t.Fatal(err)
		}
	}

	briefings, err := st.ListThreadBriefings(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(briefings) != 1 || len(briefings[0].Versions) != 3 || briefings[0].UpdatedAt != 3 {
		t.Fatalf("unexpected briefing groups: %+v", briefings)
	}
	recent, err := st.GetBriefingVersions(ctx, "b1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].VersionNo != 2 || recent[1].VersionNo != 3 {
		t.Fatalf("unexpected recent versions: %+v", recent)
	}
}
