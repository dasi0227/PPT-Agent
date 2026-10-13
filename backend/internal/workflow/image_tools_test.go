package workflow

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestReadImageReturnsOnlyOriginalPixelsAndContextProvidesHTMLAddress(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 24, 12))); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	meta, err := attachment.Create(context.Background(), dir, "pro_images", "att_image", "reference.webp", &data)
	if err != nil {
		t.Fatal(err)
	}
	input := DomainToolInput{ProjectDir: dir, Context: contextengine.ContextPack{Project: contextengine.ProjectContext{ID: "pro_images"}}, Args: map[string]any{"attachment_id": meta.ID}}
	result := (readImageTool{}).Execute(context.Background(), input)
	if !result.OK || result.Observation != "" || len(result.ObservationParts) != 1 || result.ObservationParts[0].Type != "image" || result.ObservationParts[0].MIMEType != "image/png" {
		t.Fatalf("result=%+v", result)
	}
	parts := referenceMessageParts("reference", "pro_images", []model.AttachmentReference{meta.Reference()}, nil, []model.ReferenceOrderItem{{Kind: "attachment", RefID: meta.ID}})
	text := ""
	for _, part := range parts {
		text += part.Text
	}
	if !strings.Contains(text, "attachments/att_image.png") {
		t.Fatal("attachment embedding address missing: " + text)
	}
	for _, args := range []map[string]any{{}, {"attachment_id": meta.ID, "variant": "original"}, {"attachment_id": meta.ID, "slide_id": "sli_1"}, {"image_path": "anything"}} {
		input.Args = args
		if (readImageTool{}).Execute(context.Background(), input).OK {
			t.Fatalf("invalid locator accepted: %v", args)
		}
	}
	input.Args = map[string]any{"attachment_id": meta.ID}
	path := filepath.Join(dir, meta.OriginalPath())
	for _, content := range [][]byte{nil, []byte("corrupt image"), []byte(strings.Repeat("x", attachment.MaxFileBytes+1))} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		if result := (readImageTool{}).Execute(context.Background(), input); result.Code != "IMAGE_INVALID" || len(result.ObservationParts) != 0 {
			t.Fatalf("invalid attachment did not use image error contract: %+v", result)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if result := (readImageTool{}).Execute(context.Background(), input); result.Code != "IMAGE_NOT_FOUND" {
		t.Fatalf("missing attachment did not use image error contract: %+v", result)
	}
	if result := attachmentReadFailure(&fs.PathError{Op: "read", Path: path, Err: fs.ErrPermission}, meta.ID); result.Code != model.CodeInternal {
		t.Fatalf("attachment I/O fault treated as image damage: %+v", result)
	}
}
