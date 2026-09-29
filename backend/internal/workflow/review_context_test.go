package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
)

func TestReviewMaterialUsesRunStartNetChangesAcrossResume(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("sli_1.html", "initial\n")
	write("removed.txt", "removed\n")
	write("unchanged.txt", "same\n")
	if err := ensureReviewBaseline(ctx, root, "run", false); err != nil {
		t.Fatal(err)
	}
	write("sli_1.html", "intermediate\n")
	if err := ensureReviewBaseline(ctx, root, "run", true); err != nil {
		t.Fatal(err)
	}
	write("sli_1.html", "final\n")
	write("added.txt", "added\n")
	if err := os.Remove(filepath.Join(root, "removed.txt")); err != nil {
		t.Fatal(err)
	}
	pack := testPack(model.ModeExecute, model.ScopeAllPages, false, "检查")
	session, err := NewRunSession(root, "run")
	if err != nil {
		t.Fatal(err)
	}
	state := &RunState{runID: "run", projectDir: root, pack: pack, tx: session, reviewInstructions: []ReviewInstruction{{Text: "original"}, {Text: "correction"}}}
	material, _, err := buildReviewMaterial(ctx, state, "检查本次成果")
	if err != nil {
		t.Fatal(err)
	}
	if len(material.Changes) != 3 || len(material.UserInstructions) != 2 {
		t.Fatalf("material=%+v", material)
	}
	for _, change := range material.Changes {
		if change.Path == "sli_1.html" && (!strings.Contains(change.Diff, "-initial") || !strings.Contains(change.Diff, "+final") || strings.Contains(change.Diff, "intermediate")) {
			t.Fatalf("not a cumulative diff: %s", change.Diff)
		}
	}
	write("sli_1.html", "initial\n")
	material, _, err = buildReviewMaterial(ctx, state, "再次检查")
	if err != nil || len(material.Changes) != 2 {
		t.Fatalf("reverted edit should disappear: %+v %v", material.Changes, err)
	}
	if err := ensureReviewBaseline(ctx, root, "missing-resumed-run", true); err == nil {
		t.Fatal("resume must not fabricate a new baseline")
	}
}

func TestReviewMaterialIncludesLatestExistingScreenshots(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	pack := testPack(model.ModeExecute, model.ScopeCurrentPage, false, "检查")
	raw, _ := json.Marshal(pack.Outline.Outline)
	if err := os.WriteFile(filepath.Join(root, ".outline.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sli_1.html"), []byte("current html"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"shot_old", "shot_latest"} {
		entry := renderimage.Entry{ProjectID: "p1", SlideID: "sli_1", RunID: "previous_run", ScreenshotID: id, SourceHash: "old-html"}
		var imageData bytes.Buffer
		if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, entry.ImagePath())
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, imageData.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := renderimage.Publish(root, entry); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureReviewBaseline(ctx, root, "current_run", false); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(root, "current_run")
	if err != nil {
		t.Fatal(err)
	}
	material, parts, err := buildReviewMaterial(ctx, &RunState{runID: "current_run", projectDir: root, pack: pack, tx: session}, "核对页面")
	if err != nil {
		t.Fatal(err)
	}
	counts := requestImageCounts([]llm.Message{{Content: parts}})
	if len(counts) != 1 || counts["project:p1/render:sli_1/shot_latest"] != 1 {
		t.Fatalf("latest pixels missing: %v", counts)
	}
	if len(material.Pages) != 1 || material.Pages[0].Render == nil || !material.Pages[0].Render.Stale {
		t.Fatalf("stale screenshot must not masquerade as current: %+v", material.Pages)
	}
	if len(material.Changes) != 0 {
		t.Fatalf("runtime images must not appear as authored changes: %+v", material.Changes)
	}
}
