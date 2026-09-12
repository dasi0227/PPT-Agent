package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
)

func TestReadImageExposesVerifiedOriginalPathForBothVariants(t *testing.T) {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 24, 12))); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// The uploaded name and thumbnail format must not determine the HTML path.
	meta, err := attachment.Create(context.Background(), dir, "pro_images", "att_image", "reference.webp", &data)
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"thumbnail", "original"} {
		t.Run(variant, func(t *testing.T) {
			result := (readImageTool{}).Execute(context.Background(), DomainToolInput{
				ProjectDir: dir,
				Context:    contextengine.ContextPack{Project: contextengine.ProjectContext{ID: "pro_images"}},
				Args:       map[string]any{"attachment_id": meta.ID, "variant": variant},
			})
			var observation map[string]any
			if err := json.Unmarshal([]byte(result.Observation), &observation); err != nil {
				t.Fatal(err)
			}
			if observation["original_path"] != "attachments/att_image/original.png" {
				t.Fatalf("unusable original path: %v", observation)
			}
			wantMIME := "image/png"
			if variant == "thumbnail" {
				wantMIME = "image/webp"
			}
			if observation["media_type"] != wantMIME || len(result.ObservationParts) != 2 || result.ObservationParts[1].MIMEType != wantMIME {
				t.Fatalf("variant metadata and image disagree: %+v", result)
			}
		})
	}
}
