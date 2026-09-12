package workflow

import (
	"context"
	"encoding/json"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

type readImageTool struct{}

func (readImageTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        "read_image",
		Description: "Read one project image attachment by stable attachment_id. Use thumbnail for visual reference and original for fine detail. Returns the verified original_path for HTML embedding regardless of the viewed variant; image_ref is only for model vision.",
		Parameters: objectSchema([]string{"attachment_id"}, map[string]any{
			"attachment_id": map[string]any{"type": "string", "pattern": "^att_[A-Za-z0-9_-]{1,128}$"},
			"variant":       map[string]any{"type": "string", "enum": []string{"thumbnail", "original"}, "default": "thumbnail"},
		}),
	}
}

func (readImageTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
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
		"original_path": meta.OriginalPath,
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
