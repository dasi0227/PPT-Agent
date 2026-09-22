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
	write := func(id string) Entry {
		t.Helper()
		entry := Entry{ProjectID: "pro_one", SlideID: "sli_one", RunID: "run_one", ScreenshotID: id, ImagePath: ".runtime/renders/run_one/" + id + ".png", SourceHash: "hash"}
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
		if err := Publish(root, entry); err != nil {
			t.Fatal(err)
		}
		return entry
	}
	old := write("shot_old")
	latest := write("shot_new")
	if current, err := Latest(root, "pro_one", "sli_one"); err != nil || current.ImagePath != latest.ImagePath {
		t.Fatalf("latest index incorrect: %+v %v", current, err)
	}
	if _, _, err := Read(context.Background(), root, "pro_one", old.ImageRef()); err != nil {
		t.Fatalf("already-read snapshot invalidated: %v", err)
	}
	if _, _, err := Read(context.Background(), root, "pro_one", latest.ImageRef()); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "image.png")
	if err := os.Rename(filepath.Join(root, latest.ImagePath), outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, latest.ImagePath)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(context.Background(), root, "pro_one", latest.ImageRef()); err == nil {
		t.Fatal("symlink escaped project")
	}
	latest.ImagePath = "../../image.png"
	if err := Publish(root, latest); err == nil {
		t.Fatal("arbitrary path indexed")
	}
}
