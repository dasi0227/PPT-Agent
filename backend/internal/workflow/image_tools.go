package workflow

import (
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
)

type readImageTool struct{}

func (readImageTool) Schema() ToolSchema {
	parameters := objectSchema(nil, map[string]any{
		"attachment_id": map[string]any{"type": "string", "pattern": "^att_[A-Za-z0-9_-]{1,128}$"},
		"variant":       map[string]any{"type": "string", "enum": []string{"thumbnail", "original"}, "default": "thumbnail"},
		"image_path":    map[string]any{"type": "string", "description": "Exact latest rendered image_path from runtime or render_slide. No arbitrary paths."},
	})
	parameters["oneOf"] = []any{
		map[string]any{"required": []string{"attachment_id"}, "not": map[string]any{"required": []string{"image_path"}}},
		map[string]any{"required": []string{"image_path"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"attachment_id"}}, map[string]any{"required": []string{"variant"}}}}},
	}
	return ToolSchema{
		Name:        "read_image",
		Description: "Read either an uploaded attachment by attachment_id (thumbnail/original), or a latest slide render by exact image_path. Render pixels are available for the next model response only; record visual findings as text. Rendered images are not HTML assets. Attachments return their verified original_path for embedding.",
		Parameters:  parameters,
	}
}

func (readImageTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if path, ok := input.Args["image_path"].(string); ok {
		if _, present := input.Args["attachment_id"]; present {
			return failedToolResult(CodeContentInvalid, "choose attachment_id or image_path", false)
		}
		if _, present := input.Args["variant"]; present {
			return failedToolResult(CodeContentInvalid, "variant is only valid for attachments", false)
		}
		for _, image := range latestRenderedImages(input.Context, input.ProjectDir, input.Session) {
			if image.ImagePath != path {
				continue
			}
			entry, err := renderimage.Latest(input.ProjectDir, input.Context.Project.ID, image.SlideID)
			if err != nil || entry.ImagePath() != path {
				break
			}
			if _, _, err := renderimage.Read(ctx, input.ProjectDir, input.Context.Project.ID, entry.ImageRef()); err != nil {
				break
			}
			raw, _ := json.Marshal(image)
			result := SuccessfulToolResult("rendered slide image read")
			result.Observation = string(raw)
			result.ObservationParts = []llm.ContentPart{
				{Type: "text", Text: "<rendered_image>" + string(raw) + "</rendered_image>"},
				{Type: "image", ImageRef: entry.ImageRef(), MIMEType: "image/png", Detail: "high"},
			}
			return result
		}
		return failedToolResult(CodeResourceNotFound, "render image is unavailable or superseded; use the latest runtime image_path or render the slide again", false)
	}
	id, _ := input.Args["attachment_id"].(string)
	variant, _ := input.Args["variant"].(string)
	if variant == "" {
		variant = "thumbnail"
	}
	meta, _, err := attachment.Read(ctx, input.ProjectDir, input.Context.Project.ID, id, variant)
	if err != nil {
		return failedToolResult(attachmentErrorCode(err), "attachment is unavailable", false)
	}
	mediaType := meta.MediaType
	if variant == "thumbnail" {
		mediaType = "image/webp"
	}
	observation, _ := json.Marshal(map[string]any{
		"attachment_id": meta.ID, "name": meta.OriginalName, "media_type": mediaType,
		"width": meta.Width, "height": meta.Height, "variant": variant,
		"original_path": meta.OriginalPath(),
	})
	result := SuccessfulToolResult("image attachment read")
	result.Observation = string(observation)
	result.ObservationParts = []llm.ContentPart{
		{Type: "text", Text: "<image_attachment>" + string(observation) + "</image_attachment>"},
		{Type: "image", ImageRef: meta.ImageRef(input.Context.Project.ID, variant), MIMEType: mediaType, Detail: map[string]string{"thumbnail": "low", "original": "high"}[variant]},
	}
	return result
}

func attachmentErrorCode(err error) string {
	if value, ok := err.(*attachment.Error); ok {
		return string(value.Code)
	}
	return CodeResourceNotFound
}
