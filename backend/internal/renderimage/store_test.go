package renderimage

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestLatestRenderReplacesIndexWithoutInvalidatingAnAlreadyReadSnapshot(t *testing.T) {
	root := t.TempDir()
	write := func(id, version string) Entry {
		t.Helper()
		entry := Entry{ProjectID: "pro_one", SlideID: "sli_one", RunID: "run_one", ScreenshotID: id, SourceHash: version, DependencyHash: version}
		var data bytes.Buffer
		if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, entry.ImagePath())
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Publish(root, entry); err != nil {
			t.Fatal(err)
		}
		return entry
	}
	old := write("shot_old", "A")
	latest := write("shot_new", "B")
	assertPrevious := func(want string) {
		t.Helper()
		current, err := Latest(root, "pro_one", "sli_one")
		if err != nil {
			t.Fatal(err)
		}
		previous, err := Previous(root, current)
		if err != nil || previous.ScreenshotID != want {
			t.Fatalf("previous distinct version=%+v, want %s: %v", previous, want, err)
		}
	}
	assertPrevious(old.ScreenshotID)
	latest = write("shot_repeat", "B")
	assertPrevious(old.ScreenshotID)
	lastB := latest.ScreenshotID
	latest = write("shot_third", "C")
	assertPrevious(lastB)
	if current, err := Latest(root, "pro_one", "sli_one"); err != nil || current.ImagePath() != latest.ImagePath() {
		t.Fatalf("latest index incorrect: %+v %v", current, err)
	}
	if _, _, err := Read(context.Background(), root, "pro_one", old.ImageRef()); err != nil {
		t.Fatalf("already-read snapshot invalidated: %v", err)
	}
	if _, _, err := Read(context.Background(), root, "pro_one", latest.ImageRef()); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "image.png")
	if err := os.Rename(filepath.Join(root, latest.ImagePath()), outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, latest.ImagePath())); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(context.Background(), root, "pro_one", latest.ImageRef()); err == nil {
		t.Fatal("symlink escaped project")
	}
	latest.RunID = "../../outside"
	if err := Publish(root, latest); err == nil {
		t.Fatal("path traversal in run identity accepted")
	}
}
