package attachment

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestCreatePublishesVerifiedProjectImageAndThumbnail(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, 24, 12))
	canvas.Set(3, 4, color.RGBA{R: 255, A: 255})
	var source bytes.Buffer
	if err := png.Encode(&source, canvas); err != nil {
		t.Fatal(err)
	}
	workDir := t.TempDir()
	meta, err := Create(context.Background(), workDir, "pro_current", "att_reference", "brand reference.png", bytes.NewReader(source.Bytes()))
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	if meta.MediaType != "image/png" || meta.Width != 24 || meta.Height != 12 || meta.OriginalPath() != "attachments/att_reference.png" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	loaded, original, err := Read(context.Background(), workDir, "pro_current", meta.ID, "original")
	if err != nil || loaded.ID != meta.ID || !bytes.Equal(original, source.Bytes()) {
		t.Fatalf("stored original mismatch: meta=%+v err=%v", loaded, err)
	}
	_, thumbnail, err := Read(context.Background(), workDir, "pro_current", meta.ID, "thumbnail")
	if err != nil || len(thumbnail) < 12 || string(thumbnail[:4]) != "RIFF" || string(thumbnail[8:12]) != "WEBP" {
		t.Fatalf("thumbnail is not a readable WebP: err=%v", err)
	}
	if _, _, err := Read(context.Background(), t.TempDir(), "pro_other", meta.ID, "original"); err == nil {
		t.Fatal("cross-project attachment access was accepted")
	}
}

func TestFlatOriginalsAndRebuildableThumbnails(t *testing.T) {
	ctx, root := context.Background(), t.TempDir()
	var source bytes.Buffer
	if err := jpeg.Encode(&source, image.NewRGBA(image.Rect(0, 0, 8, 4)), nil); err != nil {
		t.Fatal(err)
	}
	meta, err := Create(ctx, root, "pro_one", "att_photo", "photo.jpeg", bytes.NewReader(source.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if meta.OriginalPath() != "attachments/att_photo.jpg" {
		t.Fatalf("unexpected original: %s", meta.OriginalPath())
	}
	entries, err := os.ReadDir(filepath.Join(root, "attachments"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected attachment sidecars: %v", entries)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatal("per-image directory was created")
		}
	}
	if err := os.Remove(filepath.Join(root, meta.ThumbnailPath())); err != nil {
		t.Fatal(err)
	}
	_, thumbnail, err := Read(ctx, root, "pro_one", meta.ID, "thumbnail")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, meta.ThumbnailPath())); err != nil {
		t.Fatal("thumbnail was not rebuilt", err)
	}
	if _, err := Create(ctx, root, "pro_one", "att_webp", "photo.png", bytes.NewReader(thumbnail)); err == nil {
		t.Fatal("WebP upload accepted under a PNG filename")
	}
	var pngSource bytes.Buffer
	if err := png.Encode(&pngSource, image.NewRGBA(image.Rect(0, 0, 8, 4))); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(ctx, root, "pro_one", meta.ID, "same.png", bytes.NewReader(pngSource.Bytes())); err == nil {
		t.Fatal("two originals accepted for one attachment ID")
	}
}
