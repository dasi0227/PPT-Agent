package attachment

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
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
	if meta.MediaType != "image/png" || meta.Width != 24 || meta.Height != 12 || meta.OriginalPath != "attachments/att_reference/original.png" {
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
	if _, _, err := Read(context.Background(), workDir, "pro_other", meta.ID, "original"); err == nil {
		t.Fatal("cross-project attachment access was accepted")
	}
}
