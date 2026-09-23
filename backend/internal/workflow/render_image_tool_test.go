package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
)

func TestReadImageAllowsOnlyLatestImageOfAnExistingPage(t *testing.T) {
	root := t.TempDir()
	pack := testPack(model.ModeChat, model.ScopeCurrentPage, false, "inspect")
	outline, err := json.Marshal(pack.Outline.Outline)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outline.json"), outline, 0o600); err != nil {
		t.Fatal(err)
	}
	write := func(shot string) renderimage.Entry {
		t.Helper()
		entry := renderimage.Entry{ProjectID: "p1", SlideID: "sli_1", RunID: "previous_run", ScreenshotID: shot,
			ImagePath: ".runtime/renders/previous_run/" + shot + ".png", SourceHash: "previous-html"}
		var data bytes.Buffer
		if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, entry.ImagePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := renderimage.Publish(root, entry); err != nil {
			t.Fatal(err)
		}
		return entry
	}
	old, latest := write("shot_old"), write("shot_latest")
	read := func(args map[string]any) ToolResult {
		return (readImageTool{}).Execute(context.Background(), DomainToolInput{RunID: "new_run", ProjectDir: root, Context: pack, Args: args})
	}
	result := read(map[string]any{"image_path": latest.ImagePath})
	if !result.OK || !llm.HasRenderImages([]llm.Message{{Content: result.ObservationParts}}) {
		t.Fatalf("latest cross-run image unavailable: %+v", result)
	}
	images := latestRenderedImages(pack, root, nil)
	if len(images) != 1 || !images[0].Stale || images[0].ImagePath != latest.ImagePath {
		t.Fatalf("missing source should mark latest image stale: %+v", images)
	}
	for _, args := range []map[string]any{
		{"image_path": old.ImagePath}, {"image_path": "../../secret.png"},
		{"image_path": latest.ImagePath, "attachment_id": "att_one"},
		{"image_path": latest.ImagePath, "variant": "original"},
	} {
		if result := read(args); result.OK {
			t.Fatalf("unauthorized or ambiguous read accepted: %+v", args)
		}
	}
	pack.Outline.Outline.Sections = nil
	outline, err = json.Marshal(pack.Outline.Outline)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outline.json"), outline, 0o600); err != nil {
		t.Fatal(err)
	}
	if result := read(map[string]any{"image_path": latest.ImagePath}); result.OK {
		t.Fatal("deleted page image accepted")
	}
	if images := latestRenderedImages(pack, root, nil); len(images) != 0 {
		t.Fatalf("deleted page in runtime index: %+v", images)
	}
}
